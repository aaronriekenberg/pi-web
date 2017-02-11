package main

import (
	"bytes"
	"fmt"
	"html"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"

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
	ListenAddress  string        `yaml:"listenAddress"`
	RefreshSeconds int           `yaml:"refreshSeconds"`
	Commands       []CommandInfo `yaml:"commands"`
}

func buildMainPageString(configuration *Configuration) string {
	var buffer bytes.Buffer
	buffer.WriteString("<html><head><title>Aaron's Raspberry Pi</title></head>")
	buffer.WriteString("<body><ul>")
	for i := range configuration.Commands {
		commandInfo := &(configuration.Commands[i])
		buffer.WriteString(
			fmt.Sprintf("<li><a href=\"%v\">%v</a></li>",
				commandInfo.HttpPath, commandInfo.Description))
	}
	buffer.WriteString("</ul></body>")
	return buffer.String()
}

func mainPageHandlerFunc(configuration *Configuration) http.HandlerFunc {
	mainPageString := buildMainPageString(configuration)
	return func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, mainPageString)
	}
}

func commandRunnerHandlerFunc(configuration *Configuration, commandInfo *CommandInfo) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var outputString string
		commandOutput, err := exec.Command(commandInfo.Command, commandInfo.Args...).Output()
		if err != nil {
			outputString = fmt.Sprintf("cmd err %v", err)
		} else {
			var buffer bytes.Buffer

			buffer.WriteString(time.Now().Local().String())
			buffer.WriteString("\n\n")

			buffer.WriteString("$ ")
			buffer.WriteString(commandInfo.Command)
			if len(commandInfo.Args) > 0 {
				buffer.WriteString(" ")
				buffer.WriteString(strings.Join(commandInfo.Args, " "))
			}
			buffer.WriteString("\n\n")

			buffer.WriteString(html.EscapeString(string(commandOutput)))

			outputString = buffer.String()
		}
		fmt.Fprintf(w,
			"<html><head><meta http-equiv=\"refresh\" content=\"%d\"></head>"+
				"<body><pre>%s</pre></body></html>",
			configuration.RefreshSeconds, outputString)
	}
}

func readConfiguration(configFile string) *Configuration {
	logger.Printf("reading %v", configFile)

	source, err := ioutil.ReadFile(configFile)
	if err != nil {
		logger.Fatalf("error reading %v: %v", configFile, err)
	}

	var configuration Configuration
	err = yaml.Unmarshal(source, &configuration)
	if err != nil {
		logger.Fatalf("error parsing %v: %v", configFile, err)
	}

	return &configuration
}

func main() {
	if len(os.Args) < 2 {
		logger.Fatalf("Usage: %v <config yml file>", os.Args[0])
	}

	configFile := os.Args[1]

	configuration := readConfiguration(configFile)

	logger.Printf("configuration = %+v", configuration)

	http.HandleFunc("/", mainPageHandlerFunc(configuration))

	for i := range configuration.Commands {
		commandInfo := &(configuration.Commands[i])
		http.HandleFunc(
			commandInfo.HttpPath,
			commandRunnerHandlerFunc(configuration, commandInfo))
	}

	logger.Fatal(
		http.ListenAndServe(
			configuration.ListenAddress, nil))
}
