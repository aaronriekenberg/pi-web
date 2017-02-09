package main

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"os/exec"
	"strings"
	"time"
)

type commandInfo struct {
	httpPath    string
	description string
	command     string
	args        []string
}

var commands = []*commandInfo{
	&commandInfo{
		httpPath:    "/ntpq",
		description: "ntpq",
		command:     "ntpq",
		args:        strings.Fields("-p"),
	},
	&commandInfo{
		httpPath:    "/pitemp",
		description: "pitemp",
		command:     "pitemp.sh",
		args:        []string{},
	},
	&commandInfo{
		httpPath:    "/top",
		description: "top",
		command:     "top",
		args:        strings.Fields("-b -n1"),
	},
	&commandInfo{
		httpPath:    "/uptime",
		description: "uptime",
		command:     "uptime",
		args:        []string{},
	},
	&commandInfo{
		httpPath:    "/vmstat",
		description: "vmstat",
		command:     "vmstat",
		args:        []string{},
	},
}

var mainPageString = buildMainPageString()

func buildMainPageString() string {
	var buffer bytes.Buffer
	buffer.WriteString("<html><head><title>Aaron's Raspberry Pi</title></head>")
	buffer.WriteString("<body><ul>")
	for _, commandInfo := range commands {
		buffer.WriteString(
			fmt.Sprintf("<li><a href=\"%v\">%v</a></li>",
				commandInfo.httpPath, commandInfo.description))
	}
	buffer.WriteString("</ul></body>")
	return buffer.String()
}

func mainPageHandler(w http.ResponseWriter, r *http.Request) {
	io.WriteString(w, mainPageString)
}

type commandRunnerHandler struct {
	commandInfo *commandInfo
}

func newCommandRunnerHandler(commandInfo *commandInfo) http.Handler {
	return &commandRunnerHandler{
		commandInfo: commandInfo,
	}
}

func (c *commandRunnerHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	out, err := exec.Command(c.commandInfo.command, c.commandInfo.args...).Output()
	var outputString string
	if err != nil {
		outputString = fmt.Sprintf("cmd err %v", err)
	} else {
		var buffer bytes.Buffer

		buffer.WriteString(time.Now().Local().String())
		buffer.WriteString("\n\n")

		buffer.WriteString("$ ")
		buffer.WriteString(c.commandInfo.command)
		if len(c.commandInfo.args) > 0 {
			buffer.WriteString(" ")
			buffer.WriteString(strings.Join(c.commandInfo.args, " "))
		}
		buffer.WriteString("\n\n")

		buffer.Write(out)

		outputString = buffer.String()
	}
	fmt.Fprintf(w,
		"<html><head><meta http-equiv=\"refresh\" content=\"5\"></head>"+
			"<body><pre>%v</pre></body></html>", outputString)
}

func main() {
	http.HandleFunc("/", mainPageHandler)

	for _, commandInfo := range commands {
		handler := newCommandRunnerHandler(commandInfo)
		http.Handle(commandInfo.httpPath, handler)
	}

	http.ListenAndServe(":8080", nil)
}
