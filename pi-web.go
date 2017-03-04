package main

import (
	"bytes"
	"fmt"
	"html/template"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"time"

	"github.com/gorilla/handlers"
	"gopkg.in/natefinch/lumberjack.v2"
	"gopkg.in/yaml.v2"
)

var logger = log.New(os.Stdout, "", log.Ldate|log.Ltime|log.Lmicroseconds)

type CommandInfo struct {
	HttpPath    string   `yaml:"httpPath"`
	Description string   `yaml:"description"`
	Command     string   `yaml:"command"`
	Args        []string `yaml:"args"`
}

type Configuration struct {
	ListenAddress string            `yaml:"listenAddress"`
	FavIconFile   string            `yaml:"favIconFile"`
	RequestLogger lumberjack.Logger `yaml:"requestLogger"`
	Commands      []CommandInfo     `yaml:"commands"`
}

const (
	mainTemplateFile    = "main.html"
	commandTemplateFile = "command.html"
)

var templates = template.Must(template.ParseFiles(mainTemplateFile, commandTemplateFile))

func buildMainPageString(configuration *Configuration) string {
	var buffer bytes.Buffer
	err := templates.ExecuteTemplate(&buffer, mainTemplateFile, configuration)
	if err != nil {
		log.Fatalf("error executing main page template %v", err.Error())
	}
	return buffer.String()
}

func mainPageHandlerFunc(configuration *Configuration) http.HandlerFunc {
	mainPageString := buildMainPageString(configuration)

	return func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
		} else {
			io.WriteString(w, mainPageString)
		}
	}
}

func favIconHandlerFunc(configuration *Configuration) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, configuration.FavIconFile)
	}
}

type CommandRunData struct {
	TimeString    string
	Command       string
	Args          []string
	CommandOutput string
}

func commandRunnerHandlerFunc(commandInfo *CommandInfo) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		commandOutput, err := exec.Command(commandInfo.Command, commandInfo.Args...).Output()

		commandRunData := &CommandRunData{
			TimeString: time.Now().Local().String(),
			Command:    commandInfo.Command,
			Args:       commandInfo.Args,
		}

		if err != nil {
			commandRunData.CommandOutput = fmt.Sprintf("cmd err %v", err.Error())
		} else {
			commandRunData.CommandOutput = string(commandOutput)
		}

		err = templates.ExecuteTemplate(w, commandTemplateFile, commandRunData)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	}
}

func readConfiguration(configFile string) *Configuration {
	logger.Printf("reading %v", configFile)

	source, err := ioutil.ReadFile(configFile)
	if err != nil {
		logger.Fatalf("error reading %v: %v", configFile, err.Error())
	}

	var configuration Configuration
	err = yaml.Unmarshal(source, &configuration)
	if err != nil {
		logger.Fatalf("error parsing %v: %v", configFile, err.Error())
	}

	return &configuration
}

func main() {
	if len(os.Args) != 2 {
		logger.Fatalf("Usage: %v <config yml file>", os.Args[0])
	}

	logger.Printf("GOMAXPROCS = %v", runtime.GOMAXPROCS(0))

	configFile := os.Args[1]

	configuration := readConfiguration(configFile)

	logger.Printf("configuration = %+v", configuration)

	serveMux := http.NewServeMux()

	serveMux.Handle("/", mainPageHandlerFunc(configuration))

	serveMux.Handle("/favicon.ico", favIconHandlerFunc(configuration))

	for i := range configuration.Commands {
		commandInfo := &(configuration.Commands[i])
		serveMux.Handle(
			commandInfo.HttpPath,
			commandRunnerHandlerFunc(commandInfo))
	}

	serveHandler := handlers.CombinedLoggingHandler(
		&configuration.RequestLogger,
		serveMux)

	logger.Fatal(http.ListenAndServe(
		configuration.ListenAddress,
		serveHandler))
}
