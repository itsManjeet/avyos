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

// Row and Column implement flex-box linear layout.
// Expanded and Spacer distribute remaining main-axis space among children.
//
//	widget.Row{Children: []widget.Widget{label, widget.Spacer{}, button}}
//
//	widget.Column{
//	    MainAxisAlignment: layout.MainCenter,
//	    Children: []widget.Widget{header, body},
//	}
package widget

import (
	"avyos.dev/lib/graphics/geom"
	"avyos.dev/lib/graphics/layout"
	"avyos.dev/lib/graphics/paint"
)

// Row lays out its Children horizontally.
type Row struct {
	Children           []Widget
	MainAxisAlignment  layout.MainAxisAlignment
	CrossAxisAlignment layout.CrossAxisAlignment
	MainAxisSize       layout.MainAxisSize
}

func (r Row) RenderChildren(c layout.BoxConstraints, pctx *paint.Context, offset geom.Point, cr ChildRenderer) geom.Size {
	return flexRender(r.Children, layout.Horizontal, r.MainAxisAlignment, r.CrossAxisAlignment, r.MainAxisSize, c, pctx, offset, cr)
}

// Column lays out its Children vertically.
type Column struct {
	Children           []Widget
	MainAxisAlignment  layout.MainAxisAlignment
	CrossAxisAlignment layout.CrossAxisAlignment
	MainAxisSize       layout.MainAxisSize
}

func (col Column) RenderChildren(c layout.BoxConstraints, pctx *paint.Context, offset geom.Point, cr ChildRenderer) geom.Size {
	return flexRender(col.Children, layout.Vertical, col.MainAxisAlignment, col.CrossAxisAlignment, col.MainAxisSize, c, pctx, offset, cr)
}

// Expanded takes up remaining main-axis space within a [Row] or [Column].
// Flex controls the proportion relative to other Expanded siblings (default 1).
type Expanded struct {
	Flex  int
	Child Widget
}

func (e Expanded) RenderChildren(c layout.BoxConstraints, _ *paint.Context, offset geom.Point, cr ChildRenderer) geom.Size {
	if e.Child == nil {
		return c.Biggest()
	}
	return cr.Render(e.Child, c, offset, "child")
}

// Spacer is a flexible empty gap in a [Row] or [Column].
// Flex controls proportion (default 1).
type Spacer struct {
	Flex int
}

func (s Spacer) RenderChildren(c layout.BoxConstraints, _ *paint.Context, _ geom.Point, _ ChildRenderer) geom.Size {
	return c.Biggest()
}

// flexRender is the two-pass flex layout algorithm shared by Row and Column.
func flexRender(
	children []Widget,
	axis layout.Axis,
	mainAlign layout.MainAxisAlignment,
	crossAlign layout.CrossAxisAlignment,
	mainSz layout.MainAxisSize,
	c layout.BoxConstraints,
	pctx *paint.Context,
	offset geom.Point,
	cr ChildRenderer,
) geom.Size {
	n := len(children)
	if n == 0 {
		return flexEmptySize(axis, mainSz, c)
	}

	mMax := mainMax(c, axis)
	xMax := crossMax(c, axis)

	var stackSizes [16]geom.Size
	var sizes []geom.Size
	if n <= len(stackSizes) {
		sizes = stackSizes[:n]
	} else {
		sizes = make([]geom.Size, n)
	}
	totalFlex := 0
	usedMain := 0.0
	maxCross := 0.0

	// Pass 1: measure rigid (non-flex) children.
	for i, child := range children {
		if flex := flexFactor(child); flex > 0 {
			totalFlex += flex
			continue
		}
		remaining := mMax - usedMain
		if remaining < 0 {
			remaining = 0
		}
		childC := rigidConstraints(c, axis, crossAlign, xMax, remaining)
		sz := cr.Measure(child, childC, childPathSlot(i))
		sizes[i] = sz
		usedMain += mainOf(sz, axis)
		if cx := crossOf(sz, axis); cx > maxCross {
			maxCross = cx
		}
	}

	// Pass 2: distribute remaining space to flex children.
	remaining := mMax - usedMain
	if remaining < 0 || mMax >= layout.Inf {
		remaining = 0
	}
	for i, child := range children {
		flex := flexFactor(child)
		if flex <= 0 {
			continue
		}
		childMain := 0.0
		if totalFlex > 0 {
			childMain = remaining * float64(flex) / float64(totalFlex)
		}
		childC := flexConstraints(axis, crossAlign, xMax, childMain)
		sz := cr.Measure(child, childC, childPathSlot(i))
		if axis == layout.Horizontal {
			sz = geom.Sz(childMain, sz.Height)
		} else {
			sz = geom.Sz(sz.Width, childMain)
		}
		sizes[i] = sz
		usedMain += childMain
		if cx := crossOf(sizes[i], axis); cx > maxCross {
			maxCross = cx
		}
	}

	// CrossStretch: override the cross dimension for all children.
	if crossAlign == layout.CrossStretch && xMax < layout.Inf {
		for i := range sizes {
			if axis == layout.Horizontal {
				sizes[i] = geom.Sz(sizes[i].Width, xMax)
			} else {
				sizes[i] = geom.Sz(xMax, sizes[i].Height)
			}
		}
		maxCross = xMax
	}

	totalMainDim := usedMain
	if mainSz == layout.MainMax && mMax < layout.Inf {
		totalMainDim = mMax
	}
	result := flexConstrainSize(axis, c, totalMainDim, maxCross)
	totalMainDim = mainOf(result, axis)
	totalCrossDim := crossOf(result, axis)

	spacing, start := mainSpacing(n, usedMain, totalMainDim, mainAlign)
	mainPos := start

	for i, child := range children {
		sz := sizes[i]
		crossPos := crossPosition(crossOf(sz, axis), totalCrossDim, crossAlign)
		var childOffset geom.Point
		if axis == layout.Horizontal {
			childOffset = geom.Pt(offset.X+mainPos, offset.Y+crossPos)
		} else {
			childOffset = geom.Pt(offset.X+crossPos, offset.Y+mainPos)
		}
		cr.Render(child, layout.Tight(sz.Width, sz.Height), childOffset, childPathSlot(i))
		mainPos += mainOf(sz, axis) + spacing
	}

	return result
}

