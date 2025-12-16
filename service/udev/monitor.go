package main

import (
	"net"
	"os"
	"path/filepath"
	"sync"
	"time"
)

const (
	RuntimeDir  = "/cache/udev"
	DeadlineSec = 5
)

type Monitor struct {
	listener *net.UnixListener
	clients  map[*net.UnixConn]struct{}
	mutex    sync.Mutex
}

func NewMonitor() (*Monitor, error) {
	_ = os.MkdirAll(RuntimeDir, 0755)

	monitorSocketPath := filepath.Join(RuntimeDir, "monitor")
	_ = os.Remove(monitorSocketPath)

	l, err := net.ListenUnix("unixpacket", &net.UnixAddr{
		Name: monitorSocketPath,
		Net:  "unixpacket",
	})
	if err != nil {
		return nil, err
	}
	m := &Monitor{
		listener: l,
		clients:  map[*net.UnixConn]struct{}{},
	}

	go m.acceptLoop()

	return m, nil
}

func (m *Monitor) acceptLoop() {
	for {
		conn, err := m.listener.AcceptUnix()
		if err != nil {
			continue
		}

		m.mutex.Lock()
		m.clients[conn] = struct{}{}
		m.mutex.Unlock()

		go m.handleClient(conn)
	}
}

func (m *Monitor) handleClient(conn *net.UnixConn) {
	defer func() {
		conn.Close()
		m.mutex.Lock()
		delete(m.clients, conn)
		m.mutex.Unlock()
	}()

	buf := make([]byte, 1024)
	conn.SetReadDeadline(time.Now().Add(DeadlineSec * time.Second))
	_, _ = conn.Read(buf)

	select {}
}
