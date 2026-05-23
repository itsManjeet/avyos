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
	"sync"
	"sync/atomic"

	"avyos.dev/api/distro"
	"avyos.dev/lib/sutra"
)

type Handler struct {
	shells *shellSessionManager

	connMu  sync.RWMutex
	conns   map[uint32]*sutra.Conn
	uids    map[uint32]uint32 // objectID -> peer UID
	nextCID atomic.Uint32
}

func newHandler() *Handler {
	h := &Handler{
		conns: make(map[uint32]*sutra.Conn),
		uids:  make(map[uint32]uint32),
	}
	h.shells = newShellSessionManager(h)
	return h
}

func (h *Handler) RegisterConn(conn *sutra.Conn, uid uint32) uint32 {
	id := h.nextCID.Add(1)
	h.connMu.Lock()
	h.conns[id] = conn
	h.uids[id] = uid
	h.connMu.Unlock()
	return id
}

func (h *Handler) ConnFor(objectID uint32) *sutra.Conn {
	h.connMu.RLock()
	c := h.conns[objectID]
	h.connMu.RUnlock()
	return c
}

func (h *Handler) UnregisterConn(objectID uint32) {
	h.connMu.Lock()
	delete(h.conns, objectID)
	delete(h.uids, objectID)
	h.connMu.Unlock()
}

func (h *Handler) Status(_ uint32) (distro.StatusResponse, error) {
	installed, path, size := distroStatus()
	installedVal := uint8(0)
	if installed {
		installedVal = 1
	}
	return distro.StatusResponse{
		Installed: installedVal,
		Path:      path,
		Size:      int64(size),
	}, nil
}

func (h *Handler) Install(_ uint32, in distro.InstallRequest) error {
	return installDistro(in.URL)
}

func (h *Handler) Run(object uint32, in distro.RunRequest) (distro.RunResult, error) {
	uid := h.callerUID(object)
	return runContainer(in, uid)
}

func (h *Handler) Remove(_ uint32) error {
	return uninstallDistro()
}

func (h *Handler) ShellOpen(object uint32, in distro.ShellOpenRequest) (distro.ShellSession, error) {
	uid := h.callerUID(object)
	return h.shells.Open(object, uid, in)
}

func (h *Handler) ShellInput(object uint32, in distro.ShellInputRequest) error {
	return h.shells.Input(object, in)
}

func (h *Handler) ShellResize(object uint32, in distro.ShellResizeRequest) error {
	return h.shells.Resize(object, in)
}

func (h *Handler) ShellClose(object uint32, in distro.ShellCloseRequest) error {
	return h.shells.Close(object, in)
}

func (h *Handler) callerUID(objectID uint32) uint32 {
	h.connMu.RLock()
	uid := h.uids[objectID]
	h.connMu.RUnlock()
	return uid
}
