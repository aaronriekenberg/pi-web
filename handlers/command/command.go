package command

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os/exec"
	"strings"
	"time"

	"golang.org/x/sync/semaphore"

	"github.com/aaronriekenberg/pi-web/config"
	"github.com/aaronriekenberg/pi-web/templates"
	"github.com/aaronriekenberg/pi-web/utils"
)

type commandHandler struct {
	commandSemaphore        *semaphore.Weighted
	requestTimeout          time.Duration
	semaphoreAcquireTimeout time.Duration
}

func CreateCommandHandler(configuration *config.Configuration, serveMux *http.ServeMux) {
	commandConfiguration := &configuration.CommandConfiguration
	commandHandler := &commandHandler{
		commandSemaphore:        semaphore.NewWeighted(commandConfiguration.MaxConcurrentCommands),
		requestTimeout:          time.Duration(commandConfiguration.RequestTimeoutMilliseconds) * time.Millisecond,
		semaphoreAcquireTimeout: time.Duration(commandConfiguration.SemaphoreAcquireTimeoutMilliseconds) * time.Millisecond,
	}

	for _, commandInfo := range commandConfiguration.Commands {
		apiPath := "/api/commands/" + commandInfo.ID
		htmlPath := "/commands/" + commandInfo.ID + ".html"
		serveMux.Handle(
			htmlPath,
			commandHandler.commandRunnerHTMLHandlerFunc(configuration, commandInfo))
		serveMux.Handle(
			apiPath,
			commandHandler.commandAPIHandlerFunc(commandInfo))
	}
}

type commandHTMLData struct {
	*config.CommandInfo
}

func (commandHandler *commandHandler) commandRunnerHTMLHandlerFunc(
	configuration *config.Configuration, commandInfo config.CommandInfo) http.HandlerFunc {

	cacheControlValue := configuration.TemplatePageInfo.CacheControlValue

	commandHTMLData := &commandHTMLData{
		CommandInfo: &commandInfo,
	}

	var builder strings.Builder
	if err := templates.Templates.ExecuteTemplate(&builder, templates.CommandTemplateFile, commandHTMLData); err != nil {
		log.Fatalf("Error executing command template ID %v: %v", commandInfo.ID, err)
	}

	htmlString := builder.String()
	lastModified := time.Now()

	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add(utils.CacheControlHeaderKey, cacheControlValue)
		w.Header().Add(utils.ContentTypeHeaderKey, utils.ContentTypeTextHTML)
		http.ServeContent(w, r, templates.CommandTemplateFile, lastModified, strings.NewReader(htmlString))
	}
}

func (commandHandler *commandHandler) acquireCommandSemaphore(ctx context.Context) (err error) {
	ctx, cancel := context.WithTimeout(ctx, commandHandler.semaphoreAcquireTimeout)
	defer cancel()

	err = commandHandler.commandSemaphore.Acquire(ctx, 1)
	if err != nil {
		err = fmt.Errorf("commandHandler.acquireCommandSemaphore error calling Acquire: %w", err)
	}
	return
}

func (commandHandler *commandHandler) releaseCommandSemaphore() {
	commandHandler.commandSemaphore.Release(1)
}

type commandAPIResponse struct {
	*config.CommandInfo
	Now             string `json:"now"`
	CommandDuration string `json:"commandDuration"`
	CommandOutput   string `json:"commandOutput"`
}

func (commandHandler *commandHandler) runCommand(ctx context.Context, commandInfo *config.CommandInfo) (response *commandAPIResponse) {
	err := commandHandler.acquireCommandSemaphore(ctx)
	if err != nil {
		response = &commandAPIResponse{
			CommandInfo:   commandInfo,
			Now:           utils.FormatTime(time.Now()),
			CommandOutput: fmt.Sprintf("%v", err),
		}
		return
	}
	defer commandHandler.releaseCommandSemaphore()

	commandStartTime := time.Now()
	rawCommandOutput, err := exec.CommandContext(
		ctx, commandInfo.Command, commandInfo.Args...).CombinedOutput()
	commandEndTime := time.Now()

	var commandOutput string
	if err != nil {
		commandOutput = fmt.Sprintf("command error %v", err)
	} else {
		commandOutput = string(rawCommandOutput)
	}

	commandDuration := fmt.Sprintf("%.9f sec",
		commandEndTime.Sub(commandStartTime).Seconds())

	response = &commandAPIResponse{
		CommandInfo:     commandInfo,
		Now:             utils.FormatTime(commandEndTime),
		CommandDuration: commandDuration,
		CommandOutput:   commandOutput,
	}
	return
}

func (commandHandler *commandHandler) commandAPIHandlerFunc(commandInfo config.CommandInfo) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(context.Background(), commandHandler.requestTimeout)
		defer cancel()

		commandAPIResponse := commandHandler.runCommand(ctx, &commandInfo)

		var jsonText []byte
		jsonText, err := json.Marshal(commandAPIResponse)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Add(utils.ContentTypeHeaderKey, utils.ContentTypeApplicationJSON)
		w.Header().Add(utils.CacheControlHeaderKey, utils.MaxAgeZero)
		io.Copy(w, bytes.NewReader(jsonText))
	}
}
