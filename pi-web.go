package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"net/http/pprof"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"syscall"
	"time"

	gorillaHandlers "github.com/gorilla/handlers"
	"github.com/kr/pretty"
)

type tlsInfo struct {
	Enabled  bool   `json:"enabled"`
	CertFile string `json:"certFile"`
	KeyFile  string `json:"keyFile"`
}

type templatePageInfo struct {
	CacheControlValue string `json:"cacheControlValue"`
}

type mainPageInfo struct {
	Title string `json:"title"`
}

type pprofInfo struct {
	Enabled bool `json:"enabled"`
}

type staticFileInfo struct {
	HTTPPath          string `json:"httpPath"`
	FilePath          string `json:"filePath"`
	CacheControlValue string `json:"cacheControlValue"`
}

type staticDirectoryInfo struct {
	HTTPPath      string `json:"httpPath"`
	DirectoryPath string `json:"directoryPath"`
}

type commandTimeoutInfo struct {
	TimeoutMilliseconds int `json:"timeoutMilliseconds"`
}

type commandInfo struct {
	ID          string   `json:"id"`
	Description string   `json:"description"`
	Command     string   `json:"command"`
	Args        []string `json:"args"`
}

type proxyInfo struct {
	ID          string `json:"id"`
	Description string `json:"description"`
	URL         string `json:"url"`
}

type configuration struct {
	ListenAddress      string                `json:"listenAddress"`
	TLSInfo            tlsInfo               `json:"tlsInfo"`
	TemplatePageInfo   templatePageInfo      `json:"templatePageInfo"`
	MainPageInfo       mainPageInfo          `json:"mainPageInfo"`
	PprofInfo          pprofInfo             `json:"pprofInfo"`
	StaticFiles        []staticFileInfo      `json:"staticFiles"`
	StaticDirectories  []staticDirectoryInfo `json:"staticDirectories"`
	CommandTimeoutInfo commandTimeoutInfo    `json:"commandTimeoutInfo"`
	Commands           []commandInfo         `json:"commands"`
	Proxies            []proxyInfo           `json:"proxies"`
}

type environment struct {
	EnvVars    []string `json:"envVars"`
	GitHash    string   `json:"gitHash"`
	GoMaxProcs int      `json:"goMaxProcs"`
	GoVersion  string   `json:"goVersion"`
}

const (
	templatesDirectory         = "templates"
	mainTemplateFile           = "main.html"
	commandTemplateFile        = "command.html"
	proxyTemplateFile          = "proxy.html"
	cacheControlHeaderKey      = "cache-control"
	maxAgeZero                 = "max-age=0"
	contentTypeHeaderKey       = "content-type"
	contentTypeTextHTML        = "text/html"
	contentTypeTextPlain       = "text/plain"
	contentTypeApplicationJSON = "application/json"
)

var templates = template.Must(
	template.ParseFiles(
		filepath.Join(templatesDirectory, mainTemplateFile),
		filepath.Join(templatesDirectory, commandTemplateFile),
		filepath.Join(templatesDirectory, proxyTemplateFile)))

func formatTime(t time.Time) string {
	return t.Format("Mon Jan 2 15:04:05.000000000 -0700 MST 2006")
}

func httpHeaderToString(header http.Header) string {
	var builder strings.Builder
	keys := make([]string, 0, len(header))
	for key := range header {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for i, key := range keys {
		if i != 0 {
			builder.WriteRune('\n')
		}
		builder.WriteString(key)
		builder.WriteString(": ")
		fmt.Fprintf(&builder, "%v", header[key])
	}
	return builder.String()
}

type mainPageMetadata struct {
	*configuration
	*environment
	LastModified string
}

func buildMainPageString(configuration *configuration, environment *environment, lastModified time.Time) string {
	var builder strings.Builder
	mainPageMetadata := &mainPageMetadata{
		configuration: configuration,
		environment:   environment,
		LastModified:  formatTime(lastModified),
	}
	if err := templates.ExecuteTemplate(&builder, mainTemplateFile, mainPageMetadata); err != nil {
		log.Fatalf("error executing main page template %v", err)
	}
	return builder.String()
}

func mainPageHandlerFunc(configuration *configuration, environment *environment) http.HandlerFunc {
	lastModified := time.Now()
	mainPageString := buildMainPageString(configuration, environment, lastModified)
	cacheControlValue := configuration.TemplatePageInfo.CacheControlValue

	return func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}

		w.Header().Add(cacheControlHeaderKey, cacheControlValue)
		w.Header().Add(contentTypeHeaderKey, contentTypeTextHTML)
		http.ServeContent(w, r, mainTemplateFile, lastModified, strings.NewReader(mainPageString))
	}
}

