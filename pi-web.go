package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/kr/pretty"

	"github.com/aaronriekenberg/pi-web/config"
	"github.com/aaronriekenberg/pi-web/environment"
	"github.com/aaronriekenberg/pi-web/handlers"
	"github.com/aaronriekenberg/pi-web/servers"
)

func awaitShutdownSignal() {
	sig := make(chan os.Signal, 2)
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

	serveHandler := handlers.CreateHandlers(
		configuration,
		environment,
	)

	servers.StartServers(
		configuration.ServerInfoList,
		serveHandler,
	)

	log.Printf("after StartServers")

	awaitShutdownSignal()
}
