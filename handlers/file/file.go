package file

import (
	"bytes"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/aaronriekenberg/pi-web/config"
	"github.com/aaronriekenberg/pi-web/utils"
)

func staticFileHandlerFunc(staticFileInfo config.StaticFileInfo) http.HandlerFunc {
	if !staticFileInfo.CacheContentInMemory {
		return func(w http.ResponseWriter, r *http.Request) {
			w.Header().Add(utils.CacheControlHeaderKey, staticFileInfo.CacheControlValue)
			http.ServeFile(w, r, staticFileInfo.FilePath)
		}
	}

	fileContents, err := os.ReadFile(staticFileInfo.FilePath)
	if err != nil {
		log.Fatalf("error reading CacheContentInMemory static file %v: %v", staticFileInfo.FilePath, err)
	}
	lastModified := time.Now()

	log.Printf("cached static file %q in memory bytes = %v", staticFileInfo.FilePath, len(fileContents))

	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add(utils.CacheControlHeaderKey, staticFileInfo.CacheControlValue)

		http.ServeContent(w, r, staticFileInfo.FilePath, lastModified, bytes.NewReader(fileContents))
	}
}

func staticDirectoryHandler(staticDirectoryInfo config.StaticDirectoryInfo) http.HandlerFunc {
	fileServer := http.StripPrefix(
		staticDirectoryInfo.HTTPPath,
		http.FileServer(http.Dir(staticDirectoryInfo.DirectoryPath)))

	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add(utils.CacheControlHeaderKey, staticDirectoryInfo.CacheControlValue)
		fileServer.ServeHTTP(w, r)
	}
}

func CreateFileHandler(configuration *config.Configuration, serveMux *http.ServeMux) {
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
}