func staticFileHandlerFunc(staticFileInfo staticFileInfo) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add(cacheControlHeaderKey, staticFileInfo.CacheControlValue)
		http.ServeFile(w, r, staticFileInfo.FilePath)
	}
}

func staticDirectoryHandler(staticDirectoryInfo staticDirectoryInfo) http.Handler {
	return http.StripPrefix(
		staticDirectoryInfo.HTTPPath,
		http.FileServer(http.Dir(staticDirectoryInfo.DirectoryPath)))
}

type commandHTMLData struct {
	*commandInfo
}

func commandRunnerHTMLHandlerFunc(
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

type commandAPIResponse struct {
	*commandInfo
	Now             string `json:"now"`
	CommandDuration string `json:"commandDuration"`
	CommandOutput   string `json:"commandOutput"`
}

func commandAPIHandlerFunc(commandInfo commandInfo, commandTimeoutInfo commandTimeoutInfo) http.HandlerFunc {
	timeoutDuration := time.Duration(commandTimeoutInfo.TimeoutMilliseconds) * time.Millisecond

	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(context.Background(), timeoutDuration)
		defer cancel()

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

		commandAPIResponse := &commandAPIResponse{
			commandInfo:     &commandInfo,
			Now:             formatTime(commandEndTime),
			CommandDuration: commandDuration,
			CommandOutput:   commandOutput,
		}

		var jsonText []byte
		if jsonText, err = json.Marshal(commandAPIResponse); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Add(contentTypeHeaderKey, contentTypeApplicationJSON)
		w.Header().Add(cacheControlHeaderKey, maxAgeZero)
		io.Copy(w, bytes.NewReader(jsonText))
	}
}

type proxyHTMLData struct {
	*proxyInfo
}

func proxyHTMLHandlerFunc(
	configuration *configuration, proxyInfo proxyInfo) http.HandlerFunc {

	cacheControlValue := configuration.TemplatePageInfo.CacheControlValue

	proxyHTMLData := &proxyHTMLData{
		proxyInfo: &proxyInfo,
	}

	var builder strings.Builder
	if err := templates.ExecuteTemplate(&builder, proxyTemplateFile, proxyHTMLData); err != nil {
		log.Fatalf("Error executing proxy template ID %v: %v", proxyInfo.ID, err)
	}

	htmlString := builder.String()
	lastModified := time.Now()

	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add(cacheControlHeaderKey, cacheControlValue)
		w.Header().Add(contentTypeHeaderKey, contentTypeTextHTML)
		http.ServeContent(w, r, proxyTemplateFile, lastModified, strings.NewReader(htmlString))
	}
}

type proxyAPIResponse struct {
	*proxyInfo
	Now              string      `json:"now"`
	ProxyDuration    string      `json:"proxyDuration"`
	ProxyStatus      string      `json:"proxyStatus"`
	ProxyRespHeaders http.Header `json:"proxyRespHeaders"`
	ProxyOutput      string      `json:"proxyOutput"`
}

