package main

import (
	"encoding/binary"
	"fmt"
	"io"
	"math"
	"net"
	"sync"
	"syscall"
)

const waylandHeaderSize = 8

type wireConn struct {
	conn *net.UnixConn
	mu   sync.Mutex
}

type wireMessage struct {
	object uint32
	opcode uint16
	data   []byte
	fds    []int
	fdPos  int
	pos    int
}

func (w *wireConn) close() error { return w.conn.Close() }

func (w *wireConn) recv() (*wireMessage, error) {
	header := make([]byte, waylandHeaderSize)
	fds, err := recvWaylandFull(w.conn, header)
	if err != nil {
		return nil, err
	}
	word := binary.LittleEndian.Uint32(header[4:])
	size := int(word >> 16)
	if size < waylandHeaderSize || size > 65535 || size%4 != 0 {
		closeFDs(fds)
		return nil, fmt.Errorf("invalid Wayland message size %d", size)
	}
	data := make([]byte, size-waylandHeaderSize)
	if len(data) > 0 {
		moreFDs, err := recvWaylandFull(w.conn, data)
		if err != nil {
			closeFDs(fds)
			return nil, err
		}
		fds = append(fds, moreFDs...)
	}
	return &wireMessage{
		object: binary.LittleEndian.Uint32(header),
		opcode: uint16(word),
		data:   data,
		fds:    fds,
	}, nil
}

func (m *wireMessage) close() { closeFDs(m.fds[m.fdPos:]) }

func (m *wireMessage) uint() (uint32, error) {
	if m.pos+4 > len(m.data) {
		return 0, io.ErrUnexpectedEOF
	}
	v := binary.LittleEndian.Uint32(m.data[m.pos:])
	m.pos += 4
	return v, nil
}

func (m *wireMessage) int() (int32, error) {
	v, err := m.uint()
	return int32(v), err
}

func (m *wireMessage) fixed() (float64, error) {
	v, err := m.int()
	return float64(v) / 256, err
}

func (m *wireMessage) string() (string, error) {
	n, err := m.uint()
	if err != nil {
		return "", err
	}
	if n == 0 {
		return "", nil
	}
	padded := (int(n) + 3) &^ 3
	if int(n) > len(m.data)-m.pos || padded > len(m.data)-m.pos {
		return "", io.ErrUnexpectedEOF
	}
	b := m.data[m.pos : m.pos+int(n)]
	m.pos += padded
	if b[len(b)-1] != 0 {
		return "", fmt.Errorf("Wayland string is not NUL terminated")
	}
	return string(b[:len(b)-1]), nil
}

func (m *wireMessage) fd() (int, error) {
	if m.fdPos >= len(m.fds) {
		return -1, fmt.Errorf("missing Wayland file descriptor")
	}
	fd := m.fds[m.fdPos]
	m.fdPos++
	return fd, nil
}

func (m *wireMessage) done() error {
	if m.pos != len(m.data) {
		return fmt.Errorf("%d trailing request bytes", len(m.data)-m.pos)
	}
	if m.fdPos != len(m.fds) {
		return fmt.Errorf("%d unused request file descriptors", len(m.fds)-m.fdPos)
	}
	return nil
}

type wireBuilder struct{ data []byte }

func (b *wireBuilder) uint(v uint32) { b.data = binary.LittleEndian.AppendUint32(b.data, v) }
func (b *wireBuilder) int(v int32)   { b.uint(uint32(v)) }
func (b *wireBuilder) fixed(v float64) {
	b.int(int32(math.Round(v * 256)))
}
func (b *wireBuilder) string(v string) {
	n := len(v) + 1
	b.uint(uint32(n))
	b.data = append(b.data, v...)
	b.data = append(b.data, 0)
	for len(b.data)%4 != 0 {
		b.data = append(b.data, 0)
	}
}
func (b *wireBuilder) array(v []byte) {
	b.uint(uint32(len(v)))
	b.data = append(b.data, v...)
	for len(b.data)%4 != 0 {
		b.data = append(b.data, 0)
	}
}

func (w *wireConn) send(object uint32, opcode uint16, b wireBuilder, fds ...int) error {
	size := waylandHeaderSize + len(b.data)
	if size > 65535 || size%4 != 0 {
		return fmt.Errorf("invalid outgoing Wayland message size %d", size)
	}
	msg := make([]byte, size)
	binary.LittleEndian.PutUint32(msg, object)
	binary.LittleEndian.PutUint32(msg[4:], uint32(size)<<16|uint32(opcode))
	copy(msg[8:], b.data)

	w.mu.Lock()
	defer w.mu.Unlock()
	return sendWayland(w.conn, msg, fds)
}

func recvWaylandFull(conn *net.UnixConn, buf []byte) ([]int, error) {
	var fds []int
	for offset := 0; offset < len(buf); {
		n, got, err := recvWithFDs(conn, buf[offset:])
		fds = append(fds, got...)
		if err != nil {
			closeFDs(fds)
			return nil, err
		}
		if n == 0 {
			closeFDs(fds)
			return nil, io.EOF
		}
		offset += n
	}
	return fds, nil
}

func recvWithFDs(conn *net.UnixConn, buf []byte) (int, []int, error) {
	raw, err := conn.SyscallConn()
	if err != nil {
		return 0, nil, err
	}
	oob := make([]byte, syscall.CmsgSpace(4*16))
	var n, oobn, flags int
	var recvErr error
	err = raw.Read(func(fd uintptr) bool {
		n, oobn, flags, _, recvErr = syscall.Recvmsg(int(fd), buf, oob, 0)
		return recvErr != syscall.EINTR && recvErr != syscall.EAGAIN
	})
	if err != nil {
		return 0, nil, err
	}
	if recvErr != nil {
		return 0, nil, recvErr
	}
	if flags&syscall.MSG_CTRUNC != 0 {
		return 0, nil, fmt.Errorf("Wayland ancillary data truncated")
	}
	msgs, err := syscall.ParseSocketControlMessage(oob[:oobn])
	if err != nil {
		return 0, nil, err
	}
	var fds []int
	for _, msg := range msgs {
		got, err := syscall.ParseUnixRights(&msg)
		if err != nil {
			closeFDs(fds)
			return 0, nil, err
		}
		fds = append(fds, got...)
	}
	return n, fds, nil
}

func sendWayland(conn *net.UnixConn, msg []byte, fds []int) error {
	raw, err := conn.SyscallConn()
	if err != nil {
		return err
	}
	var rights []byte
	if len(fds) > 0 {
		rights = syscall.UnixRights(fds...)
	}
	var sendErr error
	err = raw.Write(func(fd uintptr) bool {
		sendErr = syscall.Sendmsg(int(fd), msg, rights, nil, 0)
		return sendErr != syscall.EINTR && sendErr != syscall.EAGAIN
	})
	if err != nil {
		return err
	}
	return sendErr
}

func closeFDs(fds []int) {
	for _, fd := range fds {
		_ = syscall.Close(fd)
	}
}
