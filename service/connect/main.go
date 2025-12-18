package main

import (
	"encoding/binary"
	"flag"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net"
	"sync"

	"avyos.dev/pkg/connect"
)

var (
	session bool

	mutex   sync.Mutex
	objects = map[uint32]net.Conn{}
	clients = map[uint32]net.Conn{}
)

func init() {
	flag.BoolVar(&session, "session", false, "Listen to session bus")
}

func main() {
	flag.Parse()

	socketPath := connect.SystemBusPath
	if session {
		socketPath = connect.SessionBusPath
	}

	listener, err := net.Listen("unix", socketPath)
	if err != nil {
		log.Fatal(err)
	}

	for {
		conn, err := listener.Accept()
		if err != nil {
			continue
		}

		go handleConn(conn)
	}
}

func handleConn(conn net.Conn) {
	defer conn.Close()

	clientId := rand.Uint32()
	mutex.Lock()
	clients[clientId] = conn
	mutex.Unlock()

	connect.Write(conn, connect.Message{
		Sender:      connect.BusObjectId,
		Destination: clientId,
		Event:       connect.BusReplyEvent,
		Payload:     nil,
	})

	for {
		m, err := connect.Read(conn)
		if err != nil {
			if err == io.EOF {
				return
			}
			continue
		}

		switch m.Destination {
		case connect.BusObjectId:
			handleBusConn(conn, clientId, m)
		default:
			mutex.Lock()
			dest, ok := objects[m.Destination]
			mutex.Unlock()

			if !ok {
				mutex.Lock()
				dest, ok = clients[m.Destination]
				mutex.Unlock()
			}

			if !ok {
				busErrorf(conn, clientId, "unknown object %d", m.Destination)
				continue
			}

			if err := connect.Write(dest, m); err != nil {
				busErrorf(conn, clientId, "write failed %v", err)
				continue
			}
		}
	}
}

func busErrorf(conn net.Conn, id uint32, format string, args ...any) {
	connect.Write(conn, connect.Message{
		Sender:      connect.BusObjectId,
		Destination: id,
		Event:       connect.BusErrorEvent,
		Payload:     []byte(fmt.Sprintf(format, args...)),
	})
}

func handleBusConn(conn net.Conn, id uint32, m connect.Message) {
	switch m.Event {
	case connect.BusRegisterEvent:
		if len(m.Payload) != 4 {
			busErrorf(conn, id, "invalid payload")
			return
		}
		serviceId := binary.LittleEndian.Uint32(m.Payload)
		mutex.Lock()
		objects[serviceId] = conn
		mutex.Unlock()
		log.Printf("registering client %v at %v", id, serviceId)
	case connect.BusUnregisterEvent:
		if len(m.Payload) != 4 {
			busErrorf(conn, id, "invalid payload")
			return
		}
		serviceId := binary.LittleEndian.Uint32(m.Payload)
		mutex.Lock()
		delete(objects, serviceId)
		mutex.Unlock()
		log.Printf("unregistering client %v at %v", id, serviceId)
	case connect.BusPingEvent:
		connect.Write(conn, connect.Message{
			Sender:      connect.BusObjectId,
			Destination: id,
			Event:       connect.BusReplyEvent,
			Payload:     []byte(`pong`),
		})
	}
}
