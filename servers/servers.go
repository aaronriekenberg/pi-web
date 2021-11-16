package servers

import (
	"log"
	"net/http"

	"github.com/aaronriekenberg/pi-web/config"
)

func runServer(serverInfo config.ServerInfo, serveHandler http.Handler) {
	if serverInfo.HTTP3ServerInfo != nil {
		log.Fatalf(
			"runHTTP3Server error %v",
			runHTTP3Server(
				*serverInfo.HTTP3ServerInfo,
				serveHandler,
			),
		)
	}

	if serverInfo.HTTPServerInfo != nil {
		log.Fatalf(
			"runHTTPServer error %v",
			runHTTPServer(
				*serverInfo.HTTPServerInfo,
				serveHandler,
			),
		)
	}

	log.Fatalf("invalid serverInfo %+v", serverInfo)
}

func StartServers(
	serverInfoList []config.ServerInfo,
	serveHandler http.Handler,
) {
	for _, serverInfo := range serverInfoList {
		go runServer(serverInfo, serveHandler)
	}
}