func flexFactor(w Widget) int {
	switch v := w.(type) {
	case Expanded:
		if v.Flex <= 0 {
			return 1
		}
		return v.Flex
	case Spacer:
		if v.Flex <= 0 {
			return 1
		}
		return v.Flex
	}
	return 0
}

func mainMax(c layout.BoxConstraints, axis layout.Axis) float64 {
	if axis == layout.Horizontal {
		return c.MaxWidth
	}
	return c.MaxHeight
}

func crossMax(c layout.BoxConstraints, axis layout.Axis) float64 {
	if axis == layout.Horizontal {
		return c.MaxHeight
	}
	return c.MaxWidth
}

func mainOf(sz geom.Size, axis layout.Axis) float64 {
	if axis == layout.Horizontal {
		return sz.Width
	}
	return sz.Height
}

func crossOf(sz geom.Size, axis layout.Axis) float64 {
	if axis == layout.Horizontal {
		return sz.Height
	}
	return sz.Width
}

func rigidConstraints(c layout.BoxConstraints, axis layout.Axis, crossAlign layout.CrossAxisAlignment, xMax, maxMain float64) layout.BoxConstraints {
	if axis == layout.Horizontal {
		cc := layout.BoxConstraints{MaxWidth: maxMain, MaxHeight: xMax}
		if crossAlign == layout.CrossStretch && xMax < layout.Inf {
			cc.MinHeight = xMax
		}
		return cc
	}
	cc := layout.BoxConstraints{MaxWidth: xMax, MaxHeight: maxMain}
	if crossAlign == layout.CrossStretch && xMax < layout.Inf {
		cc.MinWidth = xMax
	}
	return cc
}

func flexConstraints(axis layout.Axis, crossAlign layout.CrossAxisAlignment, xMax, main float64) layout.BoxConstraints {
	if axis == layout.Horizontal {
		cc := layout.BoxConstraints{MinWidth: main, MaxWidth: main, MaxHeight: xMax}
		if crossAlign == layout.CrossStretch && xMax < layout.Inf {
			cc.MinHeight = xMax
		}
		return cc
	}
	cc := layout.BoxConstraints{MaxWidth: xMax, MinHeight: main, MaxHeight: main}
	if crossAlign == layout.CrossStretch && xMax < layout.Inf {
		cc.MinWidth = xMax
	}
	return cc
}

func flexEmptySize(axis layout.Axis, mainSz layout.MainAxisSize, c layout.BoxConstraints) geom.Size {
	if mainSz != layout.MainMax {
		return c.Smallest()
	}
	if axis == layout.Horizontal {
		w := c.MaxWidth
		if w >= layout.Inf {
			w = c.MinWidth
		}
		return geom.Sz(w, c.MinHeight)
	}
	h := c.MaxHeight
	if h >= layout.Inf {
		h = c.MinHeight
	}
	return geom.Sz(c.MinWidth, h)
}

func mainSpacing(n int, used, total float64, align layout.MainAxisAlignment) (spacing, start float64) {
	extra := total - used
	if extra < 0 {
		extra = 0
	}
	switch align {
	case layout.MainStart:
		return 0, 0
	case layout.MainEnd:
		return 0, extra
	case layout.MainCenter:
		return 0, extra / 2
	case layout.MainSpaceBetween:
		if n > 1 {
			return extra / float64(n-1), 0
		}
		return 0, extra / 2
	case layout.MainSpaceAround:
		s := extra / float64(n)
		return s, s / 2
	case layout.MainSpaceEvenly:
		s := extra / float64(n+1)
		return s, s
	}
	return 0, 0
}

func crossPosition(childCross, totalCross float64, align layout.CrossAxisAlignment) float64 {
	switch align {
	case layout.CrossEnd:
		return totalCross - childCross
	case layout.CrossCenter:
		return (totalCross - childCross) / 2
	}
	return 0
}

func flexConstrainSize(axis layout.Axis, c layout.BoxConstraints, mainDim, crossDim float64) geom.Size {
	if axis == layout.Horizontal {
		return c.Constrain(geom.Sz(mainDim, crossDim))
	}
	return c.Constrain(geom.Sz(crossDim, mainDim))
}
