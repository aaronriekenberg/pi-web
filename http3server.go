package main

import (
	"log"
	"net/http"

	"github.com/lucas-clemente/quic-go/http3"

	"github.com/aaronriekenberg/pi-web/config"
)

// See https://github.com/lucas-clemente/quic-go/blob/master/http3/server.go#L492
// This function is needed so we can set quicServer.Port to http3Info.OverrideAltSvcPortValue.
func runHTTP3Server(
	listenAddress string,
	http3Info config.HTTP3Info,
	handler http.Handler,
) {

	log.Printf("runHTTP3Server listenAddress = %q http3Info = %+v", listenAddress, http3Info)

	// Start the servers
	httpServer := &http.Server{
		Addr: listenAddress,
	}

	http3Server := &http3.Server{
		Server: httpServer,
	}

	if http3Info.OverrideAltSvcPortEnabled {
		http3Server.Port = http3Info.OverrideAltSvcPortValue
	}

	httpServer.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http3Server.SetQuicHeaders(w.Header())
		handler.ServeHTTP(w, r)
	})

	hErr := make(chan error)
	h3Err := make(chan error)
	go func() {
		hErr <- httpServer.ListenAndServeTLS(http3Info.CertFile, http3Info.KeyFile)
	}()
	go func() {
		h3Err <- http3Server.ListenAndServeTLS(http3Info.CertFile, http3Info.KeyFile)
	}()

	select {
	case err := <-hErr:
		log.Fatalf("got http server error %v", err)
	case err := <-h3Err:
		log.Fatalf("got http3 server error %v", err)
	}
}
