package main

import (
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	gorillaHandlers "github.com/gorilla/handlers"
	"github.com/kr/pretty"

	"github.com/aaronriekenberg/pi-web/config"
	"github.com/aaronriekenberg/pi-web/environment"
	"github.com/aaronriekenberg/pi-web/handlers/command"
	"github.com/aaronriekenberg/pi-web/handlers/debug"
	"github.com/aaronriekenberg/pi-web/handlers/file"
	"github.com/aaronriekenberg/pi-web/handlers/mainpage"
	"github.com/aaronriekenberg/pi-web/handlers/proxy"
)

func runServer(listenInfo config.ListenInfo, serveHandler http.Handler) {
	log.Printf("runServer listenInfo = %#v", listenInfo)

	if listenInfo.HTTP3Info.Enabled {
		log.Fatalf(
			"runHTTP3Server error %v",
			runHTTP3Server(
				listenInfo,
				serveHandler,
			),
		)
	} else {
		server := &http.Server{
			Addr:    listenInfo.ListenAddress,
			Handler: serveHandler,
		}
		listenInfo.ServerTimeouts.ApplyToHTTPServer(server)

		if listenInfo.TLSInfo.Enabled {
			log.Fatalf(
				"ListenAndServeTLS error %v",
				server.ListenAndServeTLS(
					listenInfo.TLSInfo.CertFile,
					listenInfo.TLSInfo.KeyFile,
				),
			)
		} else {
			log.Fatalf(
				"ListenAndServe error %v",
				server.ListenAndServe(),
			)
		}
	}
}

func awaitShutdownSignal() {
	sig := make(chan os.Signal)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	s := <-sig
	log.Fatalf("Signal (%v) received, stopping", s)
}

func main() {
	log.SetFlags(log.Ldate | log.Ltime | log.Lmicroseconds)

	if len(os.Args) != 2 {
		log.Fatalf("Usage: %v <config yml file>", os.Args[0])
	}

	configFile := os.Args[1]

	configuration := config.ReadConfiguration(configFile)
	log.Printf("configuration:\n%# v", pretty.Formatter(configuration))

	environment := environment.GetEnvironment()
	log.Printf("environment:\n%# v", pretty.Formatter(environment))

	serveMux := http.NewServeMux()

	mainpage.CreateMainPageHandler(configuration, serveMux, environment)

	file.CreateFileHandler(configuration, serveMux)

	command.CreateCommandHandler(configuration, serveMux)

	proxy.CreateProxyHandler(configuration, serveMux)

	debug.CreateDebugHandler(configuration, environment, serveMux)

	var serveHandler http.Handler = serveMux
	if configuration.LogRequests {
		serveHandler = gorillaHandlers.LoggingHandler(os.Stdout, serveMux)
	}

	for _, listenInfo := range configuration.ListenInfoList {
		go runServer(listenInfo, serveHandler)
	}

	awaitShutdownSignal()
}
