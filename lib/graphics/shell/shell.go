// Copyright (c) 2026 Manjeet Singh <itsmanjeet1998@gmail.com>.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, version 3.
//
// This program is distributed in the hope that it will be useful, but
// WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the GNU
// General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with this program. If not, see <http://www.gnu.org/licenses/>.

// Package shell bridges the desktop Sutra protocol with the graphics/widget system.
// It provides Controller (service lifecycle + window registry) and Window (window model + widget).
// Everything in this package is infrastructure. Policy lives in libexec/desktop.
package shell

import (
	"fmt"
	"image"
	"net"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"

	"avyos.dev/api/desktop"
	"avyos.dev/lib/graphics/event"
	"avyos.dev/lib/sutra"
)

// windowKey uniquely identifies a window by the client that owns it plus its local ID.
type windowKey struct {
	clientID uint32
	windowID uint32
}

// Controller manages the desktop Sutra service and maintains the window registry.
type Controller struct {
	// OnCreate is called after a new Window is registered. Return a non-nil error
	// to reject the creation; the window will be removed and an error sent to the client.
	OnCreate func(*Window) error
	// OnBuffer is called when a window has updated its shared-memory buffer.
	OnBuffer func(*Window)
	// OnState is called when a window's title/subtitle/status changes.
	OnState func(*Window)
	// OnPresent is called after a window presents a new frame (one-way, no reply needed).
	OnPresent func(*Window)
	// OnDestroy is called just before the window is removed from the registry.
	OnDestroy func(*Window)
	// OnNotification is called when a client sends a notification request.
	OnNotification func(object uint32, req desktop.NotificationRequest) error
	// OnCursor is called when a client updates its preferred cursor shape.
	OnCursor func(*Window, event.CursorShape)
	// Notify is called whenever the window list may have changed (create/destroy/state).
	Notify func()

	ln     net.Listener
	nextID atomic.Uint32

	connMu  sync.RWMutex
	conns   map[uint32]*sutra.Conn
	nextCID atomic.Uint32

	mu   sync.Mutex
	wins map[windowKey]*Window
}

// NewController creates an uninitialised Controller. Call Start to begin serving.
func NewController() *Controller {
	c := &Controller{
		wins:  make(map[windowKey]*Window),
		conns: make(map[uint32]*sutra.Conn),
	}
	c.nextID.Store(1)
	c.nextCID.Store(1)
	return c
}

// Start creates and binds the desktop service socket. socketPath may be empty to
// use the default system socket derived from desktop.ServiceName.
func (c *Controller) Start(socketPath string) error {
	if socketPath == "" {
		socketPath = filepath.Join("/run", desktop.ServiceName)
	}
	_ = os.Remove(socketPath)
	ln, err := net.Listen("unix", socketPath)
	if err != nil {
		return fmt.Errorf("shell: start service: %w", err)
	}
	_ = os.Chmod(socketPath, 0o666)
	c.ln = ln
	go func() {
		for {
			nc, err := ln.Accept()
			if err != nil {
				return
			}
			go c.serveConn(nc)
		}
	}()
	return nil
}

func (c *Controller) serveConn(nc net.Conn) {
	conn := sutra.NewConn(nc)
	defer conn.Close()

	id := c.nextCID.Add(1)
	c.connMu.Lock()
	c.conns[id] = conn
	c.connMu.Unlock()
	defer func() {
		c.connMu.Lock()
		delete(c.conns, id)
		c.connMu.Unlock()
		c.removeWindowsByOwner(id)
	}()

	for {
		tx, err := conn.Recv()
		if err != nil {
			return
		}
		tx.Object = id
		if err := desktop.Dispatch(desktop.Handlers{Desktop: c}, conn, tx); err != nil {
			return
		}
	}
}

// ConnFor returns the connection for the given client ID.
func (c *Controller) ConnFor(clientID uint32) *sutra.Conn {
	c.connMu.RLock()
	conn := c.conns[clientID]
	c.connMu.RUnlock()
	return conn
}

// Stop shuts down the service listener.
func (c *Controller) Stop() error {
	if c.ln == nil {
		return nil
	}
	return c.ln.Close()
}

// Windows returns a point-in-time snapshot of all managed windows.
func (c *Controller) Windows() []*Window {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make([]*Window, 0, len(c.wins))
	for _, w := range c.wins {
		out = append(out, w)
	}
	return out
}

func (c *Controller) window(clientID, windowID uint32) *Window {
	return c.wins[windowKey{clientID, windowID}]
}

func (c *Controller) notify() {
	if c.Notify != nil {
		c.Notify()
	}
}

func (c *Controller) removeWindowsByOwner(clientID uint32) {
	c.mu.Lock()
	var toRemove []*Window
	for key, w := range c.wins {
		if key.clientID == clientID {
			delete(c.wins, key)
			toRemove = append(toRemove, w)
		}
	}
	c.mu.Unlock()
	for _, w := range toRemove {
		if c.OnDestroy != nil {
			c.OnDestroy(w)
		}
	}
	if len(toRemove) > 0 {
		c.notify()
	}
}

