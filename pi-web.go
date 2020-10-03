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

var gitCommit string

type environment struct {
	EnvVars    []string `json:"envVars"`
	GitCommit  string   `json:"gitCommit"`
	GoMaxProcs int      `json:"goMaxProcs"`
	GoVersion  string   `json:"goVersion"`
}

const (
	templatesDirectory         = "templates"
	mainTemplateFile           = "main.html"
	commandTemplateFile        = "command.html"
	proxyTemplateFile          = "proxy.html"
	debugTemplateFile          = "debug.html"
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
		filepath.Join(templatesDirectory, proxyTemplateFile),
		filepath.Join(templatesDirectory, debugTemplateFile),
	))

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
	NumStaticDirectoriesInMainPage int
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

	for i := range configuration.StaticDirectories {
		if configuration.StaticDirectories[i].IncludeInMainPage {
			mainPageMetadata.NumStaticDirectoriesInMainPage++
		}
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

func staticDirectoryHandler(staticDirectoryInfo staticDirectoryInfo) http.HandlerFunc {
	fileServer := http.StripPrefix(
		staticDirectoryInfo.HTTPPath,
		http.FileServer(http.Dir(staticDirectoryInfo.DirectoryPath)))

	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add(cacheControlHeaderKey, staticDirectoryInfo.CacheControlValue)
		fileServer.ServeHTTP(w, r)
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

type debugHTMLData struct {
	Title   string
	PreText string
}

func configurationHandlerFunction(configuration *configuration) http.HandlerFunc {
	jsonBytes, err := json.Marshal(configuration)
	if err != nil {
		log.Fatalf("error generating configuration json")
	}

	var formattedJSONBuffer bytes.Buffer
	err = json.Indent(&formattedJSONBuffer, jsonBytes, "", "  ")
	if err != nil {
		log.Fatalf("error indenting configuration json")
	}
	formattedJSONString := formattedJSONBuffer.String()

	var htmlBuilder strings.Builder
	debugHTMLData := &debugHTMLData{
		Title:   "Configuration",
		PreText: formattedJSONString,
	}

	if err := templates.ExecuteTemplate(&htmlBuilder, debugTemplateFile, debugHTMLData); err != nil {
		log.Fatalf("error executing configuration page template %v", err)
	}

	htmlString := htmlBuilder.String()

	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add(cacheControlHeaderKey, maxAgeZero)

		io.Copy(w, strings.NewReader(htmlString))
	}
}

func environmentHandlerFunction(environment *environment) http.HandlerFunc {
	jsonBytes, err := json.Marshal(environment)
	if err != nil {
		log.Fatalf("error generating environment json")
	}

	var formattedJSONBuffer bytes.Buffer
	err = json.Indent(&formattedJSONBuffer, jsonBytes, "", "  ")
	if err != nil {
		log.Fatalf("error indenting environment json")
	}
	formattedJSONString := formattedJSONBuffer.String()

	var htmlBuilder strings.Builder
	debugHTMLData := &debugHTMLData{
		Title:   "Environment",
		PreText: formattedJSONString,
	}

	if err := templates.ExecuteTemplate(&htmlBuilder, debugTemplateFile, debugHTMLData); err != nil {
		log.Fatalf("error executing environment page template %v", err)
	}

	htmlString := htmlBuilder.String()

	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add(cacheControlHeaderKey, maxAgeZero)

		io.Copy(w, strings.NewReader(htmlString))
	}
}

func requestInfoHandlerFunc() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var buffer strings.Builder

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

		var htmlBuilder strings.Builder
		debugHTMLData := &debugHTMLData{
			Title:   "Request Info",
			PreText: buffer.String(),
		}

		if err := templates.ExecuteTemplate(&htmlBuilder, debugTemplateFile, debugHTMLData); err != nil {
			log.Fatalf("error executing request info page template %v", err)
		}

		htmlString := htmlBuilder.String()

		w.Header().Add(cacheControlHeaderKey, maxAgeZero)

		io.Copy(w, strings.NewReader(htmlString))
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

func getEnvironment() *environment {
	return &environment{
		EnvVars:    os.Environ(),
		GitCommit:  gitCommit,
		GoMaxProcs: runtime.GOMAXPROCS(0),
		GoVersion:  runtime.Version(),
	}
}

func runServer(listenInfo listenInfo, serveHandler http.Handler) {
	log.Printf("runServer listenInfo = %#v", listenInfo)
	if listenInfo.TLSInfo.Enabled {
		log.Fatal(
			http.ListenAndServeTLS(
				listenInfo.ListenAddress,
				listenInfo.TLSInfo.CertFile,
				listenInfo.TLSInfo.KeyFile,
				serveHandler))
	} else {
		log.Fatal(
			http.ListenAndServe(
				listenInfo.ListenAddress,
				serveHandler))
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

	createCommandHandler(configuration, serveMux)

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

	var serveHandler http.Handler = serveMux
	if configuration.LogRequests {
		serveHandler = gorillaHandlers.LoggingHandler(os.Stdout, serveMux)
	}

	for _, listenInfo := range configuration.ListenInfoList {
		go runServer(listenInfo, serveHandler)
	}

	awaitShutdownSignal()
}
