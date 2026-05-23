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

// Package backend defines the platform abstraction layer.
//
// A Backend represents a display system (DRM/KMS, Wayland, X11, headless, …).
// It provides Windows which expose Surfaces for rendering.
//
// Dependency graph:  backend → canvas, event   (no renderer, no scene)
package backend

import (
	"time"

	"avyos.dev/pkg/graphics/canvas"
	"avyos.dev/pkg/graphics/event"
	"avyos.dev/pkg/graphics/geom"
)

// Backend is the top-level platform interface.
type Backend interface {
	// Name returns a human-readable identifier (e.g. "wayland", "drmkms").
	Name() string
	// Init initialises the backend (opens devices, connects to compositor, etc.).
	Init() error
	// Shutdown releases all resources.
	Shutdown()
	// NewWindow creates a window with the given options.
	NewWindow(opts WindowOptions) (Window, error)
	// PollEvents returns all pending events without blocking.
	PollEvents() ([]event.Event, error)
	// WaitEvent blocks until at least one event arrives.
	WaitEvent() ([]event.Event, error)
}

// LayerSurfaceLayer selects the stacking class for a layer-shell surface.
type LayerSurfaceLayer uint32

const (
	LayerBackground LayerSurfaceLayer = iota
	LayerBottom
	LayerTop
	LayerOverlay
)

// LayerSurfaceAnchor is a bitmask that pins a layer surface to screen edges.
type LayerSurfaceAnchor uint32

const (
	LayerAnchorTop LayerSurfaceAnchor = 1 << iota
	LayerAnchorBottom
	LayerAnchorLeft
	LayerAnchorRight
)

// LayerSurfaceOptions requests a layer-shell surface instead of an xdg_toplevel.
type LayerSurfaceOptions struct {
	Namespace             string
	Layer                 LayerSurfaceLayer
	Anchor                LayerSurfaceAnchor
	ExclusiveZone         int32
	MarginTop             int32
	MarginRight           int32
	MarginBottom          int32
	MarginLeft            int32
	KeyboardInteractivity uint32
}

// WindowOptions controls window creation.
type WindowOptions struct {
	Title  string
	Width  int
	Height int
	// Scale is the logical-to-physical render scale for the window.
	// Values <= 0 request the backend default.
	Scale      float64
	Fullscreen bool
	Resizable  bool
	Layer      *LayerSurfaceOptions
}

// Window represents an on-screen window or display output.
type Window interface {
	Surface() Surface
	SetTitle(title string)
	Size() (width, height int)
	Close()
}

// EventPoller is optionally implemented by a Surface that manages its own
// per-window event queue (e.g. the macOS backend).  The app loop checks for
// this interface first; if absent it falls back to Backend.PollEvents.
type EventPoller interface {
	PollEvents() []event.Event
}

// Surface is the per-frame rendering target exposed by a Window.
//
// Usage:
//
//	c, err := surface.Begin()
//	// draw onto c …
//	err = surface.Present(c)
type Surface interface {
	// Begin acquires a frame buffer and returns a Canvas for drawing.
	Begin() (canvas.Canvas, error)
	// Present submits the rendered frame to the display.
	Present(c canvas.Canvas) error
	// Resize notifies the surface that the window has been resized.
	Resize(width, height int) error
	// Size returns the current surface dimensions.
	Size() geom.Size
}

// CursorSetter is an optional Surface extension for updating the visible cursor
// shape based on the currently hovered widget.
type CursorSetter interface {
	SetCursorShape(shape event.CursorShape) error
}

// PresentStats captures an optional backend-side breakdown of Surface.Present.
// Surface implementations may expose this for tracing or diagnostics.
type PresentStats struct {
	Blit   time.Duration
	Submit time.Duration
	Wait   time.Duration
}

// PresentStatsProvider is an optional Surface extension for collecting
// backend-side present timings. Callers must type-assert.
type PresentStatsProvider interface {
	PresentStats() PresentStats
}

// ToplevelInfo describes a foreign toplevel exposed by the compositor.
type ToplevelInfo struct {
	ID         uint32
	Title      string
	AppID      string
	Activated  bool
	Maximized  bool
	Minimized  bool
	Fullscreen bool
}

// ToplevelEventType describes the lifecycle of a toplevel update.
type ToplevelEventType uint8

const (
	ToplevelAdded ToplevelEventType = iota
	ToplevelUpdated
	ToplevelRemoved
)

// ToplevelEvent reports a compositor-side toplevel change.
type ToplevelEvent struct {
	Type ToplevelEventType
	Info ToplevelInfo
}

// ToplevelManager is an optional backend extension for compositor window lists.
type ToplevelManager interface {
	Toplevels() []ToplevelInfo
	PollToplevelEvents() []ToplevelEvent
	ActivateToplevel(id uint32) error
	CloseToplevel(id uint32) error
}

// ScaleProvider is an optional backend extension that reports the preferred
// render scale for client windows on that backend.
type ScaleProvider interface {
	Scale() float64
}