func makeProxyRequest(ctx context.Context, proxyInfo *proxyInfo) (response *proxyAPIResponse, err error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(5)*time.Second)
	defer cancel()

	httpRequest, err := http.NewRequestWithContext(ctx, "GET", proxyInfo.URL, nil)
	if err != nil {
		return
	}

	proxyStartTime := time.Now()
	proxyResponse, err := http.DefaultClient.Do(httpRequest)
	proxyEndTime := time.Now()

	if err != nil {
		return
	}

	defer proxyResponse.Body.Close()

	bodyBuffer, err := ioutil.ReadAll(proxyResponse.Body)
	if err != nil {
		return
	}

	proxyDuration := fmt.Sprintf("%.9f sec", proxyEndTime.Sub(proxyStartTime).Seconds())

	response = &proxyAPIResponse{
		proxyInfo:        proxyInfo,
		Now:              formatTime(proxyEndTime),
		ProxyDuration:    proxyDuration,
		ProxyStatus:      proxyResponse.Status,
		ProxyRespHeaders: proxyResponse.Header,
		ProxyOutput:      string(bodyBuffer),
	}
	return
}

func proxyAPIHandlerFunc(proxyInfo proxyInfo) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		proxyAPIResponse, err := makeProxyRequest(ctx, &proxyInfo)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		jsonText, err := json.Marshal(proxyAPIResponse)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Add(contentTypeHeaderKey, contentTypeApplicationJSON)
		w.Header().Add(cacheControlHeaderKey, maxAgeZero)
		io.Copy(w, bytes.NewReader(jsonText))
	}
}

func configurationHandlerFunction(configuration *configuration) http.HandlerFunc {
	rawBytes, err := json.Marshal(configuration)
	if err != nil {
		log.Fatalf("error generating configuration json")
	}

	var formattedBuffer bytes.Buffer
	err = json.Indent(&formattedBuffer, rawBytes, "", "  ")
	if err != nil {
		log.Fatalf("error indenting configuration json")
	}
	formattedBytes := formattedBuffer.Bytes()

	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add(contentTypeHeaderKey, contentTypeTextPlain)
		w.Header().Add(cacheControlHeaderKey, maxAgeZero)

		io.Copy(w, bytes.NewReader(formattedBytes))
	}
}

func environmentHandlerFunction(environment *environment) http.HandlerFunc {
	rawBytes, err := json.Marshal(environment)
	if err != nil {
		log.Fatalf("error generating environment json")
	}

	var formattedBuffer bytes.Buffer
	err = json.Indent(&formattedBuffer, rawBytes, "", "  ")
	if err != nil {
		log.Fatalf("error indenting environment json")
	}
	formattedBytes := formattedBuffer.Bytes()

	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add(contentTypeHeaderKey, contentTypeTextPlain)
		w.Header().Add(cacheControlHeaderKey, maxAgeZero)

		io.Copy(w, bytes.NewReader(formattedBytes))
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
		fmt.Fprintf(&buffer, "%#v", r.URL)
		buffer.WriteRune('\n')

		buffer.WriteString("Body.ContentLength: ")
		fmt.Fprintf(&buffer, "%v", r.ContentLength)
		buffer.WriteRune('\n')

		buffer.WriteString("Close: ")
		fmt.Fprintf(&buffer, "%v", r.Close)
		buffer.WriteRune('\n')

		buffer.WriteString("TLS: ")
		fmt.Fprintf(&buffer, "%#v", r.TLS)
		buffer.WriteString("\n\n")

		buffer.WriteString("Headers:\n")
		buffer.WriteString(httpHeaderToString(r.Header))

		w.Header().Add(contentTypeHeaderKey, contentTypeTextPlain)
		w.Header().Add(cacheControlHeaderKey, maxAgeZero)

		io.Copy(w, &buffer)
	}
}

func installPprofHandlers(pprofInfo pprofInfo, serveMux *http.ServeMux) {
	if pprofInfo.Enabled {
		serveMux.Handle("/debug/pprof/", http.HandlerFunc(pprof.Index))
		serveMux.Handle("/debug/pprof/cmdline", http.HandlerFunc(pprof.Cmdline))
		serveMux.Handle("/debug/pprof/profile", http.HandlerFunc(pprof.Profile))
		serveMux.Handle("/debug/pprof/symbol", http.HandlerFunc(pprof.Symbol))
		serveMux.Handle("/debug/pprof/trace", http.HandlerFunc(pprof.Trace))
	}
}

