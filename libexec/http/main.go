package main

import (
	"flag"
	"net/http"
	"os"

	"avyos.dev/lib/logger"
)

var (
	configfile string
	log        *logger.Logger
)

func init() {
	log = logger.New("dev.avyos.http")
	_ = logger.SetupLog()

	flag.StringVar(&configfile, "config", "/etc/http.conf", "Configuration file")
}

func main() {
	flag.Parse()

	config, err := ParseConfig(configfile)
	if err != nil {
		log.Error("failed to load config file %v", err)
		os.Exit(1)
	}

	server, err := NewServer(config)
	if err != nil {
		log.Error("failed to create server %v", err)
		os.Exit(1)
	}

	go func() {
		log.Info("Listening on %s", config.Listen)
		http.ListenAndServe(config.Listen, server)
	}()

	tlsServer := http.Server{
		Addr:      config.ListenTLS,
		TLSConfig: server.TLSConfig(),
	}

	log.Info("Listening on %s", config.ListenTLS)
	tlsServer.ListenAndServeTLS("", "")
}
