package main

import (
	"os"

	"avyos.dev/api/service"
	"avyos.dev/lib/logger"
)

var log = logger.New("service")

func main() {
	if err := logger.SetupLog(); err != nil {
		log.Error("failed to setup log: %v", err)
	}

	srv, err := service.Listen()
	if err != nil {
		log.Error("failed to listen: %v", err)
		os.Exit(1)
	}
	defer srv.Close()

	srv.Handlers = service.Handlers{Service: &Handler{}}

	log.Info("service manager ready")
	if err := srv.Serve(); err != nil {
		log.Error("serve error: %v", err)
		os.Exit(1)
	}
}
