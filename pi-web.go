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

	gorillaHandlers "github.com/gorilla/handlers"
	"gopkg.in/natefinch/lumberjack.v2"
	"gopkg.in/yaml.v2"
)

type CommandInfo struct {
	HttpPath    string   `yaml:"httpPath"`
	Description string   `yaml:"description"`
	Command     string   `yaml:"command"`
	Args        []string `yaml:"args"`
}

type StaticFileInfo struct {
	HttpPath string `yaml:"httpPath"`
	FilePath string `yaml:"filePath"`
}

type StaticDirectoryInfo struct {
	HttpPath      string `yaml:"httpPath"`
	DirectoryPath string `yaml:"directoryPath"`
}

type Configuration struct {
	ListenAddress     string                `yaml:"listenAddress"`
	RequestLogger     lumberjack.Logger     `yaml:"requestLogger"`
	MainPageTitle     string                `yaml:"mainPageTitle"`
	StaticFiles       []StaticFileInfo      `yaml:"staticFiles"`
	StaticDirectories []StaticDirectoryInfo `yaml:"staticDirectories"`
	Commands          []CommandInfo         `yaml:"commands"`
}

const (
	mainTemplateFile    = "main.html"
	commandTemplateFile = "command.html"
)

var templates = template.Must(template.ParseFiles(mainTemplateFile, commandTemplateFile))

var logger = log.New(os.Stdout, "", log.Ldate|log.Ltime|log.Lmicroseconds)

func buildMainPageString(configuration *Configuration) string {
	var buffer bytes.Buffer
	err := templates.ExecuteTemplate(&buffer, mainTemplateFile, configuration)
	if err != nil {
		logger.Fatalf("error executing main page template %v", err.Error())
	}
	return buffer.String()
}

func mainPageHandlerFunc(configuration *Configuration) http.HandlerFunc {
	mainPageString := buildMainPageString(configuration)

	return func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		io.WriteString(w, mainPageString)
	}
}

func staticFileHandlerFunc(fileName string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, fileName)
	}
}

type CommandRunData struct {
	*CommandInfo
	Now             string
	CommandDuration string
	CommandOutput   string
}

func commandRunnerHandlerFunc(commandInfo CommandInfo) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		commandStartTime := time.Now()
		rawCommandOutput, err := exec.Command(
			commandInfo.Command, commandInfo.Args...).CombinedOutput()
		commandEndTime := time.Now()

		var commandOutput string
		if err != nil {
			commandOutput = fmt.Sprintf("command error %v", err.Error())
		} else {
			commandOutput = string(rawCommandOutput)
		}

		commandDuration := fmt.Sprintf("%.9f sec",
			commandEndTime.Sub(commandStartTime).Seconds())

		commandRunData := &CommandRunData{
			CommandInfo:     &commandInfo,
			Now:             commandEndTime.String(),
			CommandDuration: commandDuration,
			CommandOutput:   commandOutput,
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

	for _, staticFileInfo := range configuration.StaticFiles {
		serveMux.Handle(
			staticFileInfo.HttpPath,
			staticFileHandlerFunc(staticFileInfo.FilePath))
	}

	for _, staticDirectoryInfo := range configuration.StaticDirectories {
		serveMux.Handle(
			staticDirectoryInfo.HttpPath,
			http.StripPrefix(staticDirectoryInfo.HttpPath,
				http.FileServer(http.Dir(staticDirectoryInfo.DirectoryPath))))
	}

	for _, commandInfo := range configuration.Commands {
		serveMux.Handle(
			commandInfo.HttpPath,
			commandRunnerHandlerFunc(commandInfo))
	}

	serveHandler := gorillaHandlers.CombinedLoggingHandler(
		&(configuration.RequestLogger),
		serveMux)

	logger.Fatal(
		http.ListenAndServe(
			configuration.ListenAddress,
			serveHandler))
}
