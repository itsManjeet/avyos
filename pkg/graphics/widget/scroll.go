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

// Scroll is a stateful scroll view that wraps ScrollArea with drag gestures
// and optional scroll bars.
//
// ScrollBar is a standalone draggable scrollbar widget used by Scroll but also
// usable independently.
//
//	widget.Scroll{Child: longList}
//	widget.Scroll{Axis: layout.Horizontal, Child: wideContent}
package widget

import (
	"math"

	"avyos.dev/pkg/graphics/geom"
	"avyos.dev/pkg/graphics/layout"
)

// ─── Scroll ───────────────────────────────────────────────────────────────────

// Scroll is a [StatefulWidget] that provides a scrollable viewport for Child.
// The scroll position is managed internally; use OnScroll to observe changes.
//
// Width and Height constrain the viewport; 0 means "fill available space".
// Set Both to true to enable simultaneous horizontal and vertical scrolling.
type Scroll struct {
	Axis     layout.Axis
	Width    float64
	Height   float64
	Child    Widget
	OnScroll func(offset geom.Point)
	Both     bool
}

func (s Scroll) CreateState() State { return &scrollState{} }

type scrollState struct {
	StateBase
	widget      Scroll
	offset      geom.Point
	dragging    bool
	lastPos     geom.Point
	contentSize geom.Size
	viewSize    geom.Size
}

func (s *scrollState) UpdateWidget(w Widget) {
	if v, ok := w.(Scroll); ok {
		s.widget = v
	}
}

func (s *scrollState) Build(ctx BuildContext) Widget {
	w := s.widget
	if w.Child == nil {
		return SizedBox{}
	}

	axis := w.Axis
	if axis != layout.Horizontal && axis != layout.Vertical {
		axis = layout.Vertical
	}

	scrollArea := ScrollArea{
		Width:      w.Width,
		Height:     w.Height,
		Offset:     s.offset,
		Axis:       axis,
		Both:       w.Both,
		Child:      w.Child,
		OnMeasure:  s.onContentSize,
		OnViewport: s.onViewSize,
	}

	gesture := GestureDetector{
		OnPointerDown: func(pos geom.Point) {
			s.SetState(func() {
				s.dragging = true
				s.lastPos = pos
			})
		},
		OnDragMove: func(pos geom.Point) {
			if !s.dragging {
				return
			}
			if w.Both {
				delta := s.lastPos.Sub(pos)
				s.SetState(func() { s.lastPos = pos })
				s.applyOffset(layout.Horizontal, s.offset.X+delta.X)
				s.applyOffset(layout.Vertical, s.offset.Y+delta.Y)
				return
			}
			delta := axisCoord(s.lastPos, axis) - axisCoord(pos, axis)
			if delta == 0 {
				s.lastPos = pos
				return
			}
			s.SetState(func() { s.lastPos = pos })
			s.applyOffset(axis, axisValue(s.offset, axis)+delta)
		},
		OnDragEnd: func() {
			s.SetState(func() { s.dragging = false })
		},
		OnScroll: func(dx, dy float64) {
			if w.Both {
				s.applyOffset(layout.Horizontal, s.offset.X+dx)
				s.applyOffset(layout.Vertical, s.offset.Y+dy)
				return
			}
			if axis == layout.Horizontal {
				s.applyOffset(axis, s.offset.X+dx)
			} else {
				s.applyOffset(axis, s.offset.Y+dy)
			}
		},
		Child: scrollArea,
	}

	// Build scroll bars.
	var bars [2]Widget
	barCount := 0
	appendBar := func(bar Widget) {
		if bar == nil || barCount >= len(bars) {
			return
		}
		bars[barCount] = bar
		barCount++
	}

	buildBar := func(ax layout.Axis) {
		view := axisViewSize(s.viewSize, ax)
		content := axisViewSize(s.contentSize, ax)
		offset := axisValue(s.offset, ax)
		if view <= 0 || content <= view {
			return
		}
		if ax == layout.Vertical {
			appendBar(Positioned{
				Right:  Ptr(4),
				Top:    Ptr(0),
				Bottom: Ptr(0),
				Child: ScrollBar{
					Axis:        ax,
					Offset:      offset,
					ContentSize: content,
					Viewport:    view,
					OnThumbDrag: func(v float64) { s.applyOffset(ax, v) },
				},
			})
		} else {
			appendBar(Positioned{
				Left:   Ptr(0),
				Right:  Ptr(0),
				Bottom: Ptr(4),
				Height: Ptr(7),
				Child: ScrollBar{
					Axis:        ax,
					Offset:      offset,
					ContentSize: content,
					Viewport:    view,
					OnThumbDrag: func(v float64) { s.applyOffset(ax, v) },
				},
			})
		}
	}

	buildBar(axis)
	if w.Both {
		if axis == layout.Vertical {
			buildBar(layout.Horizontal)
		} else {
			buildBar(layout.Vertical)
		}
	}

	if barCount == 0 {
		return gesture
	}

	var children [3]Widget
	children[0] = gesture
	for i := 0; i < barCount; i++ {
		children[i+1] = bars[i]
	}
	return Stack{Children: children[:1+barCount]}
}

func (s *scrollState) onContentSize(sz geom.Size) {
	if s.contentSize == sz {
		return
	}
	s.SetState(func() { s.contentSize = sz })
}

