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

package main

import (
	"image"

	"avyos.dev/pkg/graphics/event"
	"avyos.dev/pkg/graphics/geom"
	"avyos.dev/pkg/graphics/shell"
)

const (
	titlebarHeight = 32.0
	borderWidth    = 1.0
	shelfHeight    = 48.0
	windowRadius   = 8.0
)

// ManagedWindow tracks desktop-side state for one remote window.
type ManagedWindow struct {
	Win         *shell.Window
	Icon        image.Image
	CursorShape event.CursorShape
	X, Y        float64 // workspace position of the top-left corner of the chrome
	W, H        float64 // content size (excluding chrome: titlebar and borders)

	Minimized bool
	Maximized bool

	// Drag state — persists across rebuilds.
	dragging    bool
	lastDragPos geom.Point

	// Resize drag state.
	resizing          bool
	resizeDragStarted bool
	lastResizePos     geom.Point

	// Saved geometry for restore from maximized state.
	restoreX, restoreY float64
	restoreW, restoreH float64
}

// ChromeRect returns the full on-screen rect including the titlebar and borders.
func (mw *ManagedWindow) ChromeRect() (x, y, w, h float64) {
	x = mw.X
	y = mw.Y
	w = mw.W + borderWidth*2
	h = mw.H + titlebarHeight + borderWidth*2
	return
}

// ContentRect returns the on-screen rect of just the window content area.
func (mw *ManagedWindow) ContentRect() (x, y, w, h float64) {
	x = mw.X + borderWidth
	y = mw.Y + titlebarHeight + borderWidth
	w = mw.W
	h = mw.H
	return
}
