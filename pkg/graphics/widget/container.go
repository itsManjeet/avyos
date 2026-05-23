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

// Container, SizedBox, and Padding are the three fundamental box-model
// primitives. Together they cover constrained sizing, background fills,
// borders, shadows, and spacing.
//
//	// A card-like box:
//	widget.Container{
//	    Fill: theme.Surface, Radius: 8,
//	    Border: theme.Outline, BorderWidth: 1,
//	    Shadow: theme.Shadow, ShadowBlur: 8, ShadowOffsetY: 2,
//	    Padding: layout.All(16),
//	    Child: content,
//	}
//
//	// Fixed-size gap:
//	widget.SizedBox{Width: 8}
//
//	// Add space around a child:
//	widget.Padding{Insets: layout.All(12), Child: label}
package widget

import (
	"avyos.dev/pkg/graphics/color"
	"avyos.dev/pkg/graphics/geom"
	"avyos.dev/pkg/graphics/layout"
	"avyos.dev/pkg/graphics/paint"
)

// Container is a box that optionally fills a background, draws a border,
// casts a shadow, adds padding, and renders a single Child.
//
// Color fields with A == 0 are treated as "not set" and produce no paint.
// Width/Height of 0 means "shrink-wrap child" (or fill bounded space when
// Child is nil).
type Container struct {
	// Size constraints. 0 means "derive from child or parent constraints".
	Width, Height float64

	// Background fill. A == 0 means no fill.
	Fill   color.Color
	Radius float64

	// Border. A == 0 or BorderWidth == 0 means no border.
	Border      color.Color
	BorderWidth float64

	// Drop shadow. A == 0 or ShadowBlur == 0 means no shadow.
	Shadow        color.Color
	ShadowBlur    float64
	ShadowOffsetY float64

	// Glow ring. A == 0 or GlowSpread == 0 means no glow.
	Glow       color.Color
	GlowSpread float64

	Padding layout.EdgeInsets
	Child   Widget
}

func (ct Container) RenderChildren(c layout.BoxConstraints, pctx *paint.Context, offset geom.Point, cr ChildRenderer) geom.Size {
	ownC := c
	if ct.Width > 0 {
		ownC = ownC.WithTightWidth(ct.Width)
	}
	if ct.Height > 0 {
		ownC = ownC.WithTightHeight(ct.Height)
	}

	innerC := ct.Padding.Deflate(ownC)

	var sz geom.Size
	if ownC.IsTight() {
		sz = ownC.Biggest()
	} else if ct.Child != nil {
		childSz := cr.Measure(ct.Child, innerC, "child")
		sz = ownC.Constrain(geom.Sz(
			childSz.Width+ct.Padding.Horizontal(),
			childSz.Height+ct.Padding.Vertical(),
		))
	} else {
		big := ownC.Biggest()
		if big.Width < layout.Inf && big.Height < layout.Inf {
			sz = big
		} else {
			sz = ownC.Smallest()
		}
	}

	if pctx != nil {
		r := geom.NewRect(offset.X, offset.Y, sz.Width, sz.Height)
		if ct.Shadow.A > 0 && ct.ShadowBlur > 0 {
			drawSoftShadow(pctx, r, ct.Radius, ct.Shadow, ct.ShadowBlur, ct.ShadowOffsetY)
		}
		if ct.Glow.A > 0 && ct.GlowSpread > 0 {
			drawGlow(pctx, r, ct.Radius, ct.Glow, ct.GlowSpread)
		}
		if ct.Fill.A > 0 {
			if ct.Radius > 0 {
				pctx.FillRoundedRect(r, ct.Radius, ct.Fill)
			} else {
				pctx.FillRect(r, ct.Fill)
			}
		}
		if ct.Border.A > 0 && ct.BorderWidth > 0 {
			inset := ct.BorderWidth / 2
			if ct.Radius > 0 {
				pctx.StrokeRoundedRect(r.Inset(inset, inset), insetCornerRadius(ct.Radius, inset), ct.BorderWidth, ct.Border)
			} else {
				pctx.StrokeRect(r.Inset(inset, inset), ct.BorderWidth, ct.Border)
			}
		}
	}

	if ct.Child != nil {
		tightInner := ct.Padding.Deflate(layout.Tight(sz.Width, sz.Height))
		if pctx != nil {
			pctx.Save()
			pctx.ClipRect(geom.NewRect(offset.X, offset.Y, sz.Width, sz.Height))
		}
		cr.Render(ct.Child, tightInner, offset.Add(ct.Padding.Offset()), "child")
		if pctx != nil {
			pctx.Restore()
		}
	}
	return sz
}

// SizedBox forces a specific width and/or height constraint.
// A dimension of 0 passes the parent constraint through unchanged.
// A SizedBox with no Child and both dimensions set is a fixed-size gap.
type SizedBox struct {
	Width, Height float64
	Child         Widget
}

func (sb SizedBox) RenderChildren(c layout.BoxConstraints, _ *paint.Context, offset geom.Point, cr ChildRenderer) geom.Size {
	childC := c
	if sb.Width > 0 {
		childC = childC.WithTightWidth(sb.Width)
	}
	if sb.Height > 0 {
		childC = childC.WithTightHeight(sb.Height)
	}
	var childSz geom.Size
	if sb.Child != nil {
		childSz = cr.Render(sb.Child, childC, offset, "child")
	}
	w := childSz.Width
	if sb.Width > 0 {
		w = sb.Width
	}
	h := childSz.Height
	if sb.Height > 0 {
		h = sb.Height
	}
	return c.Constrain(geom.Sz(w, h))
}

// Padding inserts EdgeInsets around its Child without any visual decoration.
// For padding with a background, use [Container] instead.
type Padding struct {
	Insets layout.EdgeInsets
	Child  Widget
}

func (p Padding) RenderChildren(c layout.BoxConstraints, _ *paint.Context, offset geom.Point, cr ChildRenderer) geom.Size {
	if p.Child == nil {
		return c.Constrain(geom.Sz(p.Insets.Horizontal(), p.Insets.Vertical()))
	}
	innerC := p.Insets.Deflate(c)
	childSz := cr.Render(p.Child, innerC, offset.Add(p.Insets.Offset()), "child")
	return geom.Sz(childSz.Width+p.Insets.Horizontal(), childSz.Height+p.Insets.Vertical())
}
