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

// Stack, Positioned, Align, and Center handle overlay and positional layout.
//
//	widget.Stack{
//	    Children: []widget.Widget{
//	        background,
//	        widget.Positioned{Bottom: widget.Ptr(16), Right: widget.Ptr(16), Child: fab},
//	    },
//	}
//
//	widget.Align{Alignment: layout.AlignBottomRight, Child: label}
//	widget.Center(child)
package widget

import (
	"math"

	"avyos.dev/lib/graphics/geom"
	"avyos.dev/lib/graphics/layout"
	"avyos.dev/lib/graphics/paint"
)

// Stack layers its Children on top of each other.
// Non-[Positioned] children are given the full Stack size.
// [Positioned] children are placed relative to the Stack's edges.
// Children are painted in order: first child is bottommost.
type Stack struct {
	Children []Widget
}

func (s Stack) RenderChildren(c layout.BoxConstraints, pctx *paint.Context, offset geom.Point, cr ChildRenderer) geom.Size {
	n := len(s.Children)
	if n == 0 {
		return c.Smallest()
	}

	// Pass 1: measure non-Positioned children to determine Stack size.
	maxW, maxH := 0.0, 0.0
	for i, child := range s.Children {
		if _, ok := child.(Positioned); ok {
			continue
		}
		sz := cr.Measure(child, c.Loosen(), childPathSlot(i))
		maxW = math.Max(maxW, sz.Width)
		maxH = math.Max(maxH, sz.Height)
	}

	// Fill bounded space, or shrink to children.
	stackW := c.MaxWidth
	if stackW >= layout.Inf {
		stackW = maxW
	}
	stackH := c.MaxHeight
	if stackH >= layout.Inf {
		stackH = maxH
	}
	stackW = c.ConstrainWidth(stackW)
	stackH = c.ConstrainHeight(stackH)
	stackSz := geom.Sz(stackW, stackH)

	// Pass 2: render all children.
	for i, child := range s.Children {
		key := childPathSlot(i)
		if p, ok := child.(Positioned); ok {
			p.renderInStack(stackSz, pctx, offset, cr, key, childMeasurePathSlot(i))
		} else {
			cr.Render(child, layout.Tight(stackW, stackH), offset, key)
		}
	}

	return stackSz
}

// Positioned places a child at explicit offsets within a [Stack].
// Nil pointer fields are unconstrained on that edge.
// When both edges of an axis are set (and the corresponding size is nil),
// the child is stretched to fill the gap.
type Positioned struct {
	Top, Right, Bottom, Left *float64
	Width, Height            *float64
	Child                    Widget
}

// RenderChildren handles the case where Positioned is used outside a Stack.
func (p Positioned) RenderChildren(c layout.BoxConstraints, _ *paint.Context, offset geom.Point, cr ChildRenderer) geom.Size {
	if p.Child == nil {
		return c.Smallest()
	}
	return cr.Render(p.Child, c, offset, "child")
}

func (p Positioned) renderInStack(stackSz geom.Size, _ *paint.Context, stackOffset geom.Point, cr ChildRenderer, key, measureKey string) {
	if p.Child == nil {
		return
	}

	childC := layout.Unconstrained()
	x, y := 0.0, 0.0

	if p.Width != nil {
		childC = childC.WithTightWidth(*p.Width)
	}
	if p.Height != nil {
		childC = childC.WithTightHeight(*p.Height)
	}

	if p.Left != nil && p.Right != nil && p.Width == nil {
		w := stackSz.Width - *p.Left - *p.Right
		if w < 0 {
			w = 0
		}
		childC = childC.WithTightWidth(w)
		x = *p.Left
	} else if p.Left != nil {
		x = *p.Left
	} else if p.Right != nil {
		sz := cr.Measure(p.Child, childC, measureKey)
		x = stackSz.Width - *p.Right - sz.Width
	}

	if p.Top != nil && p.Bottom != nil && p.Height == nil {
		h := stackSz.Height - *p.Top - *p.Bottom
		if h < 0 {
			h = 0
		}
		childC = childC.WithTightHeight(h)
		y = *p.Top
	} else if p.Top != nil {
		y = *p.Top
	} else if p.Bottom != nil {
		sz := cr.Measure(p.Child, childC, measureKey)
		y = stackSz.Height - *p.Bottom - sz.Height
	}

	cr.Render(p.Child, childC, geom.Pt(stackOffset.X+x, stackOffset.Y+y), key)
}

// Ptr returns a pointer to v. Use with [Positioned] fields that accept *float64.
//
//go:fix inline
func Ptr(v float64) *float64 { p := v; return &p }

// Align positions its Child within the available space using fractional
// alignment coordinates: (-1,-1) is top-left, (0,0) is center, (1,1) is
// bottom-right.
type Align struct {
	Alignment layout.Alignment
	Child     Widget
}

func (a Align) RenderChildren(c layout.BoxConstraints, _ *paint.Context, offset geom.Point, cr ChildRenderer) geom.Size {
	childSz := cr.Measure(a.Child, c.Loosen(), "child")

	pW := c.MaxWidth
	if pW >= layout.Inf {
		pW = childSz.Width
	}
	pH := c.MaxHeight
	if pH >= layout.Inf {
		pH = childSz.Height
	}

	childX := offset.X + (pW-childSz.Width)*(a.Alignment.X+1)/2
	childY := offset.Y + (pH-childSz.Height)*(a.Alignment.Y+1)/2
	cr.Render(a.Child, layout.Tight(childSz.Width, childSz.Height), geom.Pt(childX, childY), "child")
	return geom.Sz(pW, pH)
}

// Center is a convenience widget that centres its child in the available space.
func Center(child Widget) Widget {
	return Align{Alignment: layout.AlignCenter, Child: child}
}
