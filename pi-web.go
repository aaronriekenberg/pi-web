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
	"github.com/kr/pretty"
	"gopkg.in/yaml.v2"
)

type TLSInfo struct {
	Enabled  bool   `yaml:"enabled"`
	CertFile string `yaml:"certFile"`
	KeyFile  string `yaml:"keyFile"`
}

type MainPageInfo struct {
	Title             string `yaml:"title"`
	CacheControlValue string `yaml:"cacheControlValue"`
}

type PprofInfo struct {
	Enabled bool `yaml:"enabled"`
}

type StaticFileInfo struct {
	HttpPath          string `yaml:"httpPath"`
	FilePath          string `yaml:"filePath"`
	CacheControlValue string `yaml:"cacheControlValue"`
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

type ProxyInfo struct {
	HttpPath    string `yaml:"httpPath"`
	Description string `yaml:"description"`
	Url         string `yaml:"url"`
}

type Configuration struct {
	ListenAddress     string                `yaml:"listenAddress"`
	TLSInfo           TLSInfo               `yaml:"tlsInfo"`
	MainPageInfo      MainPageInfo          `yaml:"mainPageInfo"`
	PprofInfo         PprofInfo             `yaml:"pprofInfo"`
	StaticFiles       []StaticFileInfo      `yaml:"staticFiles"`
	StaticDirectories []StaticDirectoryInfo `yaml:"staticDirectories"`
	Commands          []CommandInfo         `yaml:"commands"`
	Proxies           []ProxyInfo           `yaml:"proxies"`
}

const (
	mainTemplateFile      = "main.html"
	commandTemplateFile   = "command.html"
	proxyTemplateFile     = "proxy.html"
	cacheControlHeaderKey = "cache-control"
	contentTypeHeaderKey  = "content-type"
)

var templates = template.Must(template.ParseFiles(mainTemplateFile, commandTemplateFile, proxyTemplateFile))

var logger = log.New(os.Stdout, "", log.Ldate|log.Ltime|log.Lmicroseconds)

var httpClient = &http.Client{
	Transport: &http.Transport{
		IdleConnTimeout: 10 * time.Second,
	},
}

func formatTime(t time.Time) string {
	return t.Format("Mon Jan 2 15:04:05.999999999 -0700 MST 2006")
}

func httpHeaderToString(header http.Header) string {
	var buffer bytes.Buffer
	keys := make([]string, 0, len(header))
	for key := range header {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for i, key := range keys {
		if i != 0 {
			buffer.WriteRune('\n')
		}
		buffer.WriteString(key)
		buffer.WriteString(": ")
		buffer.WriteString(fmt.Sprintf("%v", header[key]))
	}
	return buffer.String()
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
	if err := templates.ExecuteTemplate(&buffer, mainTemplateFile, mainPageMetadata); err != nil {
		logger.Fatalf("error executing main page template %v", err.Error())
	}
	return buffer.String()
}

func mainPageHandlerFunc(configuration *Configuration) http.HandlerFunc {
	creationTime := time.Now()
	mainPageBytes := []byte(buildMainPageString(configuration, creationTime))
	cacheControlValue := configuration.MainPageInfo.CacheControlValue

	return func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}

		w.Header().Add(cacheControlHeaderKey, cacheControlValue)
		http.ServeContent(w, r, mainTemplateFile, creationTime, bytes.NewReader(mainPageBytes))
	}
}

func staticFileHandlerFunc(staticFileInfo StaticFileInfo) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add(cacheControlHeaderKey, staticFileInfo.CacheControlValue)
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
		if err := templates.ExecuteTemplate(&buffer, commandTemplateFile, commandRunData); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Add(cacheControlHeaderKey, "max-age=0")
		http.ServeContent(w, r, commandTemplateFile, time.Time{}, bytes.NewReader(buffer.Bytes()))
	}
}

type ProxyData struct {
	*ProxyInfo
	Now              string
	ProxyDuration    string
	ProxyStatus      string
	ProxyRespHeaders string
	ProxyOutput      string
}

