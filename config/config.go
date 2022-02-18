package config

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"time"
)

type TLSInfo struct {
	CertFile string `json:"certFile"`
	KeyFile  string `json:"keyFile"`
}

type HTTPServerTimeouts struct {
	ReadTimeoutMilliseconds  int `json:"readTimeoutMilliseconds"`
	WriteTimeoutMilliseconds int `json:"writeTimeoutMilliseconds"`
}

func (httpServerTimeouts *HTTPServerTimeouts) ApplyToHTTPServer(httpServer *http.Server) {
	if httpServerTimeouts == nil {
		return
	}

	httpServer.ReadTimeout = time.Duration(httpServerTimeouts.ReadTimeoutMilliseconds) * time.Millisecond
	httpServer.WriteTimeout = time.Duration(httpServerTimeouts.WriteTimeoutMilliseconds) * time.Millisecond

	log.Printf("set httpServer.ReadTimeout = %v httpServer.WriteTimeout = %v", httpServer.ReadTimeout, httpServer.WriteTimeout)
}

type HTTP3ServerInfo struct {
	TLSInfo                 TLSInfo             `json:"tlsInfo"`
	OverrideAltSvcPortValue *int                `json:"overrideAltSvcPortValue"`
	HTTPServerTimeouts      *HTTPServerTimeouts `json:"httpServerTimeouts"`
	ListenAddress           string              `json:"listenAddress"`
}

type HTTPServerInfo struct {
	TLSInfo            *TLSInfo            `json:"tlsInfo"`
	HTTPServerTimeouts *HTTPServerTimeouts `json:"httpServerTimeouts"`
	ListenAddress      string              `json:"listenAddress"`
}

type ServerInfo struct {
	HTTP3ServerInfo *HTTP3ServerInfo `json:"http3ServerInfo"`
	HTTPServerInfo  *HTTPServerInfo  `json:"httpServerInfo"`
}

type TemplatePageInfo struct {
	CacheControlValue string `json:"cacheControlValue"`
}

type MainPageInfo struct {
	Title string `json:"title"`
}

type PprofInfo struct {
	Enabled bool `json:"enabled"`
}

type StaticFileInfo struct {
	HTTPPath             string `json:"httpPath"`
	FilePath             string `json:"filePath"`
	CacheControlValue    string `json:"cacheControlValue"`
	CacheContentInMemory bool   `json:"cacheContentInMemory"`
}

type StaticDirectoryInfo struct {
	HTTPPath          string `json:"httpPath"`
	DirectoryPath     string `json:"directoryPath"`
	CacheControlValue string `json:"cacheControlValue"`
	IncludeInMainPage bool   `json:"includeInMainPage"`
}

type CommandInfo struct {
	ID          string   `json:"id"`
	Description string   `json:"description"`
	Command     string   `json:"command"`
	Args        []string `json:"args"`
}

type CommandConfiguration struct {
	MaxConcurrentCommands               int64         `json:"maxConcurrentCommands"`
	RequestTimeoutMilliseconds          int           `json:"requestTimeoutMilliseconds"`
	SemaphoreAcquireTimeoutMilliseconds int           `json:"semaphoreAcquireTimeoutMilliseconds"`
	Commands                            []CommandInfo `json:"commands"`
}

type ProxyInfo struct {
	ID          string `json:"id"`
	Description string `json:"description"`
	URL         string `json:"url"`
}

type Configuration struct {
	LogRequests          bool                  `json:"logRequests"`
	ServerInfoList       []ServerInfo          `json:"serverInfoList"`
	TemplatePageInfo     TemplatePageInfo      `json:"templatePageInfo"`
	MainPageInfo         MainPageInfo          `json:"mainPageInfo"`
	PprofInfo            PprofInfo             `json:"pprofInfo"`
	StaticFiles          []StaticFileInfo      `json:"staticFiles"`
	StaticDirectories    []StaticDirectoryInfo `json:"staticDirectories"`
	CommandConfiguration CommandConfiguration  `json:"commandConfiguration"`
	Proxies              []ProxyInfo           `json:"proxies"`
}

func ReadConfiguration(configFile string) *Configuration {
	log.Printf("reading json file %v", configFile)

	source, err := os.ReadFile(configFile)
	if err != nil {
		log.Fatalf("error reading %v: %v", configFile, err)
	}

	var config Configuration
	if err = json.Unmarshal(source, &config); err != nil {
		log.Fatalf("error parsing %v: %v", configFile, err)
	}

	return &config
}
