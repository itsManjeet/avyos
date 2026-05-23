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

// Checkbox and Switch are boolean toggle controls.
//
// Both animate between their on/off states and respond to hover/press.
//
//	widget.Checkbox{Value: checked, OnChanged: func(v bool) { checked = v }}
//	widget.Switch{Value: enabled, OnChanged: func(v bool) { enabled = v }}
package widget

import (
	"avyos.dev/lib/graphics/color"
	"avyos.dev/lib/graphics/geom"
	"avyos.dev/lib/graphics/layout"
	"avyos.dev/lib/graphics/paint"
)

// ─── Checkbox ────────────────────────────────────────────────────────────────

// checkboxSize is the default checkbox size: 16px x 16px.
const checkboxSize = 16.0

// Checkbox is an 18×18 tappable toggle with an animated check mark.
type Checkbox struct {
	Value     bool
	OnChanged func(bool)
}

func (cb Checkbox) Build(ctx BuildContext) Widget {
	checked := cb.Value
	return GestureDetector{
		OnTap: func() {
			if cb.OnChanged != nil {
				cb.OnChanged(!checked)
			}
		},
		Builder: func(state InteractionState) Widget {
			return Animated{
				Value:    interactionTarget(state),
				Duration: ctx.Theme.Motion.Fast,
				Curve:    EaseOut,
				Builder: func(interaction float64) Widget {
					return Animated{
						Value:    boolToFloat(checked),
						Duration: ctx.Theme.Motion.Moderate,
						Curve:    EaseInOut,
						Builder: func(progress float64) Widget {
							return checkboxLeaf{
								progress:    progress,
								interaction: interaction,
								fill:        mixColor(ctx.Theme.ColorScheme.Surface, ctx.Theme.ColorScheme.Primary, progress),
								border:      mixColor(ctx.Theme.ColorScheme.Outline, ctx.Theme.ColorScheme.Primary, 0.25+0.75*progress),
								check:       ctx.Theme.ColorScheme.OnPrimary,
								glow:        ctx.Theme.ColorScheme.FocusRing.WithAlpha(0.04 + 0.08*(interaction+progress)/2),
								radius:      ctx.Theme.Shape.XXSmallRadius,
							}
						},
					}
				},
			}
		},
	}
}

type checkboxLeaf struct {
	progress    float64
	interaction float64
	fill        color.Color
	border      color.Color
	check       color.Color
	glow        color.Color
	radius      float64
}

func (cl checkboxLeaf) Layout(c layout.BoxConstraints) geom.Size {
	return c.Constrain(geom.Sz(checkboxSize, checkboxSize))
}

func (cl checkboxLeaf) Paint(ctx *paint.Context, offset geom.Point, size geom.Size) {
	rect := geom.NewRect(offset.X, offset.Y, size.Width, size.Height)
	drawGlow(ctx, rect, cl.radius, cl.glow, 2+2*cl.interaction)
	ctx.FillRoundedRect(rect, cl.radius, mixColor(cl.fill, lightenColor(cl.fill, 0.05), cl.interaction*0.25))
	ctx.StrokeRoundedRect(rect.Inset(0.5, 0.5), insetCornerRadius(cl.radius, 0.5), 1, cl.border)

	if cl.progress <= 0 {
		return
	}
	cx, cy := offset.X+size.Width/2, offset.Y+size.Height/2
	s := size.Width * 0.24
	ctx.Canvas.SetStrokeColor(cl.check.WithAlpha(cl.progress))
	ctx.Canvas.SetLineWidth(1.8)
	ctx.Canvas.DrawLine(geom.Pt(cx-s, cy), geom.Pt(cx-s*0.25, cy+s))
	ctx.Canvas.DrawLine(geom.Pt(cx-s*0.25, cy+s), geom.Pt(cx+s, cy-s))
}

func (cl checkboxLeaf) HitTest(pos, offset geom.Point, size geom.Size) bool {
	return geom.NewRect(offset.X, offset.Y, size.Width, size.Height).Contains(pos)
}

// ─── Switch ───────────────────────────────────────────────────────────────────

// Switch dimensions use a 40px track and 16px thumb.
const (
	switchWidth  = 40.0
	switchHeight = 20.0
	thumbRadius  = 8.0
)

// Switch is a tappable toggle rendered as a sliding track-and-thumb.
type Switch struct {
	Value     bool
	OnChanged func(bool)
}

func (sw Switch) Build(ctx BuildContext) Widget {
	on := sw.Value
	return GestureDetector{
		OnTap: func() {
			if sw.OnChanged != nil {
				sw.OnChanged(!on)
			}
		},
		Builder: func(state InteractionState) Widget {
			return Animated{
				Value:    interactionTarget(state),
				Duration: ctx.Theme.Motion.Fast,
				Curve:    EaseOut,
				Builder: func(interaction float64) Widget {
					return Animated{
						Value:    boolToFloat(on),
						Duration: ctx.Theme.Motion.Moderate,
						Curve:    EaseInOut,
						Builder: func(progress float64) Widget {
							return switchLeaf{
								progress:    progress,
								interaction: interaction,
								active:      ctx.Theme.ColorScheme.Primary,
								inactive:    ctx.Theme.ColorScheme.SurfaceContainerHighest,
								outline:     mixColor(ctx.Theme.ColorScheme.Outline, ctx.Theme.ColorScheme.Primary, progress),
								thumb:       mixColor(color.White, ctx.Theme.ColorScheme.OnPrimary, progress),
								glow:        ctx.Theme.ColorScheme.FocusRing.WithAlpha(0.04 + 0.08*(progress+interaction)/2),
							}
						},
					}
				},
			}
		},
	}
}

type switchLeaf struct {
	progress    float64
	interaction float64
	active      color.Color
	inactive    color.Color
	outline     color.Color
	thumb       color.Color
	glow        color.Color
}

func (sl switchLeaf) Layout(c layout.BoxConstraints) geom.Size {
	return c.Constrain(geom.Sz(switchWidth, switchHeight))
}

func (sl switchLeaf) Paint(ctx *paint.Context, offset geom.Point, size geom.Size) {
	trackRect := geom.NewRect(offset.X, offset.Y, size.Width, size.Height)
	trackRadius := size.Height / 2
	trackColor := mixColor(sl.inactive, sl.active, sl.progress)
	drawGlow(ctx, trackRect, trackRadius, sl.glow, 2+2*sl.interaction)
	ctx.FillRoundedRect(trackRect, trackRadius, mixColor(trackColor, lightenColor(trackColor, 0.06), sl.interaction*0.2))
	ctx.StrokeRoundedRect(trackRect.Inset(0.5, 0.5), insetCornerRadius(trackRadius, 0.5), 1, sl.outline)

	// Thumb travels: margin(2px) + thumbRadius to width - thumbRadius - margin(2px)
	cy := offset.Y + size.Height/2
	cx := offset.X + thumbRadius + 2 + (size.Width-(thumbRadius*2)-4)*sl.progress
	thumbRect := geom.NewRect(cx-thumbRadius, cy-thumbRadius, thumbRadius*2, thumbRadius*2)
	drawSoftShadow(ctx, thumbRect, thumbRadius, color.Black.WithAlpha(0.14), 8, 2)
	ctx.Canvas.SetFillColor(sl.thumb)
	ctx.Canvas.FillCircle(geom.Pt(cx, cy), thumbRadius+sl.interaction*0.2)
}

func (sl switchLeaf) HitTest(pos, offset geom.Point, size geom.Size) bool {
	return geom.NewRect(offset.X, offset.Y, size.Width, size.Height).Contains(pos)
}
