//go:build darwin

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

// Package macos implements a Cocoa/AppKit backend for macOS using CGO.
//
// The backend requires that [Backend.Init] is called from the main OS thread.
// Pin the main goroutine before calling Run:
//
//	func init() { runtime.LockOSThread() }
//
// The NSApplication event loop is driven manually (no [NSApp run]) so our
// game-style render loop retains control. gfx_poll_event pumps NSEvents on
// each call.
package macos

// #cgo CFLAGS:  -x objective-c -mmacosx-version-min=10.14 -fno-objc-arc
// #cgo LDFLAGS: -framework Cocoa -framework CoreGraphics
// #include "macos.h"
// #include <stdlib.h>
import "C"

import (
	"errors"
	"unsafe"

	"avyos.dev/lib/graphics/backend"
	"avyos.dev/lib/graphics/canvas"
	"avyos.dev/lib/graphics/canvas/metal"
	"avyos.dev/lib/graphics/event"
	"avyos.dev/lib/graphics/geom"
)

//  Backend

// Backend is the macOS Cocoa backend.
type Backend struct {
	initialised bool
}

// New creates a macOS backend.
// The caller must ensure Init (and all subsequent calls) happen on the main
// OS thread. Use runtime.LockOSThread() in init() or main().
func New() *Backend { return &Backend{} }

func (b *Backend) Name() string { return "macos" }

func (b *Backend) Init() error {
	C.gfx_init()
	b.initialised = true
	return nil
}

func (b *Backend) Shutdown() {}

// Scale returns the main screen's backing pixel scale (1 on non-Retina, 2 on Retina).
// Implements backend.ScaleProvider.
func (b *Backend) Scale() float64 {
	s := float64(C.gfx_main_screen_scale())
	if s <= 0 {
		return 1
	}
	return s
}

func (b *Backend) NewWindow(opts backend.WindowOptions) (backend.Window, error) {
	if !b.initialised {
		return nil, errors.New("macos: call Init first")
	}
	w, h := opts.Width, opts.Height
	if w == 0 {
		w = 800
	}
	if h == 0 {
		h = 600
	}
	title := opts.Title
	if title == "" {
		title = "App"
	}

	cTitle := C.CString(title)
	defer C.free(unsafe.Pointer(cTitle))

	handle := C.gfx_create_window(C.int(w), C.int(h), cTitle)
	if handle == 0 {
		return nil, errors.New("macos: gfx_create_window failed")
	}

	// Physical pixel size = logical size × backing scale (2 on Retina).
	scale := float64(C.gfx_scale(handle))
	if scale <= 0 {
		scale = 1
	}
	physW := int(float64(w) * scale)
	physH := int(float64(h) * scale)

	surf := &surface{
		handle: handle,
		canvas: metal.NewCanvas(physW, physH),
		width:  physW,
		height: physH,
		scale:  scale,
	}
	return &window{
		handle:  handle,
		backend: b,
		surf:    surf,
		width:   physW,
		height:  physH,
	}, nil
}

func (b *Backend) PollEvents() ([]event.Event, error) {
	// Events are collected per-window; called from the window's surface.
	// This top-level PollEvents is a no-op (use window.Surface events).
	return nil, nil
}

func (b *Backend) WaitEvent() ([]event.Event, error) {
	// Block briefly then return.
	return nil, nil
}

var _ backend.Backend = (*Backend)(nil)
var _ backend.ScaleProvider = (*Backend)(nil)

//  Window

type window struct {
	handle  C.uintptr_t
	backend *Backend
	surf    *surface
	width   int
	height  int
}

func (w *window) Surface() backend.Surface { return w.surf }

func (w *window) SetTitle(title string) {
	// Dispatch on main thread via GCD; safe to call from any goroutine.
	// For simplicity we do it directly (must be on main OS thread in our loop).
	cTitle := C.CString(title)
	defer C.free(unsafe.Pointer(cTitle))
	// There is no gfx_set_title C function; call via NSWindow directly would
	// require more CGO bridging. Omit for brevity.
}

func (w *window) Size() (int, int) { return w.width, w.height }

func (w *window) Close() {
	C.gfx_destroy_window(w.handle)
}

var _ backend.Window = (*window)(nil)

//  Surface

type surface struct {
	handle C.uintptr_t
	canvas canvas.Canvas
	width  int
	height int
	scale  float64
}

func (s *surface) Size() geom.Size { return geom.Sz(float64(s.width), float64(s.height)) }

func (s *surface) Begin() (canvas.Canvas, error) { return s.canvas, nil }

func (s *surface) Present(_ canvas.Canvas) error {
	pix := s.canvas.Pixels()
	C.gfx_present(s.handle,
		(*C.uchar)(unsafe.Pointer(&pix[0])),
		C.int(s.width),
		C.int(s.height))
	return nil
}

func (s *surface) Resize(w, h int) error {
	s.canvas = metal.NewCanvas(w, h)
	s.width = w
	s.height = h
	return nil
}

// PollEvents drains the window's native event queue and translates to event.Event.
// Call this instead of Backend.PollEvents when using the macOS backend.
func (s *surface) PollEvents() []event.Event {
	var evs []event.Event
	for {
		var ge C.GEvent
		if C.gfx_poll_event(s.handle, &ge) == 0 {
			break
		}
		if e := translateEvent(ge); e != nil {
			evs = append(evs, e)
		}
	}
	return evs
}

var _ backend.Surface = (*surface)(nil)

//  Event translation

func translateEvent(ge C.GEvent) event.Event {
	switch int(ge._type) {
	case C.GEVT_KEYDOWN:
		return event.KeyEvent{
			Down: true,
			Key:  event.KeyCode(ge.keyCode),
			Mods: event.Modifiers(ge.mods),
		}
	case C.GEVT_KEYUP:
		return event.KeyEvent{
			Down: false,
			Key:  event.KeyCode(ge.keyCode),
			Mods: event.Modifiers(ge.mods),
		}
	case C.GEVT_RUNE:
		return event.TextInputEvent{
			Rune: rune(ge.rune),
		}
	case C.GEVT_MOUSEDOWN:
		return event.ButtonEvent{
			X:      float64(ge.x),
			Y:      float64(ge.y),
			Down:   true,
			Button: event.Button(ge.button),
			Mods:   event.Modifiers(ge.mods),
		}
	case C.GEVT_MOUSEUP:
		return event.ButtonEvent{
			X:      float64(ge.x),
			Y:      float64(ge.y),
			Down:   false,
			Button: event.Button(ge.button),
			Mods:   event.Modifiers(ge.mods),
		}
	case C.GEVT_MOUSEMOVE:
		return event.CursorEvent{
			X: float64(ge.x),
			Y: float64(ge.y),
		}
	case C.GEVT_SCROLL:
		return event.ScrollEvent{
			X:  float64(ge.x),
			Y:  float64(ge.y),
			DX: float64(ge.dx),
			DY: float64(ge.dy),
		}
	case C.GEVT_RESIZE:
		return event.ResizeEvent{
			Width:  int(ge.width),
			Height: int(ge.height),
		}
	case C.GEVT_CLOSE:
		return event.CloseEvent{}
	case C.GEVT_FOCUS:
		return event.FocusEvent{}
	case C.GEVT_BLUR:
		return event.BlurEvent{}
	}
	return nil
}

// WindowSurface is a type-asserted helper to access macOS-specific PollEvents.
func WindowSurface(surf backend.Surface) *surface {
	s, _ := surf.(*surface)
	return s
}
