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
//
// Package noop provides a no-op Backend for testing and headless rendering.
//
// All operations succeed; nothing is displayed. The CPU canvas backs the
// surface so rendering code is fully exercised without a display.
package noop

import (
	"avyos.dev/lib/graphics/backend"
	"avyos.dev/lib/graphics/canvas"
	"avyos.dev/lib/graphics/canvas/pixbuf"
	"avyos.dev/lib/graphics/event"
	"avyos.dev/lib/graphics/geom"
)

// Backend is a no-op backend.
type Backend struct {
	events chan event.Event
}

// New creates a no-op backend.
func New() *Backend { return &Backend{events: make(chan event.Event, 64)} }

func (b *Backend) Name() string { return "noop" }
func (b *Backend) Init() error  { return nil }
func (b *Backend) Shutdown()    {}

func (b *Backend) NewWindow(opts backend.WindowOptions) (backend.Window, error) {
	w, h := opts.Width, opts.Height
	if w == 0 {
		w = 800
	}
	if h == 0 {
		h = 600
	}
	return &window{
		title:  opts.Title,
		width:  w,
		height: h,
		surf:   newSurface(w, h),
	}, nil
}

func (b *Backend) PollEvents() ([]event.Event, error) {
	var evs []event.Event
	for {
		select {
		case e := <-b.events:
			evs = append(evs, e)
		default:
			return evs, nil
		}
	}
}

func (b *Backend) WaitEvent() ([]event.Event, error) {
	e := <-b.events
	more, _ := b.PollEvents()
	return append([]event.Event{e}, more...), nil
}

// Push injects a synthetic event (useful for tests).
func (b *Backend) Push(e event.Event) {
	select {
	case b.events <- e:
	default:
	}
}

var _ backend.Backend = (*Backend)(nil)

type window struct {
	title  string
	width  int
	height int
	surf   *surface
}

func (w *window) Surface() backend.Surface { return w.surf }
func (w *window) SetTitle(t string)        { w.title = t }
func (w *window) Size() (int, int)         { return w.width, w.height }
func (w *window) Close()                   {}

var _ backend.Window = (*window)(nil)

type surface struct {
	c canvas.Canvas
}

func newSurface(w, h int) *surface { return &surface{c: pixbuf.NewCanvas(w, h)} }

func (s *surface) Begin() (canvas.Canvas, error) { return s.c, nil }
func (s *surface) Present(_ canvas.Canvas) error { return nil }
func (s *surface) Resize(w, h int) error {
	s.c = pixbuf.NewCanvas(w, h)
	return nil
}
func (s *surface) Size() geom.Size { return s.c.Size() }

var _ backend.Surface = (*surface)(nil)
