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
	"os/exec"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"

	"avyos.dev/api/dbg"
	"avyos.dev/lib/pty"
)

type shellSession struct {
	id    uint32
	owner uint32
	token string
	pty   *pty.PTY
	cmd   *exec.Cmd
	once  sync.Once
}

type shellSessionManager struct {
	handler *Handler

	mu       sync.RWMutex
	sessions map[uint32]*shellSession
	nextID   atomic.Uint32
}

func newShellSessionManager(handler *Handler) *shellSessionManager {
	return &shellSessionManager{
		handler:  handler,
		sessions: make(map[uint32]*shellSession),
	}
}

func (m *shellSessionManager) Open(owner uint32, sess *authSession, req dbg.ShellOpenRequest) (dbg.ShellSession, error) {
	if sess == nil || sess.Identity == nil {
		return dbg.ShellSession{}, errors.New("invalid auth session")
	}

	workdir, err := resolveWorkDir(sess.Identity, req.Cwd)
	if err != nil {
		return dbg.ShellSession{}, fmt.Errorf("failed to resolve workdir: %w", err)
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
		return dbg.ShellSession{}, fmt.Errorf("open pty: %w", err)
	}
	if err := ptyPair.SetSize(rows, cols); err != nil {
		_ = ptyPair.Close()
		return dbg.ShellSession{}, fmt.Errorf("set pty size: %w", err)
	}

	shellPath, err := resolveUserShell(sess)
	if err != nil {
		_ = ptyPair.Close()
		return dbg.ShellSession{}, err
	}

	cmd := exec.Command(shellPath)
	cmd.Dir = workdir
	cmd.Env = buildUserEnv(sess.Identity)
	cmd.Stdin = ptyPair.Slave
	cmd.Stdout = ptyPair.Slave
	cmd.Stderr = ptyPair.Slave
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Credential: buildCredential(sess.Identity),
		Setsid:     true,
		Setctty:    true,
		Ctty:       0,
	}

	if err := cmd.Start(); err != nil {
		_ = ptyPair.Close()
		return dbg.ShellSession{}, fmt.Errorf("start shell: %w", err)
	}
	_ = ptyPair.Slave.Close()

	sessionID := m.nextID.Add(1)
	session := &shellSession{
		id:    sessionID,
		owner: owner,
		token: sess.Token,
		pty:   ptyPair,
		cmd:   cmd,
	}

	m.mu.Lock()
	m.sessions[sessionID] = session
	m.mu.Unlock()

	go m.streamOutput(session)
	go m.waitProcess(session)

	return dbg.ShellSession{SessionID: sessionID}, nil
}

func (m *shellSessionManager) Input(owner uint32, sess *authSession, req dbg.ShellInputRequest) error {
	session, err := m.sessionFor(owner, sess, req.SessionID)
	if err != nil {
		return err
	}
	if len(req.Data) == 0 {
		return nil
	}
	_, err = session.pty.Write(req.Data)
	return err
}

func (m *shellSessionManager) Resize(owner uint32, sess *authSession, req dbg.ShellResizeRequest) error {
	session, err := m.sessionFor(owner, sess, req.SessionID)
	if err != nil {
		return err
	}
	if req.Rows <= 0 || req.Cols <= 0 {
		return nil
	}
	return session.pty.SetSize(int(req.Rows), int(req.Cols))
}

func (m *shellSessionManager) Close(owner uint32, sess *authSession, req dbg.ShellCloseRequest) error {
	session, err := m.sessionFor(owner, sess, req.SessionID)
	if err != nil {
		return err
	}
	return m.closeSession(session)
}

func (m *shellSessionManager) CloseByToken(owner uint32, token string) {
	token = strings.TrimSpace(token)
	if token == "" {
		return
	}

	m.mu.RLock()
	sessions := make([]*shellSession, 0, len(m.sessions))
	for _, session := range m.sessions {
		if session.owner == owner && session.token == token {
			sessions = append(sessions, session)
		}
	}
	m.mu.RUnlock()

	for _, session := range sessions {
		_ = m.closeSession(session)
	}
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
		_ = m.closeSession(session)
	}
}

func (m *shellSessionManager) sessionFor(owner uint32, sess *authSession, sessionID uint32) (*shellSession, error) {
	if sessionID == 0 {
		return nil, errors.New("invalid shell session")
	}
	if sess == nil {
		return nil, errors.New("invalid auth session")
	}

	m.mu.RLock()
	session := m.sessions[sessionID]
	m.mu.RUnlock()
	if session == nil {
		return nil, errors.New("shell session not found")
	}
	if session.owner != owner || session.token != sess.Token {
		return nil, errors.New("shell session access denied")
	}
	return session, nil
}

func (m *shellSessionManager) closeSession(session *shellSession) error {
	if session == nil || session.cmd == nil || session.cmd.Process == nil {
		return nil
	}
	return session.cmd.Process.Signal(syscall.SIGTERM)
}

func (m *shellSessionManager) streamOutput(session *shellSession) {
	buf := make([]byte, 4096)
	for {
		n, err := session.pty.Read(buf)
		if n > 0 && m.handler != nil {
			data := make([]byte, n)
			copy(data, buf[:n])
			if conn := m.handler.ConnFor(session.owner); conn != nil {
				_ = dbg.SendDbgShellOutput(conn, session.owner, dbg.ShellOutputEvent{
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
			_ = dbg.SendDbgShellExit(conn, session.owner, dbg.ShellExitEvent{
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
	})
}

func resolveUserShell(sess *authSession) (string, error) {
	shell := defaultShell()
	if shell == "" {
		shell = "/usr/bin/sh"
	}

	path, err := resolveExecutable(shell)
	if err != nil {
		return "", fmt.Errorf("resolve shell executable: %w", err)
	}
	return path, nil
}
