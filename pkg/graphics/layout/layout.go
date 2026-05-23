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

// Package layout provides the constraint-based layout primitives for the UI framework.
//
// Layout follows Flutter's box model:
//
//	Parent passes BoxConstraints down → child returns Size up.
//
// Dependency: geom only.
package layout

import (
	"math"

	"avyos.dev/pkg/graphics/geom"
)

// Inf is the "unbounded" constraint value.
const Inf = math.MaxFloat64

// BoxConstraints describes the size space a widget is allowed to occupy.
type BoxConstraints struct {
	MinWidth, MaxWidth   float64
	MinHeight, MaxHeight float64
}

// Tight returns constraints that force exactly w×h.
func Tight(w, h float64) BoxConstraints {
	return BoxConstraints{MinWidth: w, MaxWidth: w, MinHeight: h, MaxHeight: h}
}

// TightSize returns tight constraints matching sz.
func TightSize(sz geom.Size) BoxConstraints { return Tight(sz.Width, sz.Height) }

// Loose returns constraints with no minimum and the given maximum.
func Loose(maxW, maxH float64) BoxConstraints {
	return BoxConstraints{MaxWidth: maxW, MaxHeight: maxH}
}

// Expand returns constraints that force the widget to fill all available space.
func Expand(w, h float64) BoxConstraints { return Tight(w, h) }

// Unconstrained returns fully unconstrained constraints.
func Unconstrained() BoxConstraints { return BoxConstraints{MaxWidth: Inf, MaxHeight: Inf} }

// Constrain clamps sz to satisfy these constraints.
func (c BoxConstraints) Constrain(sz geom.Size) geom.Size {
	return geom.Sz(clamp(sz.Width, c.MinWidth, c.MaxWidth), clamp(sz.Height, c.MinHeight, c.MaxHeight))
}

// ConstrainWidth clamps w to [MinWidth, MaxWidth].
func (c BoxConstraints) ConstrainWidth(w float64) float64 { return clamp(w, c.MinWidth, c.MaxWidth) }

// ConstrainHeight clamps h to [MinHeight, MaxHeight].
func (c BoxConstraints) ConstrainHeight(h float64) float64 {
	return clamp(h, c.MinHeight, c.MaxHeight)
}

// IsTight reports whether the constraints allow exactly one size.
func (c BoxConstraints) IsTight() bool { return c.MinWidth >= c.MaxWidth && c.MinHeight >= c.MaxHeight }

// HasBoundedWidth reports whether MaxWidth is finite.
func (c BoxConstraints) HasBoundedWidth() bool { return c.MaxWidth < Inf }

// HasBoundedHeight reports whether MaxHeight is finite.
func (c BoxConstraints) HasBoundedHeight() bool { return c.MaxHeight < Inf }

// Biggest returns the largest size satisfying c.
func (c BoxConstraints) Biggest() geom.Size {
	w := c.MaxWidth
	if w >= Inf {
		w = c.MinWidth
	}
	h := c.MaxHeight
	if h >= Inf {
		h = c.MinHeight
	}
	return geom.Sz(w, h)
}

// Smallest returns the smallest size satisfying c.
func (c BoxConstraints) Smallest() geom.Size { return geom.Sz(c.MinWidth, c.MinHeight) }

// Loosen removes minimum constraints (child may be smaller than max).
func (c BoxConstraints) Loosen() BoxConstraints {
	return BoxConstraints{MaxWidth: c.MaxWidth, MaxHeight: c.MaxHeight}
}

// DeflateBy shrinks max dimensions by dw/dh, used to apply padding.
func (c BoxConstraints) DeflateBy(dw, dh float64) BoxConstraints {
	return BoxConstraints{
		MinWidth:  math.Max(0, c.MinWidth-dw),
		MaxWidth:  math.Max(0, c.MaxWidth-dw),
		MinHeight: math.Max(0, c.MinHeight-dh),
		MaxHeight: math.Max(0, c.MaxHeight-dh),
	}
}

