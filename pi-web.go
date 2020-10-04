package main

import (
	"html/template"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
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

	createProxyHandler(configuration, serveMux)

	createDebugHandler(configuration, environment, serveMux)

	var serveHandler http.Handler = serveMux
	if configuration.LogRequests {
		serveHandler = gorillaHandlers.LoggingHandler(os.Stdout, serveMux)
	}

	for _, listenInfo := range configuration.ListenInfoList {
		go runServer(listenInfo, serveHandler)
	}

	awaitShutdownSignal()
}