func proxyHandlerFunc(configuration *Configuration, proxyInfo ProxyInfo) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		proxyStartTime := time.Now()
		proxyResponse, err := httpClient.Get(proxyInfo.Url)
		proxyEndTime := time.Now()

		var proxyOutput string
		var proxyStatus string
		var proxyRespHeaders string
		if err != nil {
			proxyOutput = fmt.Sprintf("proxy error %v", err.Error())
		} else {
			defer proxyResponse.Body.Close()
			proxyRespHeaders = httpHeaderToString(proxyResponse.Header)
			proxyStatus = proxyResponse.Status

			if body, err := ioutil.ReadAll(proxyResponse.Body); err != nil {
				proxyOutput = fmt.Sprintf("proxy read body error %v", err.Error())
			} else {
				proxyOutput = string(body)
			}
		}

		proxyDuration := fmt.Sprintf("%.9f sec",
			proxyEndTime.Sub(proxyStartTime).Seconds())

		proxyData := &ProxyData{
			ProxyInfo:        &proxyInfo,
			Now:              formatTime(proxyEndTime),
			ProxyDuration:    proxyDuration,
			ProxyStatus:      proxyStatus,
			ProxyRespHeaders: proxyRespHeaders,
			ProxyOutput:      proxyOutput,
		}

		var buffer bytes.Buffer
		if err := templates.ExecuteTemplate(&buffer, proxyTemplateFile, proxyData); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Add(cacheControlHeaderKey, "max-age=0")
		http.ServeContent(w, r, commandTemplateFile, time.Time{}, bytes.NewReader(buffer.Bytes()))
	}
}

func requestInfoHandlerFunc() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var buffer bytes.Buffer

		buffer.WriteString("Method: ")
		buffer.WriteString(r.Method)
		buffer.WriteRune('\n')

		buffer.WriteString("Protocol: ")
		buffer.WriteString(r.Proto)
		buffer.WriteRune('\n')

		buffer.WriteString("Host: ")
		buffer.WriteString(r.Host)
		buffer.WriteRune('\n')

		buffer.WriteString("RemoteAddr: ")
		buffer.WriteString(r.RemoteAddr)
		buffer.WriteRune('\n')

		buffer.WriteString("RequestURI: ")
		buffer.WriteString(r.RequestURI)
		buffer.WriteRune('\n')

		buffer.WriteString("URL: ")
		buffer.WriteString(fmt.Sprintf("%#v", r.URL))
		buffer.WriteRune('\n')

		buffer.WriteString("Body.ContentLength: ")
		buffer.WriteString(fmt.Sprintf("%v", r.ContentLength))
		buffer.WriteRune('\n')

		buffer.WriteString("Close: ")
		buffer.WriteString(fmt.Sprintf("%v", r.Close))
		buffer.WriteRune('\n')

		buffer.WriteString("TLS: ")
		buffer.WriteString(fmt.Sprintf("%#v", r.TLS))
		buffer.WriteString("\n\n")

		buffer.WriteString("Headers:\n")
		buffer.WriteString(httpHeaderToString(r.Header))

		w.Header().Add(contentTypeHeaderKey, "text/plain")
		w.Header().Add(cacheControlHeaderKey, "max-age=0")

		io.Copy(w, &buffer)
	}
}

func installPprofHandlers(pprofInfo PprofInfo, serveMux *http.ServeMux) {
	if pprofInfo.Enabled {
		serveMux.Handle("/debug/pprof/", http.HandlerFunc(pprof.Index))
		serveMux.Handle("/debug/pprof/cmdline", http.HandlerFunc(pprof.Cmdline))
		serveMux.Handle("/debug/pprof/profile", http.HandlerFunc(pprof.Profile))
		serveMux.Handle("/debug/pprof/symbol", http.HandlerFunc(pprof.Symbol))
		serveMux.Handle("/debug/pprof/trace", http.HandlerFunc(pprof.Trace))
	}
}

func readConfiguration(configFile string) *Configuration {
	logger.Printf("reading %v", configFile)

	source, err := ioutil.ReadFile(configFile)
	if err != nil {
		logger.Fatalf("error reading %v: %v", configFile, err.Error())
	}

	var configuration Configuration
	if err := yaml.Unmarshal(source, &configuration); err != nil {
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

	logger.Printf("configuration =\n%# v", pretty.Formatter(configuration))

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

	for _, proxyInfo := range configuration.Proxies {
		serveMux.Handle(
			proxyInfo.HttpPath,
			proxyHandlerFunc(configuration, proxyInfo))
	}

	serveMux.Handle("/reqinfo", requestInfoHandlerFunc())

	installPprofHandlers(configuration.PprofInfo, serveMux)

	serveHandler := gorillaHandlers.CombinedLoggingHandler(os.Stdout, serveMux)

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
