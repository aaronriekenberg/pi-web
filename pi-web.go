package main

import (
	"bytes"
	"fmt"
	"html/template"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"net/http/pprof"
	"os"
	"os/exec"
	"runtime"
	"sort"
	"time"

	gorillaHandlers "github.com/gorilla/handlers"
	"gopkg.in/natefinch/lumberjack.v2"
	"gopkg.in/yaml.v2"
)

type TLSInfo struct {
	Enabled  bool   `yaml:"enabled"`
	CertFile string `yaml:"certFile"`
	KeyFile  string `yaml:"keyFile"`
}

type MainPageInfo struct {
	Title string `yaml:"title"`
}

type PushInfo struct {
	Targets []string `yaml:"targets"`
}

type PprofInfo struct {
	Enabled bool `yaml:"enabled"`
}

type StaticFileInfo struct {
	HttpPath string `yaml:"httpPath"`
	FilePath string `yaml:"filePath"`
}

type StaticDirectoryInfo struct {
	HttpPath      string `yaml:"httpPath"`
	DirectoryPath string `yaml:"directoryPath"`
}

type CommandInfo struct {
	HttpPath    string   `yaml:"httpPath"`
	Description string   `yaml:"description"`
	Command     string   `yaml:"command"`
	Args        []string `yaml:"args"`
}

type Configuration struct {
	ListenAddress     string                `yaml:"listenAddress"`
	RequestLogger     lumberjack.Logger     `yaml:"requestLogger"`
	TLSInfo           TLSInfo               `yaml:"tlsInfo"`
	MainPageInfo      MainPageInfo          `yaml:"mainPageInfo"`
	PushInfo          PushInfo              `yaml:"pushInfo"`
	PprofInfo         PprofInfo             `yaml:"pprofInfo"`
	StaticFiles       []StaticFileInfo      `yaml:"staticFiles"`
	StaticDirectories []StaticDirectoryInfo `yaml:"staticDirectories"`
	Commands          []CommandInfo         `yaml:"commands"`
}

const (
	mainTemplateFile    = "main.html"
	commandTemplateFile = "command.html"
	contentTypeHaderKey = "Content-Type"
)

var templates = template.Must(template.ParseFiles(mainTemplateFile, commandTemplateFile))

var logger = log.New(os.Stdout, "", log.Ldate|log.Ltime|log.Lmicroseconds)

func formatTime(t time.Time) string {
	return t.Format("Mon Jan 2 15:04:05.999999999 -0700 MST 2006")
}

func handlePushFiles(w http.ResponseWriter, pushInfo *PushInfo) {
	if len(pushInfo.Targets) == 0 {
		return
	}
	if pusher, ok := w.(http.Pusher); ok {
		for _, target := range pushInfo.Targets {
			if err := pusher.Push(target, nil); err != nil {
				log.Printf("Failed to push %v: %v", target, err)
			}
		}
	}
}

type MainPageMetadata struct {
	*Configuration
	LastModified string
}

func buildMainPageString(configuration *Configuration, creationTime time.Time) string {
	var buffer bytes.Buffer
	mainPageMetadata := &MainPageMetadata{
		Configuration: configuration,
		LastModified:  formatTime(creationTime),
	}
	err := templates.ExecuteTemplate(&buffer, mainTemplateFile, mainPageMetadata)
	if err != nil {
		logger.Fatalf("error executing main page template %v", err.Error())
	}
	return buffer.String()
}

func mainPageHandlerFunc(configuration *Configuration) http.HandlerFunc {
	creationTime := time.Now()
	mainPageBytes := []byte(buildMainPageString(configuration, creationTime))

	return func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}

		handlePushFiles(w, &configuration.PushInfo)

		http.ServeContent(w, r, mainTemplateFile, creationTime, bytes.NewReader(mainPageBytes))
	}
}

func staticFileHandlerFunc(staticFileInfo StaticFileInfo) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, staticFileInfo.FilePath)
	}
}

