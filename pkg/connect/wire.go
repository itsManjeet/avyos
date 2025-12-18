package connect

import (
	"encoding/binary"
	"errors"
	"io"
	"net"
	"sync"
	"unsafe"
)

const (
	BusObjectId        = 0
	BusErrorEvent      = 0
	BusReplyEvent      = 1
	BusRegisterEvent   = 2
	BusUnregisterEvent = 3
	BusPingEvent       = 4
)

type Callback func([]byte)

type Connection struct {
	id        uint32
	conn      net.Conn
	mutex     sync.Mutex
	callbacks map[uint16]Callback
}

func (c *Connection) Register(id uint32) error {
	buf := make([]byte, unsafe.Sizeof(uint32(0)))
	binary.LittleEndian.PutUint32(buf[:4], id)

	return Write(c.conn, Message{
		Destination: BusObjectId,
		Sender:      c.id,
		Event:       BusRegisterEvent,
		Payload:     buf,
	})
}

func (c *Connection) Unregister(id uint32) error {
	buf := make([]byte, unsafe.Sizeof(uint32(0)))
	binary.LittleEndian.PutUint32(buf[:4], id)

	return Write(c.conn, Message{
		Destination: BusObjectId,
		Sender:      c.id,
		Event:       BusUnregisterEvent,
		Payload:     buf,
	})
}

func (c *Connection) Call(object uint32, event uint16, payload []byte) ([]byte, error) {
	if err := Write(c.conn, Message{
		Destination: object,
		Sender:      c.id,
		Event:       event,
		Payload:     payload,
	}); err != nil {
		return nil, err
	}

	reply, err := Read(c.conn)
	if err != nil {
		return nil, err
	}

	if reply.Event == BusErrorEvent {
		return nil, errors.New(string(reply.Payload))
	}
	return reply.Payload, nil
}

func (c *Connection) Recieve() (Message, error) {
	return Read(c.conn)
}

func (c *Connection) Send(m Message) error {
	m.Sender = c.id
	return Write(c.conn, m)
}

func (c *Connection) Bind(event uint16, callback Callback) {
	c.callbacks[event] = callback
}

func (c *Connection) Listen() error {
	for {
		m, err := Read(c.conn)
		if err != nil {
			if err == io.EOF {
				return nil
			}
			return err
		}

		c.mutex.Lock()
		f, ok := c.callbacks[m.Event]
		c.mutex.Unlock()
		if !ok {
			continue
		}

		f(m.Payload)
	}
}
