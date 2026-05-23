/*
 * Copyright (c) 2026 Manjeet Singh <itsmanjeet1998@gmail.com>.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU General Public License as published by
 * the Free Software Foundation, version 3.
 *
 * This program is distributed in the hope that it will be useful, but
 * WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the GNU
 * General Public License for more details.
 *
 * You should have received a copy of the GNU General Public License
 * along with this program. If not, see <http://www.gnu.org/licenses/>.
 *
 */

package main

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	"avyos.dev/pkg/fs"
)

const (
	distroWaylandRuntime        = "/run/wayland"
	distroWaylandDisplay        = "waylayer"
	distroWaylandRuntimeHostEnv = "DISTRO_WAYLAND_RUNTIME_HOST"
)

// waylandBridge relays Wayland connections from inside the container to
// the session-level waylayer running at the user's XDG_RUNTIME_DIR.
type waylandBridge struct {
	runtimeHost    string
	socketHost     string
	upstreamSocket string

	listener *net.UnixListener

	closeOnce sync.Once
	closed    chan struct{}
}

// newWaylandBridge creates a bridge that relays container Wayland clients
// to the session waylayer socket at /cache/runtime/<uid>/waylayer.
func newWaylandBridge(uid uint32) (*waylandBridge, error) {
	upstreamSocket := fs.Resolve("user:dev.avyos.waylayer")
	if _, err := os.Stat(upstreamSocket); err != nil {
		return nil, fmt.Errorf("session waylayer socket not found at %s: %w", upstreamSocket, err)
	}

	runtimeRoot := fs.Resolve("system:distro/wayland")
	if err := os.MkdirAll(runtimeRoot, 0755); err != nil {
		return nil, fmt.Errorf("create distro wayland system root: %w", err)
	}

	runtimeHost, err := os.MkdirTemp(runtimeRoot, "session-")
	if err != nil {
		return nil, fmt.Errorf("create distro wayland runtime: %w", err)
	}
	if err := os.MkdirAll(runtimeHost, 0777); err != nil {
		_ = os.RemoveAll(runtimeHost)
		return nil, fmt.Errorf("create distro wayland runtime: %w", err)
	}
	_ = os.Chmod(runtimeHost, 0777)

	socketHost := filepath.Join(runtimeHost, distroWaylandDisplay)
	_ = os.Remove(socketHost)

	addr, err := net.ResolveUnixAddr("unix", socketHost)
	if err != nil {
		_ = os.RemoveAll(runtimeHost)
		return nil, fmt.Errorf("resolve distro wayland socket: %w", err)
	}

	ln, err := net.ListenUnix("unix", addr)
	if err != nil {
		_ = os.RemoveAll(runtimeHost)
		return nil, fmt.Errorf("listen distro wayland socket: %w", err)
	}
	_ = os.Chmod(socketHost, 0666)

	b := &waylandBridge{
		runtimeHost:    runtimeHost,
		socketHost:     socketHost,
		upstreamSocket: upstreamSocket,
		listener:       ln,
		closed:         make(chan struct{}),
	}

	go b.acceptLoop()
	return b, nil
}

func (b *waylandBridge) RuntimeHost() string {
	if b == nil {
		return ""
	}
	return b.runtimeHost
}

func waylandBaseEnv() []string {
	return []string{
		"XDG_RUNTIME_DIR=" + distroWaylandRuntime,
		"WAYLAND_DISPLAY=" + distroWaylandDisplay,
		"XDG_SESSION_TYPE=wayland",
		"XDG_CURRENT_DESKTOP=avyos",
		"GDK_BACKEND=wayland",
		"QT_QPA_PLATFORM=wayland",
		"SDL_VIDEODRIVER=wayland",
		"EGL_PLATFORM=wayland",
		"MOZ_ENABLE_WAYLAND=1",
	}
}

func (b *waylandBridge) Env() []string {
	return waylandBaseEnv()
}

func (b *waylandBridge) Close() {
	b.closeOnce.Do(func() {
		close(b.closed)
		if b.listener != nil {
			_ = b.listener.Close()
		}
		if b.socketHost != "" {
			_ = os.Remove(b.socketHost)
		}
		if b.runtimeHost != "" {
			_ = os.RemoveAll(b.runtimeHost)
		}
	})
}

func (b *waylandBridge) acceptLoop() {
	for {
		conn, err := b.listener.AcceptUnix()
		if err != nil {
			select {
			case <-b.closed:
				return
			default:
			}
			if errors.Is(err, net.ErrClosed) {
				return
			}
			continue
		}
		go b.handleConn(conn)
	}
}

