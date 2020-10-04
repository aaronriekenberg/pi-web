package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/pprof"
	"strings"
)

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
