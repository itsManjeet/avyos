package connect

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
)

var (
	SystemBusPath  = "/cache/runtime/bus.sock"
	SessionBusPath = filepath.Join(os.Getenv("HOME"), "/.cache/bus.sock")
)

func Connect(path string) (*Connection, error) {
	c, err := net.Dial("unix", path)
	if err != nil {
		return nil, err
	}
	conn := &Connection{conn: c}

	mesg, err := conn.Recieve()
	if err != nil {
		conn.conn.Close()
		return nil, err
	}
	if !(mesg.Sender == BusObjectId && mesg.Event == BusReplyEvent) {
		conn.conn.Close()
		return nil, fmt.Errorf("failed to get connection id")
	}
	conn.id = mesg.Destination
	return conn, nil
}

func SystemBus() (*Connection, error) {
	return Connect(SystemBusPath)
}

func SessionBus() (*Connection, error) {
	return Connect(SessionBusPath)
}
