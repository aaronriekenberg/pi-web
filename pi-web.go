package main

import (
	"bytes"
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
	"path/filepath"
	"runtime"
	"sort"
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
	ListenAddress     string                `json:"listenAddress"`
	TLSInfo           tlsInfo               `json:"tlsInfo"`
	TemplatePageInfo  templatePageInfo      `json:"templatePageInfo"`
	MainPageInfo      mainPageInfo          `json:"mainPageInfo"`
	PprofInfo         pprofInfo             `json:"pprofInfo"`
	StaticFiles       []staticFileInfo      `json:"staticFiles"`
	StaticDirectories []staticDirectoryInfo `json:"staticDirectories"`
	Commands          []commandInfo         `json:"commands"`
	Proxies           []proxyInfo           `json:"proxies"`
}

const (
	templatesDirectory         = "templates"
	mainTemplateFile           = "main.html"
	commandTemplateFile        = "command.html"
	proxyTemplateFile          = "proxy.html"
	cacheControlHeaderKey      = "cache-control"
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

var logger = log.New(os.Stdout, "", log.Ldate|log.Ltime|log.Lmicroseconds)

var httpClient = &http.Client{
	Transport: &http.Transport{
		IdleConnTimeout: 10 * time.Second,
	},
	Timeout: 5 * time.Second,
}

func formatTime(t time.Time) string {
	return t.Format("Mon Jan 2 15:04:05.000000000 -0700 MST 2006")
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

type mainPageMetadata struct {
	*configuration
	LastModified string
}

func buildMainPageString(configuration *configuration, creationTime time.Time) string {
	var buffer bytes.Buffer
	mainPageMetadata := &mainPageMetadata{
		configuration: configuration,
		LastModified:  formatTime(creationTime),
	}
	if err := templates.ExecuteTemplate(&buffer, mainTemplateFile, mainPageMetadata); err != nil {
		logger.Fatalf("error executing main page template %v", err.Error())
	}
	return buffer.String()
}

func mainPageHandlerFunc(configuration *configuration) http.HandlerFunc {
	creationTime := time.Now()
	mainPageBytes := []byte(buildMainPageString(configuration, creationTime))
	cacheControlValue := configuration.TemplatePageInfo.CacheControlValue

	return func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}

		w.Header().Add(cacheControlHeaderKey, cacheControlValue)
		w.Header().Add(contentTypeHeaderKey, contentTypeTextHTML)
		http.ServeContent(w, r, mainTemplateFile, creationTime, bytes.NewReader(mainPageBytes))
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

	var buffer bytes.Buffer
	if err := templates.ExecuteTemplate(&buffer, commandTemplateFile, commandHTMLData); err != nil {
		logger.Fatalf("Error executing command template ID %v: %v", commandInfo.ID, err.Error())
	}

	lastModified := time.Now()

	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add(cacheControlHeaderKey, cacheControlValue)
		w.Header().Add(contentTypeHeaderKey, contentTypeTextHTML)
		http.ServeContent(w, r, commandTemplateFile, lastModified, bytes.NewReader(buffer.Bytes()))
	}
}

type commandAPIResponse struct {
	*commandInfo
	Now             string `json:"now"`
	CommandDuration string `json:"commandDuration"`
	CommandOutput   string `json:"commandOutput"`
}

func commandAPIHandlerFunc(configuration *configuration, commandInfo commandInfo) http.HandlerFunc {
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
		w.Header().Add(cacheControlHeaderKey, "max-age=0")
		http.ServeContent(w, r, "", time.Time{}, bytes.NewReader(jsonText))
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

	var buffer bytes.Buffer
	if err := templates.ExecuteTemplate(&buffer, proxyTemplateFile, proxyHTMLData); err != nil {
		logger.Fatalf("Error executing proxy template ID %v: %v", proxyInfo.ID, err.Error())
	}

	lastModified := time.Now()

	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add(cacheControlHeaderKey, cacheControlValue)
		w.Header().Add(contentTypeHeaderKey, contentTypeTextHTML)
		http.ServeContent(w, r, proxyTemplateFile, lastModified, bytes.NewReader(buffer.Bytes()))
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

func proxyAPIHandlerFunc(configuration *configuration, proxyInfo proxyInfo) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		proxyStartTime := time.Now()
		proxyResponse, err := httpClient.Get(proxyInfo.URL)
		proxyEndTime := time.Now()

		var proxyOutput string
		var proxyStatus string
		var proxyRespHeaders http.Header
		if err != nil {
			proxyOutput = fmt.Sprintf("proxy error %v", err.Error())
		} else {
			defer proxyResponse.Body.Close()
			proxyStatus = proxyResponse.Status
			proxyRespHeaders = proxyResponse.Header

			var body []byte
			if body, err = ioutil.ReadAll(proxyResponse.Body); err != nil {
				proxyOutput = fmt.Sprintf("proxy read body error %v", err.Error())
			} else {
				proxyOutput = string(body)
			}
		}

		proxyDuration := fmt.Sprintf("%.9f sec",
			proxyEndTime.Sub(proxyStartTime).Seconds())

		proxyAPIResponse := &proxyAPIResponse{
			proxyInfo:        &proxyInfo,
			Now:              formatTime(proxyEndTime),
			ProxyDuration:    proxyDuration,
			ProxyStatus:      proxyStatus,
			ProxyRespHeaders: proxyRespHeaders,
			ProxyOutput:      proxyOutput,
		}

		var jsonText []byte
		if jsonText, err = json.Marshal(proxyAPIResponse); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Add(contentTypeHeaderKey, contentTypeApplicationJSON)
		w.Header().Add(cacheControlHeaderKey, "max-age=0")
		http.ServeContent(w, r, "", time.Time{}, bytes.NewReader(jsonText))
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

		w.Header().Add(contentTypeHeaderKey, contentTypeTextPlain)
		w.Header().Add(cacheControlHeaderKey, "max-age=0")

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
	logger.Printf("reading json file %v", configFile)

	source, err := ioutil.ReadFile(configFile)
	if err != nil {
		logger.Fatalf("error reading %v: %v", configFile, err.Error())
	}

	var config configuration
	if err := json.Unmarshal(source, &config); err != nil {
		logger.Fatalf("error parsing %v: %v", configFile, err.Error())
	}

	return &config
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
			commandAPIHandlerFunc(configuration, commandInfo))
	}

	for _, proxyInfo := range configuration.Proxies {
		apiPath := "/api/proxies/" + proxyInfo.ID
		htmlPath := "/proxies/" + proxyInfo.ID + ".html"
		serveMux.Handle(
			htmlPath,
			proxyHTMLHandlerFunc(configuration, proxyInfo))
		serveMux.Handle(
			apiPath,
			proxyAPIHandlerFunc(configuration, proxyInfo))
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
