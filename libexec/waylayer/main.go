// Copyright (c) 2026 Manjeet Singh <itsmanjeet1998@gmail.com>.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, version 3.

package main

import (
	"errors"
	"flag"
	"fmt"
	"net"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	desktopapi "avyos.dev/api/desktop"
	"avyos.dev/lib/logger"
)

var log = logger.New("dev.avyos.waylayer")

func main() {
	defaultSocket := filepath.Join(fmt.Sprintf("/run/user/%d", os.Getuid()), "dev.avyos.waylayer")
	socket := flag.String("socket", defaultSocket, "Wayland Unix socket path")
	flag.Parse()

	if err := logger.SetupLog(); err != nil {
		fmt.Fprintf(os.Stderr, "waylayer: setup log: %v\n", err)
	}
	if err := serve(*socket); err != nil {
		log.Error("%v", err)
		fmt.Fprintf(os.Stderr, "waylayer: %v\n", err)
		os.Exit(1)
	}
}

func serve(socket string) error {
	if err := os.MkdirAll(filepath.Dir(socket), 0o700); err != nil {
		return fmt.Errorf("create socket directory: %w", err)
	}
	_ = os.Remove(socket)
	addr, err := net.ResolveUnixAddr("unix", socket)
	if err != nil {
		return fmt.Errorf("resolve socket: %w", err)
	}
	ln, err := net.ListenUnix("unix", addr)
	if err != nil {
		return fmt.Errorf("listen on %s: %w", socket, err)
	}
	defer func() {
		_ = ln.Close()
		_ = os.Remove(socket)
	}()
	if err := os.Chmod(socket, 0o600); err != nil {
		return fmt.Errorf("set socket permissions: %w", err)
	}

	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP, syscall.SIGQUIT)
	defer signal.Stop(signals)
	go func() {
		<-signals
		_ = ln.Close()
	}()

	log.Info("Wayland socket ready at %s", socket)
	for {
		conn, err := ln.AcceptUnix()
		if err != nil {
			if errors.Is(err, net.ErrClosed) {
				return nil
			}
			return fmt.Errorf("accept: %w", err)
		}
		go serveClient(conn)
	}
}

func serveClient(conn *net.UnixConn) {
	desktop, err := desktopapi.Connect()
	if err != nil {
		log.Debug("rejecting Wayland client: desktop service unavailable: %v", err)
		_ = conn.Close()
		return
	}
	c := newClient(conn, desktop)
	defer c.close()
	if err := c.run(); err != nil && !errors.Is(err, net.ErrClosed) {
		log.Debug("Wayland client disconnected: %v", err)
	}
}
