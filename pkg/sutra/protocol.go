package sutra

import (
	"bytes"
	"encoding/binary"
	"encoding/gob"
	"fmt"
	"io"
	"sync"
)

var ByteOrder = binary.LittleEndian

const HeaderSize = 8 // Object(4) + Event(2) + Size(2)

type Transaction struct {
	Object  uint32
	Event   uint16
	Payload []byte
}

// MarshalPayload serializes a value into a payload byte slice using gob.
func MarshalPayload[T any](val T) ([]byte, error) {
	var buf bytes.Buffer
	if err := gob.NewEncoder(&buf).Encode(&val); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// UnmarshalPayload deserializes a payload byte slice into a typed value using gob.
func UnmarshalPayload[T any](data []byte) (T, error) {
	var val T
	err := gob.NewDecoder(bytes.NewReader(data)).Decode(&val)
	return val, err
}

// Encode builds a full wire message: [Object:4][Event:2][Size:2][Payload:N]
// Retained for compatibility; MarshalPayload is the primary serialization path.
func Encode[T any](object uint32, event uint16, payload T) ([]byte, error) {
	pbytes, err := MarshalPayload(payload)
	if err != nil {
		return nil, fmt.Errorf("sutra: encode payload: %w", err)
	}
	size := uint16(len(pbytes))
	buf := bytes.NewBuffer(make([]byte, 0, HeaderSize+int(size)))
	binary.Write(buf, ByteOrder, object)
	binary.Write(buf, ByteOrder, event)
	binary.Write(buf, ByteOrder, size)
	buf.Write(pbytes)
	return buf.Bytes(), nil
}

// Decode parses a full wire message back into its components.
func Decode[T any](data []byte) (object uint32, event uint16, payload T, err error) {
	if len(data) < HeaderSize {
		err = fmt.Errorf("sutra: message too short: %d bytes", len(data))
		return
	}
	r := bytes.NewReader(data)
	binary.Read(r, ByteOrder, &object)
	binary.Read(r, ByteOrder, &event)
	var size uint16
	binary.Read(r, ByteOrder, &size)
	rest := make([]byte, size)
	if _, err = io.ReadFull(r, rest); err != nil {
		return
	}
	payload, err = UnmarshalPayload[T](rest)
	return
}

// Conn wraps an io.ReadWriter for sending and receiving Transactions.
//
// Two usage patterns are supported:
//
//  1. Server-side: a single goroutine calls Recv in a loop, then Send for
//     the response. No concurrency on the read path per connection.
//
//  2. Client-side: a background goroutine calls Recv for events while the
//     foreground calls SendRecv for request/response pairs. SendRecv
//     registers a pending channel before sending; the event loop calls
//     Route after each Recv to deliver the message to the right waiter or
//     fall through to the event dispatcher.
//
// Send is protected by a write mutex so it is safe to call from multiple
// goroutines (e.g. both request responses and async event pushes on the
// server side).
type Conn struct {
	rw      io.ReadWriter
	writeMu sync.Mutex

	pendingMu sync.Mutex
	pending   map[uint16]chan Transaction
}

func NewConn(rw io.ReadWriter) *Conn {
	return &Conn{
		rw:      rw,
		pending: make(map[uint16]chan Transaction),
	}
}

func (c *Conn) writeMsg(t Transaction) error {
	var hdr [HeaderSize]byte
	ByteOrder.PutUint32(hdr[0:4], t.Object)
	ByteOrder.PutUint16(hdr[4:6], t.Event)
	ByteOrder.PutUint16(hdr[6:8], uint16(len(t.Payload)))
	if _, err := c.rw.Write(hdr[:]); err != nil {
		return err
	}
	if len(t.Payload) > 0 {
		_, err := c.rw.Write(t.Payload)
		return err
	}
	return nil
}

func (c *Conn) readMsg() (Transaction, error) {
	var hdr [HeaderSize]byte
	if _, err := io.ReadFull(c.rw, hdr[:]); err != nil {
		return Transaction{}, err
	}
	t := Transaction{
		Object: ByteOrder.Uint32(hdr[0:4]),
		Event:  ByteOrder.Uint16(hdr[4:6]),
	}
	size := ByteOrder.Uint16(hdr[6:8])
	if size > 0 {
		t.Payload = make([]byte, size)
		if _, err := io.ReadFull(c.rw, t.Payload); err != nil {
			return Transaction{}, err
		}
	}
	return t, nil
}

// Send writes a transaction. Safe to call from multiple goroutines.
func (c *Conn) Send(t Transaction) error {
	c.writeMu.Lock()
	defer c.writeMu.Unlock()
	return c.writeMsg(t)
}

// Recv reads the next incoming transaction. Not safe to call concurrently
// with itself; intended for single-goroutine server loops or client event
// loops that pair with SendRecv via Route.
func (c *Conn) Recv() (Transaction, error) {
	return c.readMsg()
}

// SendRecv sends a request and waits for its paired response.
// It registers a pending channel keyed by the request opcode before
// sending, so the client event loop (calling Recv + Route) can deliver
// the response without blocking the read path.
func (c *Conn) SendRecv(req Transaction) (Transaction, error) {
	ch := make(chan Transaction, 1)
	c.pendingMu.Lock()
	c.pending[req.Event] = ch
	c.pendingMu.Unlock()

	c.writeMu.Lock()
	err := c.writeMsg(req)
	c.writeMu.Unlock()

	if err != nil {
		c.pendingMu.Lock()
		delete(c.pending, req.Event)
		c.pendingMu.Unlock()
		return Transaction{}, err
	}

	return <-ch, nil
}

// Route checks whether tx is a response to a pending SendRecv call.
// Returns true and delivers the transaction if so; returns false if the
// caller should treat it as an incoming event.
func (c *Conn) Route(tx Transaction) bool {
	c.pendingMu.Lock()
	ch, ok := c.pending[tx.Event]
	if ok {
		delete(c.pending, tx.Event)
	}
	c.pendingMu.Unlock()
	if ok {
		ch <- tx
		return true
	}
	return false
}

func (c *Conn) Close() error {
	if closer, ok := c.rw.(io.Closer); ok {
		return closer.Close()
	}
	return nil
}

// EventDispatcher handles incoming event transactions.
// HandleEvent returns (true, nil) if the opcode was recognized (even if no
// callback is registered), (false, nil) if unrecognized, or (_, err) on
// decode failure.
type EventDispatcher interface {
	HandleEvent(tx Transaction) (bool, error)
}

// DispatchEvent tries each dispatcher in order until one handles the event.
func DispatchEvent(tx Transaction, dispatchers ...EventDispatcher) error {
	for _, d := range dispatchers {
		handled, err := d.HandleEvent(tx)
		if err != nil {
			return err
		}
		if handled {
			return nil
		}
	}
	return fmt.Errorf("sutra: unhandled event: %d", tx.Event)
}