// ─── DesktopHandler interface ─────────────────────────────────────────────────

func (c *Controller) CreateWindow(object uint32, in desktop.WindowCreateRequest) (desktop.WindowCreateResponse, error) {
	id := c.nextID.Add(1)

	surf := newSurface()
	w := &Window{
		ID:        id,
		ClientID:  object,
		AppID:     in.AppId,
		AppName:   in.AppName,
		Icon:      in.Icon,
		Accent:    colorFromUint32(in.Accent),
		Title:     in.Title,
		Subtitle:  in.Subtitle,
		Status:    in.Status,
		Width:     int(in.Width),
		Height:    int(in.Height),
		MinWidth:  int(in.MinWidth),
		MinHeight: int(in.MinHeight),
		Scale:     int(in.ScaleMilli),
		surface:   surf,
		ctrl:      c,
	}

	if in.BufferPath != "" {
		if err := surf.setBuffer(in.BufferPath, int(in.Width), int(in.Height), int(in.ScaleMilli)); err != nil {
			surf.Close()
			return desktop.WindowCreateResponse{}, fmt.Errorf("shell: set initial buffer: %w", err)
		}
	}

	if c.OnCreate != nil {
		if err := c.OnCreate(w); err != nil {
			surf.Close()
			return desktop.WindowCreateResponse{}, fmt.Errorf("shell: OnCreate rejected: %w", err)
		}
	}

	c.mu.Lock()
	c.wins[windowKey{object, id}] = w
	c.mu.Unlock()

	c.notify()

	return desktop.WindowCreateResponse{WindowId: id}, nil
}

func (c *Controller) DestroyWindow(object uint32, in desktop.WindowRequest) error {
	key := windowKey{object, in.WindowId}
	c.mu.Lock()
	w := c.wins[key]
	if w != nil {
		delete(c.wins, key)
	}
	c.mu.Unlock()

	if w != nil && c.OnDestroy != nil {
		c.OnDestroy(w)
	}

	c.notify()
	return nil
}

func (c *Controller) PresentWindow(object uint32, in desktop.WindowPresentRequest) error {
	c.mu.Lock()
	w := c.window(object, in.WindowId)
	c.mu.Unlock()

	if w != nil {
		if in.BufferPath != "" && in.BufferPath != w.lastBufferPath {
			_ = w.surface.setBuffer(in.BufferPath, w.Width, w.Height, w.Scale)
			w.lastBufferPath = in.BufferPath
		}

		if len(in.DamageRects) > 0 {
			w.Damage = w.Damage[:0]
			for _, r := range in.DamageRects {
				if r.W > 0 && r.H > 0 {
					w.Damage = append(w.Damage, image.Rect(
						int(r.X), int(r.Y), int(r.X+r.W), int(r.Y+r.H),
					))
				}
			}
		} else {
			w.Damage = w.Damage[:0]
		}

		if c.OnPresent != nil {
			c.OnPresent(w)
		}
	}

	return nil
}

func (c *Controller) SendNotification(object uint32, in desktop.NotificationRequest) error {
	if c.OnNotification != nil {
		return c.OnNotification(object, in)
	}
	return nil
}

func (c *Controller) UpdateWindowBuffer(object uint32, in desktop.WindowBufferRequest) error {
	w := c.findByWindowID(in.WindowId)
	if w == nil {
		return fmt.Errorf("shell: window %d not found", in.WindowId)
	}

	if err := w.surface.setBuffer(in.BufferPath, int(in.Width), int(in.Height), int(in.ScaleMilli)); err != nil {
		return fmt.Errorf("shell: set buffer: %w", err)
	}
	w.Width = int(in.Width)
	w.Height = int(in.Height)
	w.Scale = int(in.ScaleMilli)

	if c.OnBuffer != nil {
		c.OnBuffer(w)
	}

	return nil
}

func (c *Controller) UpdateWindowState(object uint32, in desktop.WindowStateRequest) error {
	w := c.findByWindowID(in.WindowId)
	if w == nil {
		return fmt.Errorf("shell: window %d not found", in.WindowId)
	}

	w.Title = in.Title
	w.Subtitle = in.Subtitle
	w.Status = in.Status

	if c.OnState != nil {
		c.OnState(w)
	}
	c.notify()

	return nil
}

func (c *Controller) UpdateCursor(object uint32, in desktop.WindowCursorRequest) error {
	w := c.findByWindowID(in.WindowId)
	if w == nil {
		return fmt.Errorf("shell: window %d not found", in.WindowId)
	}

	shape := event.CursorShape(in.Shape)
	w.Cursor = shape
	if c.OnCursor != nil {
		c.OnCursor(w, shape)
	}
	return nil
}

// findByWindowID searches all registered windows for one with the given window ID.
func (c *Controller) findByWindowID(id uint32) *Window {
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, w := range c.wins {
		if w.ID == id {
			return w
		}
	}
	return nil
}
