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

// Slider is a draggable range input.
//
//	widget.Slider{Min: 0, Max: 100, Value: vol, OnChanged: func(v float64) { vol = v }}
package widget

import (
	"math"

	"avyos.dev/pkg/graphics/color"
	"avyos.dev/pkg/graphics/geom"
	"avyos.dev/pkg/graphics/layout"
	"avyos.dev/pkg/graphics/paint"
)

// Slider is a horizontal range control.
// Dragging the thumb calls OnChanged with the new value in [Min, Max].
// If Max <= Min, the range defaults to [Min, Min+1].
type Slider struct {
	Value     float64
	Min       float64
	Max       float64
	OnChanged func(float64)
}

func (Slider) CreateState() State { return &sliderState{} }

type sliderState struct {
	StateBase
	widget   Slider
	width    float64
	dragging bool
}

func (s *sliderState) UpdateWidget(w Widget) {
	if v, ok := w.(Slider); ok {
		s.widget = v
	}
}

const (
	sliderDefaultWidth = 220.0
	sliderHeight       = 22.0
	sliderThumbRadius  = 8.0
	sliderThumbHitPad  = 4.0
)

func (s *sliderState) Build(ctx BuildContext) Widget {
	w := s.widget
	minV := w.Min
	maxV := w.Max
	if maxV <= minV {
		maxV = minV + 1
	}
	progress := clamp01((w.Value - minV) / (maxV - minV))
	frame := ctx.frame

	update := func(local geom.Point, width float64) {
		if width <= 0 || w.OnChanged == nil {
			return
		}
		w.OnChanged(minV + clamp01(local.X/width)*(maxV-minV))
	}

	thumbHit := func(local geom.Point, width float64) bool {
		if width <= 0 {
			width = sliderDefaultWidth
		}
		thumbX := width * progress
		thumbY := sliderHeight / 2
		radius := sliderThumbRadius + sliderThumbHitPad
		return math.Abs(local.X-thumbX) <= radius && math.Abs(local.Y-thumbY) <= radius
	}

	return GestureDetector{
		OnPointerDownLocal: func(local geom.Point) {
			if !thumbHit(local, s.width) {
				return
			}
			s.dragging = true
			if frame != nil {
				frame.MarkDirty()
			}
		},
		OnPointerMoveLocal: func(local geom.Point) {
			if !s.dragging {
				return
			}
			update(local, s.width)
		},
		OnPointerUpLocal: func(local geom.Point) {
			if !s.dragging {
				return
			}
			update(local, s.width)
			s.dragging = false
			if frame != nil {
				frame.MarkDirty()
			}
		},
		OnDragEnd: func() {
			if !s.dragging {
				return
			}
			s.dragging = false
			if frame != nil {
				frame.MarkDirty()
			}
		},
		Builder: func(state InteractionState) Widget {
			interaction := interactionTarget(state)
			if s.dragging && interaction < 1 {
				interaction = 1
			}
			return sliderLeaf{
				progress:    progress,
				interaction: interaction,
				active:      ctx.Theme.ColorScheme.Primary,
				track:       ctx.Theme.ColorScheme.SurfaceContainer,
				thumb:       ctx.Theme.ColorScheme.Surface,
				outline:     ctx.Theme.ColorScheme.Outline,
				onLayout: func(width float64) {
					s.width = width
				},
			}
		},
	}
}

type sliderLeaf struct {
	progress    float64
	interaction float64
	active      color.Color
	track       color.Color
	thumb       color.Color
	outline     color.Color
	onLayout    func(width float64)
}

func (sl sliderLeaf) Layout(c layout.BoxConstraints) geom.Size {
	w := c.MaxWidth
	if w <= 0 || w >= layout.Inf {
		w = sliderDefaultWidth
	}
	sz := c.Constrain(geom.Sz(w, sliderHeight))
	if sl.onLayout != nil {
		sl.onLayout(sz.Width)
	}
	return sz
}

// Chakra UI Slider: track height=4px, thumb=white circle with border on active.
func (sl sliderLeaf) Paint(ctx *paint.Context, offset geom.Point, size geom.Size) {
	trackRect := geom.NewRect(offset.X, offset.Y+size.Height/2-2, size.Width, 4)
	ctx.FillRoundedRect(trackRect, 999, sl.track)
	ctx.FillRoundedRect(
		geom.NewRect(trackRect.Min.X, trackRect.Min.Y, trackRect.Width()*sl.progress, trackRect.Height()),
		999, sl.active,
	)
	thumbX := offset.X + size.Width*sl.progress
	thumbRect := geom.NewRect(thumbX-sliderThumbRadius, offset.Y+size.Height/2-sliderThumbRadius, sliderThumbRadius*2, sliderThumbRadius*2)
	drawSoftShadow(ctx, thumbRect, sliderThumbRadius, color.Black.WithAlpha(0.12), 6, 1)
	ctx.FillRoundedRect(thumbRect, sliderThumbRadius, sl.thumb)
	ctx.StrokeRoundedRect(thumbRect.Inset(0.5, 0.5), sliderThumbRadius-0.5, 1,
		mixColor(sl.outline, sl.active, sl.interaction*0.5))
}

func (sl sliderLeaf) HitTest(pos, offset geom.Point, size geom.Size) bool {
	return geom.NewRect(offset.X, offset.Y, size.Width, size.Height).Contains(pos)
}
