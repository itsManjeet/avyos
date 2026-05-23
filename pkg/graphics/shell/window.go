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

package shell

import (
	"image"

	"avyos.dev/api/desktop"
	"avyos.dev/pkg/graphics/canvas"
	"avyos.dev/pkg/graphics/color"
	"avyos.dev/pkg/graphics/event"
	"avyos.dev/pkg/graphics/geom"
	"avyos.dev/pkg/graphics/layout"
	"avyos.dev/pkg/graphics/paint"
	"avyos.dev/pkg/graphics/widget"
)

// Window holds the desktop-side model for one remote application window.
// Fields are updated by Controller handlers; reads from the UI thread are safe
// because SetState serialises mutations onto the main goroutine.
type Window struct {
	ID        uint32
	ClientID  uint32
	AppID     string
	AppName   string
	Icon      string
	Accent    color.Color
	Title     string
	Subtitle  string
	Status    string
	Cursor    event.CursorShape
	Width     int
	Height    int
	MinWidth  int
	MinHeight int
	// Scale is in milli-units: 1000 = 1x, 2000 = 2x HiDPI.
	Scale int

	// Damage holds the physical-pixel dirty regions reported by the client in
	// the most recent PresentWindow call. Empty means the full buffer changed.
	// Written by Controller.handlePresent on the sutra goroutine.
	// Callers must snapshot this slice before enqueueing closures that run
	// on the main goroutine to avoid a concurrent-write data race.
	Damage []image.Rectangle

	// unexported
	surface        *surface
	ctrl           *Controller
	lastBufferPath string // last mmap path registered; handlePresent skips re-mmap when unchanged
}

// Build satisfies widget.Buildable. Returns a windowContent that will lock
// the surface only during Paint, preventing the client from reusing the buffer
// while DrawImage is reading it.
func (w *Window) Build(ctx widget.BuildContext) widget.Widget {
	if w.surface.image() == nil {
		return widget.Container{Fill: ctx.Theme.ColorScheme.Surface}
	}
	return windowContent{surface: w.surface, width: w.Width, height: w.Height}
}

// CloseBuffer releases the shared-memory mappings for this window's surface.
// Must be called from the main goroutine after the window has been removed
// from the compositor's render list (so no in-flight Paint can touch the
// memory). Do NOT call this from a sutra goroutine.
func (w *Window) CloseBuffer() {
	w.surface.Close()
}

// SendInput forwards an encoded input event payload to the remote window.
func (w *Window) SendInput(payload []byte) error {
	conn := w.ctrl.ConnFor(w.ClientID)
	if conn == nil {
		return nil
	}
	// Async desktop events are sent on the client's singleton Desktop object.
	return desktop.SendDesktopInput(conn, 0, desktop.WindowInputEvent{WindowId: w.ID, Payload: payload})
}

// SendResize notifies the remote window that it should render at a new size.
func (w *Window) SendResize(width, height uint32) error {
	conn := w.ctrl.ConnFor(w.ClientID)
	if conn == nil {
		return nil
	}
	return desktop.SendDesktopResize(conn, 0, desktop.WindowResizeEvent{WindowId: w.ID, Width: width, Height: height})
}

// SendCloseRequested asks the remote window to close gracefully.
func (w *Window) SendCloseRequested() error {
	conn := w.ctrl.ConnFor(w.ClientID)
	if conn == nil {
		return nil
	}
	return desktop.SendDesktopCloseRequested(conn, 0, desktop.WindowRequest{WindowId: w.ID})
}

// ─── windowContent ────────────────────────────────────────────────────────────

// windowContent is an unexported RenderBox that blits the window's shared-memory
// image into the desktop canvas. It holds a reference to the surface so it can
// lock the buffer for the duration of DrawImage, preventing a concurrent write
// from the client (which is only unblocked after setBuffer returns the write lock).
type windowContent struct {
	surface *surface
	width   int
	height  int
}

func (wc windowContent) Layout(c layout.BoxConstraints) geom.Size {
	return c.Constrain(geom.Sz(float64(wc.width), float64(wc.height)))
}

func (wc windowContent) Paint(ctx *paint.Context, offset geom.Point, size geom.Size) {
	img := wc.surface.lockImage()
	if img == nil {
		wc.surface.unlockImage()
		return
	}
	dst := geom.NewRect(offset.X, offset.Y, size.Width, size.Height)
	if drawer, ok := ctx.Canvas.(canvas.OpaqueImageDrawer); ok {
		drawer.DrawOpaqueImage(img, dst)
	} else {
		ctx.Canvas.DrawImage(img, dst)
	}
	wc.surface.unlockImage()
}

func (wc windowContent) HitTest(pos, offset geom.Point, size geom.Size) bool {
	return geom.NewRect(offset.X, offset.Y, size.Width, size.Height).Contains(pos)
}

// ─── helpers ─────────────────────────────────────────────────────────────────

// colorFromUint32 converts a packed 0xAARRGGBB uint32 to a color.Color.
func colorFromUint32(v uint32) color.Color {
	return color.FromHex(v)
}
