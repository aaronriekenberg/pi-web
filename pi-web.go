package main

import (
	"fmt"
	"net/http"
	"os/exec"
)

type commandInfo struct {
	httpPath string
	command  string
	args     []string
}

var commands = []commandInfo{
	commandInfo{
		httpPath: "/ntpq",
		command:  "ntpq",
		args:     []string{"-p"},
	},
	commandInfo{
		httpPath: "/uptime",
		command:  "uptime",
		args:     []string{},
	},
	commandInfo{
		httpPath: "/vmstat",
		command:  "vmstat",
		args:     []string{},
	},
}

func mainPageHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "<html><head><title>Aaron's Raspberry Pi</title></head>")
	fmt.Fprintf(w, "<body><ul>")
	for _, commandInfo := range commands {
		fmt.Fprintf(w, "<li><a href=\"%v\">%v</a></li>", commandInfo.httpPath, commandInfo.command)
	}
	fmt.Fprintf(w, "</ul></body>")
}

type commandRunnerHandler struct {
	commandInfo commandInfo
}

func newCommandRunnerHandler(commandInfo commandInfo) http.Handler {
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
		outputString = string(out)
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
