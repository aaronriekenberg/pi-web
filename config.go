package main

import (
	"encoding/json"
	"io/ioutil"
	"log"
)

type tlsInfo struct {
	Enabled  bool   `json:"enabled"`
	CertFile string `json:"certFile"`
	KeyFile  string `json:"keyFile"`
}

type listenInfo struct {
	TLSInfo       tlsInfo `json:"tlsInfo"`
	ListenAddress string  `json:"listenAddress"`
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
	HTTPPath          string `json:"httpPath"`
	DirectoryPath     string `json:"directoryPath"`
	CacheControlValue string `json:"cacheControlValue"`
}

type commandInfo struct {
	ID          string   `json:"id"`
	Description string   `json:"description"`
	Command     string   `json:"command"`
	Args        []string `json:"args"`
}

type commandConfiguration struct {
	MaxConcurrentCommands               int64         `json:"maxConcurrentCommands"`
	RequestTimeoutMilliseconds          int           `json:"requestTimeoutMilliseconds"`
	SemaphoreAcquireTimeoutMilliseconds int           `json:"semaphoreAcquireTimeoutMilliseconds"`
	Commands                            []commandInfo `json:"commands"`
}

type proxyInfo struct {
	ID          string `json:"id"`
	Description string `json:"description"`
	URL         string `json:"url"`
}

type configuration struct {
	LogRequests          bool                  `json:"logRequests"`
	ListenInfoList       []listenInfo          `json:"listenInfoList"`
	TemplatePageInfo     templatePageInfo      `json:"templatePageInfo"`
	MainPageInfo         mainPageInfo          `json:"mainPageInfo"`
	PprofInfo            pprofInfo             `json:"pprofInfo"`
	StaticFiles          []staticFileInfo      `json:"staticFiles"`
	StaticDirectories    []staticDirectoryInfo `json:"staticDirectories"`
	CommandConfiguration commandConfiguration  `json:"commandConfiguration"`
	Proxies              []proxyInfo           `json:"proxies"`
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
