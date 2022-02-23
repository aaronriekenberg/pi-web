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

func CreateHandlers(
	configuration *config.Configuration,
) http.Handler {

	serveMux := http.NewServeMux()

	mainpage.CreateMainPageHandler(configuration, serveMux)

	file.CreateFileHandler(configuration, serveMux)

	command.CreateCommandHandler(configuration, serveMux)

	proxy.CreateProxyHandler(configuration, serveMux)

	debug.CreateDebugHandler(configuration, serveMux)

	disallowNonGETHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
			return
		}

		serveMux.ServeHTTP(w, r)
	})

	var serveHandler http.Handler = disallowNonGETHandler
	if configuration.LogRequests {
		serveHandler = gorillaHandlers.CombinedLoggingHandler(os.Stdout, disallowNonGETHandler)
	}

	return serveHandler
}
