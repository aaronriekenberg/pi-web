package debug

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/pprof"
	"sort"
	"strings"

	"github.com/aaronriekenberg/pi-web/config"
	"github.com/aaronriekenberg/pi-web/environment"
	"github.com/aaronriekenberg/pi-web/templates"
	"github.com/aaronriekenberg/pi-web/utils"
)

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

type debugHTMLData struct {
	Title   string
	PreText string
}

func configurationHandlerFunction(configuration *config.Configuration) http.HandlerFunc {
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

	if err := templates.Templates.ExecuteTemplate(&htmlBuilder, templates.DebugTemplateFile, debugHTMLData); err != nil {
		log.Fatalf("error executing configuration page template %v", err)
	}

	htmlString := htmlBuilder.String()

	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add(utils.CacheControlHeaderKey, utils.MaxAgeZero)

		io.Copy(w, strings.NewReader(htmlString))
	}
}

func environmentHandlerFunction(environment *environment.Environment) http.HandlerFunc {
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

	if err := templates.Templates.ExecuteTemplate(&htmlBuilder, templates.DebugTemplateFile, debugHTMLData); err != nil {
		log.Fatalf("error executing environment page template %v", err)
	}

	htmlString := htmlBuilder.String()

	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add(utils.CacheControlHeaderKey, utils.MaxAgeZero)

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

		buffer.WriteString("Request Headers:\n")
		buffer.WriteString(httpHeaderToString(r.Header))

		var htmlBuilder strings.Builder
		debugHTMLData := &debugHTMLData{
			Title:   "Request Info",
			PreText: buffer.String(),
		}

		if err := templates.Templates.ExecuteTemplate(&htmlBuilder, templates.DebugTemplateFile, debugHTMLData); err != nil {
			log.Printf("error executing request info page template %v", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		htmlString := htmlBuilder.String()

		w.Header().Add(utils.CacheControlHeaderKey, utils.MaxAgeZero)

		io.Copy(w, strings.NewReader(htmlString))
	}
}

func installPprofHandlers(pprofInfo config.PprofInfo, serveMux *http.ServeMux) {
	if pprofInfo.Enabled {
		serveMux.Handle("/debug/pprof/", http.HandlerFunc(pprof.Index))
		serveMux.Handle("/debug/pprof/cmdline", http.HandlerFunc(pprof.Cmdline))
		serveMux.Handle("/debug/pprof/profile", http.HandlerFunc(pprof.Profile))
		serveMux.Handle("/debug/pprof/symbol", http.HandlerFunc(pprof.Symbol))
		serveMux.Handle("/debug/pprof/trace", http.HandlerFunc(pprof.Trace))
	}
}

func CreateDebugHandler(configuration *config.Configuration, environment *environment.Environment, serveMux *http.ServeMux) {
	serveMux.Handle("/configuration", configurationHandlerFunction(configuration))
	serveMux.Handle("/environment", environmentHandlerFunction(environment))
	serveMux.Handle("/request_info", requestInfoHandlerFunc())
	installPprofHandlers(configuration.PprofInfo, serveMux)
}