// WithTightWidth returns constraints with fixed width and same height constraints.
func (c BoxConstraints) WithTightWidth(w float64) BoxConstraints {
	return BoxConstraints{MinWidth: w, MaxWidth: w, MinHeight: c.MinHeight, MaxHeight: c.MaxHeight}
}

// WithTightHeight returns constraints with fixed height and same width constraints.
func (c BoxConstraints) WithTightHeight(h float64) BoxConstraints {
	return BoxConstraints{MinWidth: c.MinWidth, MaxWidth: c.MaxWidth, MinHeight: h, MaxHeight: h}
}

// EdgeInsets represents padding/margin on all four sides.
type EdgeInsets struct{ Top, Right, Bottom, Left float64 }

// All returns EdgeInsets with the same value on all four sides.
func All(v float64) EdgeInsets { return EdgeInsets{v, v, v, v} }

// Symmetric returns EdgeInsets with h on left/right and v on top/bottom.
func Symmetric(h, v float64) EdgeInsets { return EdgeInsets{v, h, v, h} }

// LTRB returns EdgeInsets with explicit left, top, right, bottom values.
func LTRB(left, top, right, bottom float64) EdgeInsets { return EdgeInsets{top, right, bottom, left} }

// Only returns EdgeInsets with explicit per-side values (same as LTRB; use for readability).
func Only(top, right, bottom, left float64) EdgeInsets { return EdgeInsets{top, right, bottom, left} }

// Horizontal returns the total horizontal padding (Left + Right).
func (e EdgeInsets) Horizontal() float64 { return e.Left + e.Right }

// Vertical returns the total vertical padding (Top + Bottom).
func (e EdgeInsets) Vertical() float64 { return e.Top + e.Bottom }

// Deflate shrinks a BoxConstraints by this padding.
func (e EdgeInsets) Deflate(c BoxConstraints) BoxConstraints {
	return c.DeflateBy(e.Horizontal(), e.Vertical())
}

// Offset returns the top-left corner offset created by this padding.
func (e EdgeInsets) Offset() geom.Point { return geom.Pt(e.Left, e.Top) }

// Axis is horizontal or vertical.
type Axis int

const (
	Horizontal Axis = iota
	Vertical
)

// MainAxisAlignment controls how children are placed along the main axis.
type MainAxisAlignment int

const (
	MainStart MainAxisAlignment = iota
	MainEnd
	MainCenter
	MainSpaceBetween
	MainSpaceAround
	MainSpaceEvenly
)

// CrossAxisAlignment controls placement along the cross axis.
type CrossAxisAlignment int

const (
	CrossStart CrossAxisAlignment = iota
	CrossEnd
	CrossCenter
	CrossStretch
)

// MainAxisSize controls whether to be as large as possible or shrink-wrap.
type MainAxisSize int

const (
	MainMin MainAxisSize = iota
	MainMax
)

// Alignment places a child within a parent using a [-1,1]×[-1,1] coordinate.
type Alignment struct{ X, Y float64 }

var (
	AlignTopLeft      = Alignment{-1, -1}
	AlignTopCenter    = Alignment{0, -1}
	AlignTopRight     = Alignment{1, -1}
	AlignCenterLeft   = Alignment{-1, 0}
	AlignCenter       = Alignment{0, 0}
	AlignCenterRight  = Alignment{1, 0}
	AlignBottomLeft   = Alignment{-1, 1}
	AlignBottomCenter = Alignment{0, 1}
	AlignBottomRight  = Alignment{1, 1}
)

// Along returns the size component along the given axis.
func Along(axis Axis, sz geom.Size) float64 {
	if axis == Horizontal {
		return sz.Width
	}
	return sz.Height
}

// Clamp v between lo and hi.
func clamp(v, lo, hi float64) float64 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}