func (b *waylandBridge) handleConn(clientConn *net.UnixConn) {
	upstreamConn, err := b.connectUpstream()
	if err != nil {
		serviceLog.Debug("wayland bridge upstream connect failed: %v", err)
		_ = clientConn.Close()
		return
	}

	done := make(chan struct{}, 2)
	go relayWaylandStream(upstreamConn, clientConn, done)
	go relayWaylandStream(clientConn, upstreamConn, done)
	<-done
	_ = clientConn.Close()
	_ = upstreamConn.Close()
}

func relayWaylandStream(dst, src *net.UnixConn, done chan<- struct{}) {
	defer func() { done <- struct{}{} }()

	for {
		msg, fds, err := recvWaylandMessage(src)
		if err != nil {
			return
		}
		if err := sendWaylandMessage(dst, msg, fds); err != nil {
			return
		}
	}
}

func recvWaylandMessage(conn *net.UnixConn) ([]byte, []int, error) {
	const waylandHeaderSize = 8

	header := make([]byte, waylandHeaderSize)
	fds, err := recvWaylandFull(conn, header)
	if err != nil {
		return nil, nil, err
	}

	size := int(binary.LittleEndian.Uint32(header[4:8]) >> 16)
	if size < waylandHeaderSize || size > 65535 {
		closeFDList(fds)
		return nil, nil, fmt.Errorf("invalid wayland message size %d", size)
	}

	msg := make([]byte, size)
	copy(msg, header)
	if size > waylandHeaderSize {
		payloadFDs, err := recvWaylandFull(conn, msg[waylandHeaderSize:])
		if err != nil {
			closeFDList(fds)
			return nil, nil, err
		}
		fds = append(fds, payloadFDs...)
	}

	return msg, fds, nil
}

func recvWaylandFull(conn *net.UnixConn, buf []byte) ([]int, error) {
	allFDs := make([]int, 0)
	offset := 0

	for offset < len(buf) {
		n, fds, err := recvWithFDs(conn, buf[offset:])
		if len(fds) > 0 {
			allFDs = append(allFDs, fds...)
		}
		if err != nil {
			closeFDList(allFDs)
			return nil, err
		}
		if n == 0 {
			closeFDList(allFDs)
			return nil, io.EOF
		}
		offset += n
	}

	return allFDs, nil
}

func recvWithFDs(conn *net.UnixConn, buf []byte) (int, []int, error) {
	rawConn, err := conn.SyscallConn()
	if err != nil {
		return 0, nil, err
	}

	oob := make([]byte, syscall.CmsgSpace(4*16))
	var n, oobn, flags int
	var recvErr error

	err = rawConn.Read(func(fd uintptr) bool {
		n, oobn, flags, _, recvErr = syscall.Recvmsg(int(fd), buf, oob, 0)
		if recvErr == syscall.EINTR || recvErr == syscall.EAGAIN {
			return false
		}
		return true
	})
	if err != nil {
		return 0, nil, err
	}
	if recvErr != nil {
		return 0, nil, recvErr
	}
	if flags&syscall.MSG_CTRUNC != 0 {
		return 0, nil, fmt.Errorf("wayland ancillary data truncated")
	}

	return n, parseControlFDs(oob[:oobn]), nil
}

func sendWaylandMessage(conn *net.UnixConn, msg []byte, fds []int) error {
	defer closeFDList(fds)

	rawConn, err := conn.SyscallConn()
	if err != nil {
		return err
	}

	var rights []byte
	if len(fds) > 0 {
		rights = syscall.UnixRights(fds...)
	}

	var sendErr error
	err = rawConn.Write(func(fd uintptr) bool {
		sendErr = syscall.Sendmsg(int(fd), msg, rights, nil, 0)
		if sendErr == syscall.EINTR || sendErr == syscall.EAGAIN {
			return false
		}
		return true
	})
	if err != nil {
		return err
	}
	return sendErr
}

func parseControlFDs(control []byte) []int {
	if len(control) == 0 {
		return nil
	}

	msgs, err := syscall.ParseSocketControlMessage(control)
	if err != nil {
		return nil
	}

	var fds []int
	for _, msg := range msgs {
		parsed, err := syscall.ParseUnixRights(&msg)
		if err != nil {
			continue
		}
		fds = append(fds, parsed...)
	}
	return fds
}

func closeFDList(fds []int) {
	for _, fd := range fds {
		_ = syscall.Close(fd)
	}
}

func (b *waylandBridge) connectUpstream() (*net.UnixConn, error) {
	var lastErr error
	for range 20 {
		addr, err := net.ResolveUnixAddr("unix", b.upstreamSocket)
		if err != nil {
			lastErr = err
			time.Sleep(50 * time.Millisecond)
			continue
		}
		conn, err := net.DialUnix("unix", nil, addr)
		if err == nil {
			return conn, nil
		}
		lastErr = err
		time.Sleep(50 * time.Millisecond)
	}

	if lastErr == nil {
		lastErr = fmt.Errorf("timeout")
	}
	return nil, fmt.Errorf("connect session waylayer: %w", lastErr)
}