func (s *scrollState) onViewSize(sz geom.Size) {
	if s.viewSize == sz {
		return
	}
	s.SetState(func() { s.viewSize = sz })
}

func (s *scrollState) applyOffset(axis layout.Axis, value float64) {
	limit := math.Max(0, axisViewSize(s.contentSize, axis)-axisViewSize(s.viewSize, axis))
	next := clampFloat64(value, 0, limit)
	if next == axisValue(s.offset, axis) {
		return
	}
	s.SetState(func() {
		if axis == layout.Horizontal {
			s.offset.X = next
		} else {
			s.offset.Y = next
		}
	})
	if s.widget.OnScroll != nil {
		s.widget.OnScroll(s.offset)
	}
}

// ─── ScrollBar ────────────────────────────────────────────────────────────────

// ScrollBar is a draggable scrollbar track and thumb.
// Offset is the current scroll position; ContentSize is the total content size;
// Viewport is the visible extent. OnThumbDrag is called with a new offset.
type ScrollBar struct {
	Axis        layout.Axis
	Offset      float64
	ContentSize float64
	Viewport    float64
	Thickness   float64 // 0 defaults to 7
	MinThumb    float64 // minimum thumb length; 0 defaults to 24
	OnThumbDrag func(float64)
}

func (ScrollBar) CreateState() State { return &scrollBarState{} }

type scrollBarState struct {
	StateBase
	widget   ScrollBar
	dragging bool
	lastPos  float64
}

func (s *scrollBarState) UpdateWidget(w Widget) {
	if v, ok := w.(ScrollBar); ok {
		s.widget = v
	}
}

func (s *scrollBarState) Build(ctx BuildContext) Widget {
	w := s.widget
	view := math.Max(0, w.Viewport)
	content := math.Max(0, w.ContentSize)
	if view <= 0 || content <= view {
		return SizedBox{}
	}

	thickness := w.Thickness
	if thickness <= 0 {
		thickness = 7
	}

	trackLen := view
	thumbLen := w.thumbLen(trackLen, view, content)
	if thumbLen > trackLen {
		thumbLen = trackLen
	}
	trackRange := trackLen - thumbLen
	thumbPos := 0.0
	if scrollRange := content - view; scrollRange > 0 && trackRange > 0 {
		thumbPos = clampFloat64((w.Offset/scrollRange)*trackRange, 0, trackRange)
	}

	trackColor := ctx.Theme.ColorScheme.SurfaceVariant.WithAlpha(0.48)
	thumbColor := ctx.Theme.ColorScheme.OnSurface.WithAlpha(0.85)

	var sized Widget
	if w.Axis == layout.Vertical {
		sized = SizedBox{
			Width:  thickness,
			Height: trackLen,
			Child: Stack{Children: []Widget{
				Container{Fill: trackColor},
				Positioned{
					Top:    Ptr(thumbPos),
					Left:   Ptr(0),
					Right:  Ptr(0),
					Height: Ptr(thumbLen),
					Child:  Container{Fill: thumbColor, Radius: 999},
				},
			}},
		}
	} else {
		sized = SizedBox{
			Width:  trackLen,
			Height: thickness,
			Child: Stack{Children: []Widget{
				Container{Fill: trackColor},
				Positioned{
					Left:   Ptr(thumbPos),
					Top:    Ptr(0),
					Bottom: Ptr(0),
					Width:  Ptr(thumbLen),
					Child:  Container{Fill: thumbColor, Radius: 999},
				},
			}},
		}
	}

	axis := w.Axis
	return GestureDetector{
		OnPointerDown: func(pos geom.Point) {
			s.dragging = true
			s.lastPos = axisCoord(pos, axis)
		},
		OnDragMove: func(pos geom.Point) {
			if !s.dragging || w.OnThumbDrag == nil {
				return
			}
			coord := axisCoord(pos, axis)
			delta := coord - s.lastPos
			if delta == 0 {
				return
			}
			s.lastPos = coord
			scrollRange := math.Max(0, content-view)
			if scrollRange <= 0 || trackRange <= 0 {
				return
			}
			next := clampFloat64(w.Offset+(delta/trackRange)*scrollRange, 0, scrollRange)
			w.OnThumbDrag(next)
		},
		OnDragEnd: func() { s.dragging = false },
		Child:     sized,
	}
}

func (sb ScrollBar) thumbLen(trackLen, view, content float64) float64 {
	ratio := 0.0
	if content > 0 {
		ratio = math.Min(1, view/content)
	}
	length := ratio * trackLen
	minLen := sb.MinThumb
	if minLen <= 0 {
		minLen = 24
	}
	return math.Max(length, minLen)
}

// ─── axis helpers ─────────────────────────────────────────────────────────────

func axisCoord(pos geom.Point, axis layout.Axis) float64 {
	if axis == layout.Horizontal {
		return pos.X
	}
	return pos.Y
}

func axisValue(p geom.Point, axis layout.Axis) float64 {
	if axis == layout.Horizontal {
		return p.X
	}
	return p.Y
}

func axisViewSize(sz geom.Size, axis layout.Axis) float64 {
	if axis == layout.Horizontal {
		return sz.Width
	}
	return sz.Height
}