func staticDirectoryHandler(staticDirectoryInfo StaticDirectoryInfo) http.Handler {
	return http.StripPrefix(
		staticDirectoryInfo.HttpPath,
		http.FileServer(http.Dir(staticDirectoryInfo.DirectoryPath)))
}

type CommandRunData struct {
	*CommandInfo
	Now             string
	CommandDuration string
	CommandOutput   string
}

func commandRunnerHandlerFunc(configuration *Configuration, commandInfo CommandInfo) http.HandlerFunc {
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
			Now:             formatTime(commandEndTime),
			CommandDuration: commandDuration,
			CommandOutput:   commandOutput,
		}

		var buffer bytes.Buffer
		err = templates.ExecuteTemplate(&buffer, commandTemplateFile, commandRunData)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		handlePushFiles(w, &configuration.PushInfo)

		http.ServeContent(w, r, commandTemplateFile, time.Time{}, bytes.NewReader(buffer.Bytes()))
	}
}

func requestInfoHandlerFunc() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var buffer bytes.Buffer

		buffer.WriteString("Method: " + r.Method + "\n")
		buffer.WriteString("Protocol: " + r.Proto + "\n")
		buffer.WriteString("Host: " + r.Host + "\n")
		buffer.WriteString("RemoteAddr: " + r.RemoteAddr + "\n")
		buffer.WriteString("RequestURI: " + r.RequestURI + "\n")
		buffer.WriteString("URL: " + fmt.Sprintf("%#v", r.URL) + "\n")
		buffer.WriteString("Body.ContentLength: " + fmt.Sprintf("%v", r.ContentLength) + "\n")
		buffer.WriteString("Close: " + fmt.Sprintf("%#v", r.Close) + "\n")
		buffer.WriteString("TLS: " + fmt.Sprintf("%#v", r.TLS) + "\n")

		buffer.WriteString("\nHeaders:\n")
		keys := make([]string, 0, len(r.Header))
		for key := range r.Header {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			buffer.WriteString(key + ": " + fmt.Sprintf("%v", r.Header[key]) + "\n")
		}

		w.Header().Add(contentTypeHaderKey, "text/plain")
		io.Copy(w, &buffer)
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
			staticFileHandlerFunc(staticFileInfo))
	}

	for _, staticDirectoryInfo := range configuration.StaticDirectories {
		serveMux.Handle(
			staticDirectoryInfo.HttpPath,
			staticDirectoryHandler(staticDirectoryInfo))
	}

	for _, commandInfo := range configuration.Commands {
		serveMux.Handle(
			commandInfo.HttpPath,
			commandRunnerHandlerFunc(configuration, commandInfo))
	}

	serveMux.Handle("/reqinfo", requestInfoHandlerFunc())

	if configuration.PprofInfo.Enabled {
		serveMux.Handle("/debug/pprof/", http.HandlerFunc(pprof.Index))
		serveMux.Handle("/debug/pprof/cmdline", http.HandlerFunc(pprof.Cmdline))
		serveMux.Handle("/debug/pprof/profile", http.HandlerFunc(pprof.Profile))
		serveMux.Handle("/debug/pprof/symbol", http.HandlerFunc(pprof.Symbol))
		serveMux.Handle("/debug/pprof/trace", http.HandlerFunc(pprof.Trace))
	}

	serveHandler := gorillaHandlers.CombinedLoggingHandler(
		&(configuration.RequestLogger),
		serveMux)

	if configuration.TLSInfo.Enabled {
		logger.Fatal(
			http.ListenAndServeTLS(
				configuration.ListenAddress,
				configuration.TLSInfo.CertFile,
				configuration.TLSInfo.KeyFile,
				serveHandler))
	} else {
		logger.Fatal(
			http.ListenAndServe(
				configuration.ListenAddress,
				serveHandler))
	}
}
