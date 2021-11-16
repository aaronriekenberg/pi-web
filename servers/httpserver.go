package servers

import (
	"log"
	"net/http"

	"github.com/kr/pretty"

	"github.com/aaronriekenberg/pi-web/config"
)

func runHTTPServer(
	httpServerInfo config.HTTPServerInfo,
	serveHandler http.Handler,
) error {

	log.Printf("runHTTPServer httpServerInfo:\n%# v", pretty.Formatter(httpServerInfo))

	server := &http.Server{
		Addr:    httpServerInfo.ListenAddress,
		Handler: serveHandler,
	}
	httpServerInfo.HTTPServerTimeouts.ApplyToHTTPServer(server)

	if httpServerInfo.TLSInfo != nil {
		return server.ListenAndServeTLS(
			httpServerInfo.TLSInfo.CertFile,
			httpServerInfo.TLSInfo.KeyFile,
		)
	}

	return server.ListenAndServe()

}
