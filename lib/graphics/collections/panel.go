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

package collections

import (
	"sync"

	"avyos.dev/lib/graphics/widget"
)

// PanelController manages a single exclusive overlay panel — at most one panel
// is visible at a time. Showing a new panel automatically closes the current
// one. Typical use-cases: launcher, quick-settings popover, notification
// centre, command palette.
//
// # Usage
//
//	// In state init:
//	s.panels = collections.NewPanelController(s.overlay)
//	s.panels.SetNotify(func() { s.SetState(nil) })
//
//	// Toggle a panel open/closed:
//	s.panels.Toggle("launcher", func() widget.Widget {
//	    return widget.Positioned{
//	        Left: widget.Ptr(0), Bottom: widget.Ptr(48),
//	        Child: LauncherPanel{OnClose: s.panels.Close},
//	    }
//	})
//
//	// Query open state (e.g. for button highlight):
//	isOpen := s.panels.IsOpen("launcher")
type PanelController struct {
	overlay *OverlayManager

	mu      sync.Mutex
	current string // key of the open panel; "" = none
	dismiss func() // removes current entry from overlay
	notify  func() // called when open/closed state changes
}

// NewPanelController creates a PanelController backed by oc.
func NewPanelController(oc *OverlayManager) *PanelController {
	return &PanelController{overlay: oc}
}

// SetNotify registers fn to be called whenever a panel is opened or closed.
// Pass s.SetState(nil) here so the parent widget rebuilds and can update
// button highlight state via IsOpen.
func (pc *PanelController) SetNotify(fn func()) {
	pc.mu.Lock()
	pc.notify = fn
	pc.mu.Unlock()
}

// Toggle opens the panel identified by key, or closes it if it is already
// open. Any other currently open panel is closed first. build is called each
// frame by the OverlayHost to produce the panel widget.
func (pc *PanelController) Toggle(key string, build func() widget.Widget) {
	pc.mu.Lock()

	if pc.current == key {
		// Panel is already open — close it (toggle off).
		dismiss := pc.dismiss
		pc.current = ""
		pc.dismiss = nil
		notify := pc.notify
		pc.mu.Unlock()
		if dismiss != nil {
			dismiss()
		}
		if notify != nil {
			notify()
		}
		return
	}

	// Close any panel that is currently open.
	dismiss := pc.dismiss
	pc.mu.Unlock()
	if dismiss != nil {
		dismiss()
	}

	// Open the new panel.
	e := &OverlayEntry{
		Builder: func(_ widget.BuildContext) widget.Widget { return build() },
	}
	pc.overlay.Insert(e)

	pc.mu.Lock()
	pc.current = key
	pc.dismiss = e.Remove
	notify := pc.notify
	pc.mu.Unlock()

	if notify != nil {
		notify()
	}
}

// Close closes whatever panel is currently open, if any.
func (pc *PanelController) Close() {
	pc.mu.Lock()
	dismiss := pc.dismiss
	pc.current = ""
	pc.dismiss = nil
	notify := pc.notify
	pc.mu.Unlock()

	if dismiss != nil {
		dismiss()
	}
	if notify != nil {
		notify()
	}
}

// IsOpen reports whether the panel with the given key is currently visible.
func (pc *PanelController) IsOpen(key string) bool {
	pc.mu.Lock()
	defer pc.mu.Unlock()
	return pc.current == key
}
