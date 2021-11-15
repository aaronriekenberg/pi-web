package servers

import (
	"crypto/tls"
	"log"
	"net"
	"net/http"

	"github.com/kr/pretty"
	"github.com/lucas-clemente/quic-go/http3"

	"github.com/aaronriekenberg/pi-web/config"
)

// See https://github.com/lucas-clemente/quic-go/blob/master/http3/server.go#L492
// This function is needed so we can set quicServer.Port to http3Info.OverrideAltSvcPortValue.
// Also read and write timeouts are set on the TCP http server to listenInfo.HTTPServerTimeouts.
func RunHTTP3Server(
	http3ServerInfo config.HTTP3ServerInfo,
	handler http.Handler,
) error {

	log.Printf("runHTTP3Server http3ServerInfo:\n%# v", pretty.Formatter(http3ServerInfo))

	// Load certs
	var err error
	certs := make([]tls.Certificate, 1)
	certs[0], err = tls.LoadX509KeyPair(http3ServerInfo.TLSInfo.CertFile, http3ServerInfo.TLSInfo.KeyFile)
	if err != nil {
		return err
	}
	// We currently only use the cert-related stuff from tls.Config,
	// so we don't need to make a full copy.
	config := &tls.Config{
		Certificates: certs,
	}

	// Open the listeners
	udpAddr, err := net.ResolveUDPAddr("udp", http3ServerInfo.ListenAddress)
	if err != nil {
		return err
	}
	udpConn, err := net.ListenUDP("udp", udpAddr)
	if err != nil {
		return err
	}
	defer udpConn.Close()

	tcpAddr, err := net.ResolveTCPAddr("tcp", http3ServerInfo.ListenAddress)
	if err != nil {
		return err
	}
	tcpConn, err := net.ListenTCP("tcp", tcpAddr)
	if err != nil {
		return err
	}
	defer tcpConn.Close()

	tlsConn := tls.NewListener(tcpConn, config)
	defer tlsConn.Close()

	// Start the servers
	httpServer := &http.Server{
		Addr:      http3ServerInfo.ListenAddress,
		TLSConfig: config,
	}
	http3ServerInfo.HTTPServerTimeouts.ApplyToHTTPServer(httpServer)

	quicServer := &http3.Server{
		Server: httpServer,
	}

	if http3ServerInfo.OverrideAltSvcPortValue != nil {
		quicServer.Port = *http3ServerInfo.OverrideAltSvcPortValue
	}

	httpServer.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		quicServer.SetQuicHeaders(w.Header())
		handler.ServeHTTP(w, r)
	})

	hErr := make(chan error)
	qErr := make(chan error)
	go func() {
		hErr <- httpServer.Serve(tlsConn)
	}()
	go func() {
		qErr <- quicServer.Serve(udpConn)
	}()

	select {
	case err := <-hErr:
		quicServer.Close()
		return err
	case err := <-qErr:
		// Cannot close the HTTP server or wait for requests to complete properly :/
		return err
	}
}
