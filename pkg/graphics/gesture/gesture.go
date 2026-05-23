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

// Package gesture provides hit-testing and gesture recognition for the UI framework.
package gesture

import "avyos.dev/pkg/graphics/geom"

// HitTestEntry records one node that was hit during a hit test.
type HitTestEntry struct {
	// Target is the object that was hit.
	Target HitTestTarget
	// LocalPosition is the hit position in the target's local coordinate space.
	LocalPosition geom.Point
}

// HitTestTarget can handle pointer events after being hit.
type HitTestTarget interface {
	HandleEvent(event PointerEvent, entry HitTestEntry)
}

// HitTestResult accumulates entries during a hit test walk.
type HitTestResult struct {
	Path []HitTestEntry
}

// Add records an entry in the hit path.
func (r *HitTestResult) Add(entry HitTestEntry) { r.Path = append(r.Path, entry) }

// PointerEvent represents a pointer (mouse / touch) input event.
type PointerEvent struct {
	Kind     PointerKind
	Position geom.Point
	Button   int
}

// PointerKind identifies the type of pointer event.
type PointerKind int

const (
	PointerDown PointerKind = iota
	PointerMove
	PointerUp
	PointerCancel
)

// TapCallback is called when a tap is recognised.
type TapCallback func()

// TapRecognizer detects tap gestures on a region.
type TapRecognizer struct {
	OnTap   TapCallback
	down    bool
	downPos geom.Point
}

// PointerDown records the initial press.
func (t *TapRecognizer) PointerDown(pos geom.Point) {
	t.down = true
	t.downPos = pos
}

// PointerUp fires OnTap if the pointer came up near the press position.
func (t *TapRecognizer) PointerUp(pos geom.Point) {
	if !t.down {
		return
	}
	t.down = false
	if t.OnTap != nil {
		dx := pos.X - t.downPos.X
		dy := pos.Y - t.downPos.Y
		dist := dx*dx + dy*dy
		if dist < 400 { // within 20px radius
			t.OnTap()
		}
	}
}

// Cancel cancels a pending gesture.
func (t *TapRecognizer) Cancel() { t.down = false }
