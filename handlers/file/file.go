package file

import (
	"net/http"

	"github.com/aaronriekenberg/pi-web/config"
	"github.com/aaronriekenberg/pi-web/utils"
)

func staticFileHandlerFunc(staticFileInfo config.StaticFileInfo) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add(utils.CacheControlHeaderKey, staticFileInfo.CacheControlValue)
		http.ServeFile(w, r, staticFileInfo.FilePath)
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
