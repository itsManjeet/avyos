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
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"

	"avyos.dev/api/distro"
	"avyos.dev/pkg/fs"
	"avyos.dev/pkg/pty"
)

type shellSession struct {
	id      uint32
	owner   uint32
	pty     *pty.PTY
	cmd     *exec.Cmd
	wayland *waylandBridge
	once    sync.Once
}

type shellSessionManager struct {
	handler *Handler

	mu       sync.RWMutex
	sessions map[uint32]*shellSession
	nextID   atomic.Uint32
}

func newShellSessionManager(handler *Handler) *shellSessionManager {
	m := &shellSessionManager{
		handler:  handler,
		sessions: make(map[uint32]*shellSession),
	}
	m.nextID.Store(1)
	return m
}

func (m *shellSessionManager) Open(owner, uid uint32, req distro.ShellOpenRequest) (distro.ShellSession, error) {
	rootfs := linuxBase
	if _, err := os.Stat(filepath.Join(rootfs, "bin")); os.IsNotExist(err) {
		return distro.ShellSession{}, fmt.Errorf("distro not installed (use 'distro install' first)")
	}

	workdir := strings.TrimSpace(req.Workdir)
	if workdir == "" {
		workdir = "/root"
	}

	rows := int(req.Rows)
	cols := int(req.Cols)
	if rows <= 0 {
		rows = 24
	}
	if cols <= 0 {
		cols = 80
	}

	ptyPair, err := pty.Open()
	if err != nil {
		return distro.ShellSession{}, fmt.Errorf("open pty: %w", err)
	}
	if err := ptyPair.SetSize(rows, cols); err != nil {
		_ = ptyPair.Close()
		return distro.ShellSession{}, fmt.Errorf("set pty size: %w", err)
	}

	waylandBridge, err := newWaylandBridge(uid)
	if err != nil {
		_ = ptyPair.Close()
		return distro.ShellSession{}, fmt.Errorf("setup wayland bridge: %w", err)
	}

	exePath, err := os.Readlink(fs.Resolve("process:self/exe"))
	if err != nil {
		if waylandBridge != nil {
			waylandBridge.Close()
		}
		_ = ptyPair.Close()
		return distro.ShellSession{}, fmt.Errorf("resolve executable path: %w", err)
	}

	cmd := exec.Command(exePath, "init")
	cmd.Stdin = ptyPair.Slave
	cmd.Stdout = ptyPair.Slave
	cmd.Stderr = ptyPair.Slave

	env := os.Environ()
	env = setEnv(env, "PATH", "/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin")
	env = setEnv(env, "HOME", "/root")
	env = setEnv(env, "USER", "root")
	env = setEnv(env, "LOGNAME", "root")
	env = setEnv(env, "TERM", "xterm-256color")
	env = setEnv(env, "LANG", "C.UTF-8")
	env = setEnv(env, "DISTRO_ROOTFS", rootfs)
	env = setEnv(env, "DISTRO_COMMAND", distro.EncodeCommand([]string{defaultShell}))
	env = setEnv(env, "DISTRO_WORKDIR", workdir)
	env = setEnv(env, "DISTRO_BIND", req.Bind)
	env = setEnvMany(env, waylandBaseEnv())
	if waylandBridge != nil && strings.TrimSpace(waylandBridge.RuntimeHost()) != "" {
		env = setEnv(env, distroWaylandRuntimeHostEnv, waylandBridge.RuntimeHost())
	}
	if strings.TrimSpace(req.Env) != "" {
		env = setEnvKV(env, req.Env)
	}
	cmd.Env = env

	attr := &syscall.SysProcAttr{
		Cloneflags: syscall.CLONE_NEWUTS | syscall.CLONE_NEWPID | syscall.CLONE_NEWNS,
		Setsid:     true,
		Setctty:    true,
		Ctty:       0,
	}
	if os.Geteuid() != 0 {
		attr.Cloneflags |= syscall.CLONE_NEWUSER
		attr.UidMappings = []syscall.SysProcIDMap{
			{ContainerID: 0, HostID: os.Geteuid(), Size: 1},
		}
		attr.GidMappings = []syscall.SysProcIDMap{
			{ContainerID: 0, HostID: os.Getegid(), Size: 1},
		}
		attr.GidMappingsEnableSetgroups = false
	}
	cmd.SysProcAttr = attr

	if err := cmd.Start(); err != nil {
		if waylandBridge != nil {
			waylandBridge.Close()
		}
		_ = ptyPair.Close()
		return distro.ShellSession{}, fmt.Errorf("start shell: %w", err)
	}

	_ = ptyPair.Slave.Close()

	sessionID := m.nextID.Add(1)
	session := &shellSession{
		id:      sessionID,
		owner:   owner,
		pty:     ptyPair,
		cmd:     cmd,
		wayland: waylandBridge,
	}

	m.mu.Lock()
	m.sessions[sessionID] = session
	m.mu.Unlock()

	go m.streamOutput(session)
	go m.waitProcess(session)

	return distro.ShellSession{SessionID: sessionID}, nil
}