func readConfiguration(configFile string) *configuration {
	log.Printf("reading json file %v", configFile)

	source, err := ioutil.ReadFile(configFile)
	if err != nil {
		log.Fatalf("error reading %v: %v", configFile, err)
	}

	var config configuration
	if err = json.Unmarshal(source, &config); err != nil {
		log.Fatalf("error parsing %v: %v", configFile, err)
	}

	return &config
}

func getGitHash() string {
	rawCommandOutput, err := exec.Command("git", "rev-parse", "HEAD").CombinedOutput()
	if err != nil {
		log.Fatalf("error executing git: %v", err)
	}

	return strings.TrimSpace(string(rawCommandOutput))
}

func getEnvironment() *environment {
	return &environment{
		EnvVars:    os.Environ(),
		GitHash:    getGitHash(),
		GoMaxProcs: runtime.GOMAXPROCS(0),
		GoVersion:  runtime.Version(),
	}
}

func awaitShutdownSignal() {
	sig := make(chan os.Signal)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	s := <-sig
	log.Fatalf("Signal (%v) received, stopping", s)
}

func main() {
	log.SetFlags(log.Ldate | log.Ltime | log.Lmicroseconds)

	if len(os.Args) != 2 {
		log.Fatalf("Usage: %v <config yml file>", os.Args[0])
	}

	configFile := os.Args[1]

	configuration := readConfiguration(configFile)
	log.Printf("configuration:\n%# v", pretty.Formatter(configuration))

	environment := getEnvironment()
	log.Printf("environment:\n%# v", pretty.Formatter(environment))

	serveMux := http.NewServeMux()

	serveMux.Handle("/", mainPageHandlerFunc(configuration, environment))

	for _, staticFileInfo := range configuration.StaticFiles {
		serveMux.Handle(
			staticFileInfo.HTTPPath,
			staticFileHandlerFunc(staticFileInfo))
	}

	for _, staticDirectoryInfo := range configuration.StaticDirectories {
		serveMux.Handle(
			staticDirectoryInfo.HTTPPath,
			staticDirectoryHandler(staticDirectoryInfo))
	}

	for _, commandInfo := range configuration.Commands {
		apiPath := "/api/commands/" + commandInfo.ID
		htmlPath := "/commands/" + commandInfo.ID + ".html"
		serveMux.Handle(
			htmlPath,
			commandRunnerHTMLHandlerFunc(configuration, commandInfo))
		serveMux.Handle(
			apiPath,
			commandAPIHandlerFunc(commandInfo, configuration.CommandTimeoutInfo))
	}

	for _, proxyInfo := range configuration.Proxies {
		apiPath := "/api/proxies/" + proxyInfo.ID
		htmlPath := "/proxies/" + proxyInfo.ID + ".html"
		serveMux.Handle(
			htmlPath,
			proxyHTMLHandlerFunc(configuration, proxyInfo))
		serveMux.Handle(
			apiPath,
			proxyAPIHandlerFunc(proxyInfo))
	}

	serveMux.Handle("/configuration", configurationHandlerFunction(configuration))
	serveMux.Handle("/environment", environmentHandlerFunction(environment))
	serveMux.Handle("/reqinfo", requestInfoHandlerFunc())
	installPprofHandlers(configuration.PprofInfo, serveMux)

	serveHandler := gorillaHandlers.LoggingHandler(os.Stdout, serveMux)

	go awaitShutdownSignal()

	if configuration.TLSInfo.Enabled {
		log.Fatal(
			http.ListenAndServeTLS(
				configuration.ListenAddress,
				configuration.TLSInfo.CertFile,
				configuration.TLSInfo.KeyFile,
				serveHandler))
	} else {
		log.Fatal(
			http.ListenAndServe(
				configuration.ListenAddress,
				serveHandler))
	}
}
