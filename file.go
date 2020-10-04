package main

import "net/http"

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

func createFileHandler(configuration *configuration, serveMux *http.ServeMux) {
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