func (m *shellSessionManager) Input(owner uint32, req distro.ShellInputRequest) error {
	session, err := m.sessionFor(owner, req.SessionID)
	if err != nil {
		return err
	}
	if len(req.Data) == 0 {
		return nil
	}
	_, err = session.pty.Write(req.Data)
	return err
}

func (m *shellSessionManager) Resize(owner uint32, req distro.ShellResizeRequest) error {
	session, err := m.sessionFor(owner, req.SessionID)
	if err != nil {
		return err
	}
	if req.Rows <= 0 || req.Cols <= 0 {
		return nil
	}
	return session.pty.SetSize(int(req.Rows), int(req.Cols))
}

func (m *shellSessionManager) Close(owner uint32, req distro.ShellCloseRequest) error {
	session, err := m.sessionFor(owner, req.SessionID)
	if err != nil {
		return err
	}
	if session.cmd.Process == nil {
		return nil
	}
	_ = session.cmd.Process.Signal(syscall.SIGTERM)
	return nil
}

func (m *shellSessionManager) CloseByOwner(owner uint32) {
	m.mu.RLock()
	sessions := make([]*shellSession, 0, len(m.sessions))
	for _, session := range m.sessions {
		if session.owner == owner {
			sessions = append(sessions, session)
		}
	}
	m.mu.RUnlock()

	for _, session := range sessions {
		_ = m.Close(owner, distro.ShellCloseRequest{SessionID: session.id})
	}
}

func (m *shellSessionManager) sessionFor(owner, sessionID uint32) (*shellSession, error) {
	if sessionID == 0 {
		return nil, fmt.Errorf("invalid shell session")
	}

	m.mu.RLock()
	session := m.sessions[sessionID]
	m.mu.RUnlock()
	if session == nil {
		return nil, fmt.Errorf("shell session not found")
	}
	if session.owner != owner {
		return nil, fmt.Errorf("shell session access denied")
	}
	return session, nil
}

func (m *shellSessionManager) streamOutput(session *shellSession) {
	buf := make([]byte, 4096)
	for {
		n, err := session.pty.Read(buf)
		if n > 0 && m.handler != nil {
			data := make([]byte, n)
			copy(data, buf[:n])
			if conn := m.handler.ConnFor(session.owner); conn != nil {
				_ = distro.SendDistroShellOutput(conn, session.owner, distro.ShellOutputEvent{
					SessionID: session.id,
					Data:      data,
				})
			}
		}
		if err != nil {
			if !errors.Is(err, io.EOF) {
				serviceLog.Debug("shell output stream closed for session %d: %v", session.id, err)
			}
			return
		}
	}
}

func (m *shellSessionManager) waitProcess(session *shellSession) {
	exitCode := 0
	if err := session.cmd.Wait(); err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			exitCode = exitErr.ExitCode()
		} else {
			exitCode = -1
		}
	} else if session.cmd.ProcessState != nil {
		exitCode = session.cmd.ProcessState.ExitCode()
	}

	m.cleanup(session.id)

	if m.handler != nil {
		if conn := m.handler.ConnFor(session.owner); conn != nil {
			_ = distro.SendDistroShellExit(conn, session.owner, distro.ShellExitEvent{
				SessionID: session.id,
				ExitCode:  int32(exitCode),
			})
		}
	}
}

func (m *shellSessionManager) cleanup(sessionID uint32) {
	m.mu.Lock()
	session := m.sessions[sessionID]
	delete(m.sessions, sessionID)
	m.mu.Unlock()
	if session == nil {
		return
	}

	session.once.Do(func() {
		if session.pty != nil {
			_ = session.pty.Close()
		}
		if session.wayland != nil {
			session.wayland.Close()
		}
	})
}
