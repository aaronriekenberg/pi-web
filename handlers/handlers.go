package handlers

import (
	"net/http"
	"os"

	"github.com/aaronriekenberg/pi-web/config"
	"github.com/aaronriekenberg/pi-web/handlers/command"
	"github.com/aaronriekenberg/pi-web/handlers/debug"
	"github.com/aaronriekenberg/pi-web/handlers/file"
	"github.com/aaronriekenberg/pi-web/handlers/mainpage"
	"github.com/aaronriekenberg/pi-web/handlers/proxy"

	gorillaHandlers "github.com/gorilla/handlers"
)

var allowedHTTPMethods = map[string]bool{
	http.MethodGet:  true,
	http.MethodHead: true,
}

func CreateHandlers(
	configuration *config.Configuration,
) http.Handler {

	serveMux := http.NewServeMux()

	mainpage.CreateMainPageHandler(configuration, serveMux)

	file.CreateFileHandler(configuration, serveMux)

	command.CreateCommandHandler(configuration, serveMux)

	proxy.CreateProxyHandler(configuration, serveMux)

	debug.CreateDebugHandler(configuration, serveMux)

	allowedHTTPMethodsHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !allowedHTTPMethods[r.Method] {
			http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
			return
		}

		serveMux.ServeHTTP(w, r)
	})

	var serveHandler http.Handler = allowedHTTPMethodsHandler
	if configuration.LogRequests {
		serveHandler = gorillaHandlers.CombinedLoggingHandler(os.Stdout, serveHandler)
	}

	return serveHandler
}
