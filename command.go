package main

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
)

type commandHandler struct {
	commandSemaphore        *semaphore.Weighted
	requestTimeout          time.Duration
	semaphoreAcquireTimeout time.Duration
}

func createCommandHandler(configuration *configuration, serveMux *http.ServeMux) {
	commandConfiguration := &configuration.CommandConfiguration
	commandHandler := &commandHandler{
		commandSemaphore:        semaphore.NewWeighted(commandConfiguration.MaxConcurrentCommands),
		requestTimeout:          time.Duration(commandConfiguration.RequestTimeoutMilliseconds) * time.Millisecond,
		semaphoreAcquireTimeout: time.Duration(commandConfiguration.SemaphoreAcquireTimeoutMilliseconds) * time.Millisecond,
	}

	log.Printf("commandHandler MaxConcurrentCommands = %v", commandConfiguration.MaxConcurrentCommands)
	log.Printf("commandHandler.requestTimeout = %v", commandHandler.requestTimeout)
	log.Printf("commandHandler.semaphoreAcquireTimeout = %v", commandHandler.semaphoreAcquireTimeout)

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
	*commandInfo
}

func (commandHandler *commandHandler) commandRunnerHTMLHandlerFunc(
	configuration *configuration, commandInfo commandInfo) http.HandlerFunc {

	cacheControlValue := configuration.TemplatePageInfo.CacheControlValue

	commandHTMLData := &commandHTMLData{
		commandInfo: &commandInfo,
	}

	var builder strings.Builder
	if err := templates.ExecuteTemplate(&builder, commandTemplateFile, commandHTMLData); err != nil {
		log.Fatalf("Error executing command template ID %v: %v", commandInfo.ID, err)
	}

	htmlString := builder.String()
	lastModified := time.Now()

	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add(cacheControlHeaderKey, cacheControlValue)
		w.Header().Add(contentTypeHeaderKey, contentTypeTextHTML)
		http.ServeContent(w, r, commandTemplateFile, lastModified, strings.NewReader(htmlString))
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
	*commandInfo
	Now             string `json:"now"`
	CommandDuration string `json:"commandDuration"`
	CommandOutput   string `json:"commandOutput"`
}

func (commandHandler *commandHandler) runCommand(ctx context.Context, commandInfo *commandInfo) (response *commandAPIResponse) {
	err := commandHandler.acquireCommandSemaphore(ctx)
	if err != nil {
		response = &commandAPIResponse{
			commandInfo:   commandInfo,
			Now:           formatTime(time.Now()),
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
		commandInfo:     commandInfo,
		Now:             formatTime(commandEndTime),
		CommandDuration: commandDuration,
		CommandOutput:   commandOutput,
	}
	return
}

func (commandHandler *commandHandler) commandAPIHandlerFunc(commandInfo commandInfo) http.HandlerFunc {
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

		w.Header().Add(contentTypeHeaderKey, contentTypeApplicationJSON)
		w.Header().Add(cacheControlHeaderKey, maxAgeZero)
		io.Copy(w, bytes.NewReader(jsonText))
	}
}
