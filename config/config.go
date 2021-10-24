package config

import (
	"encoding/json"
	"io/ioutil"
	"log"
)

type HTTP3Info struct {
	Enabled                   bool   `json:"enabled"`
	CertFile                  string `json:"certFile"`
	KeyFile                   string `json:"keyFile"`
	OverrideAltSvcPortEnabled bool   `json:"overrideAltSvcPortEnabled"`
	OverrideAltSvcPortValue   uint32 `json:"overrideAltSvcPortValue"`
}

type TLSInfo struct {
	Enabled  bool   `json:"enabled"`
	CertFile string `json:"certFile"`
	KeyFile  string `json:"keyFile"`
}

type ListenInfo struct {
	HTTP3Info     HTTP3Info `json:"http3Info"`
	TLSInfo       TLSInfo   `json:"tlsInfo"`
	ListenAddress string    `json:"listenAddress"`
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
	HTTPPath          string `json:"httpPath"`
	FilePath          string `json:"filePath"`
	CacheControlValue string `json:"cacheControlValue"`
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
	ListenInfoList       []ListenInfo          `json:"listenInfoList"`
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

	source, err := ioutil.ReadFile(configFile)
	if err != nil {
		log.Fatalf("error reading %v: %v", configFile, err)
	}

	var config Configuration
	if err = json.Unmarshal(source, &config); err != nil {
		log.Fatalf("error parsing %v: %v", configFile, err)
	}

	return &config
}
