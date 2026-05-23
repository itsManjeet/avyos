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

// Supplementary layout primitives beyond the core Row/Column/Stack.
//
//   - [Flex]       — direction-agnostic flex with a uniform gap.
//   - [Grid]       — equal-width columns.
//   - [Wrap]       — flowing horizontal layout that wraps to new rows.
//   - [ScrollArea] — viewport that clips a larger child.
//   - [Splitter]   — two resizable panes with a fixed ratio.
//   - [Separator]  — a thin dividing line.
//   - [AspectRatio]— maintains a fixed width:height ratio.
//   - [Bleed]      — lets a child paint outside its layout box.
package widget

import (
	"math"

	"avyos.dev/pkg/graphics/color"
	"avyos.dev/pkg/graphics/geom"
	"avyos.dev/pkg/graphics/layout"
	"avyos.dev/pkg/graphics/paint"
)

// Flex is a direction-agnostic flex layout with a uniform Gap between children.
// Use [Row] or [Column] when you need per-child alignment control; use Flex
// when you just want a direction-aware list with spacing.
type Flex struct {
	Direction          layout.Axis
	Gap                float64
	MainAxisAlignment  layout.MainAxisAlignment
	CrossAxisAlignment layout.CrossAxisAlignment
	MainAxisSize       layout.MainAxisSize
	Children           []Widget
}

func (f Flex) Build(BuildContext) Widget {
	children := insertGaps(f.Children, f.Direction, f.Gap)
	if f.Direction == layout.Vertical {
		return Column{
			Children:           children,
			MainAxisAlignment:  f.MainAxisAlignment,
			CrossAxisAlignment: f.CrossAxisAlignment,
			MainAxisSize:       f.MainAxisSize,
		}
	}
	return Row{
		Children:           children,
		MainAxisAlignment:  f.MainAxisAlignment,
		CrossAxisAlignment: f.CrossAxisAlignment,
		MainAxisSize:       f.MainAxisSize,
	}
}

// Grid lays children out in a uniform column grid.
// Either Columns (fixed count) or MinChildWidth (auto-computed count) must be set.
type Grid struct {
	Columns       int     // fixed column count; 0 = auto from MinChildWidth
	MinChildWidth float64 // used to compute column count when Columns == 0
	Gap           float64
	Children      []Widget
}

func (g Grid) RenderChildren(c layout.BoxConstraints, _ *paint.Context, offset geom.Point, cr ChildRenderer) geom.Size {
	n := len(g.Children)
	if n == 0 {
		return c.Smallest()
	}

	maxWidth := c.MaxWidth
	if maxWidth >= layout.Inf {
		maxWidth = 0
	}

	cols := g.Columns
	if cols <= 0 {
		cols = columnsForWidth(maxWidth, g.MinChildWidth, g.Gap)
	}
	if cols <= 0 {
		cols = 1
	}

	cellWidth := 0.0
	if maxWidth > 0 {
		cellWidth = (maxWidth - g.Gap*float64(cols-1)) / float64(cols)
		if cellWidth < 0 {
			cellWidth = 0
		}
	}

	rows := (n + cols - 1) / cols
	var stackSizes [16]geom.Size
	var sizes []geom.Size
	if n <= len(stackSizes) {
		sizes = stackSizes[:n]
	} else {
		sizes = make([]geom.Size, n)
	}
	var stackColWidths [8]float64
	var colWidths []float64
	if cols <= len(stackColWidths) {
		colWidths = stackColWidths[:cols]
	} else {
		colWidths = make([]float64, cols)
	}
	var stackRowHeights [16]float64
	var rowHeights []float64
	if rows <= len(stackRowHeights) {
		rowHeights = stackRowHeights[:rows]
	} else {
		rowHeights = make([]float64, rows)
	}

	for i, child := range g.Children {
		childC := c.Loosen()
		if cellWidth > 0 {
			childC = childC.WithTightWidth(cellWidth)
		}
		sz := cr.Measure(child, childC, childPathSlot(i))
		sizes[i] = sz
		col, row := i%cols, i/cols
		if sz.Width > colWidths[col] {
			colWidths[col] = sz.Width
		}
		if sz.Height > rowHeights[row] {
			rowHeights[row] = sz.Height
		}
	}

	totalW := sumWithGap(colWidths, g.Gap)
	totalH := sumWithGap(rowHeights, g.Gap)

	xPos := cumulative(colWidths, g.Gap)
	yPos := cumulative(rowHeights, g.Gap)

	for i, child := range g.Children {
		sz := sizes[i]
		cr.Render(child, layout.Tight(sz.Width, sz.Height),
			geom.Pt(offset.X+xPos[i%cols], offset.Y+yPos[i/cols]),
			childPathSlot(i))
	}

	return c.Constrain(geom.Sz(totalW, totalH))
}

// Wrap lays children in a horizontal flow, wrapping to the next row when the
// available width is exceeded. Useful for tags, chips, or variable-sized items.
type Wrap struct {
	Children   []Widget
	Spacing    float64 // horizontal gap between children
	RunSpacing float64 // vertical gap between rows
}

func (w Wrap) RenderChildren(c layout.BoxConstraints, _ *paint.Context, offset geom.Point, cr ChildRenderer) geom.Size {
	n := len(w.Children)
	if n == 0 {
		return c.Smallest()
	}

	maxW := c.MaxWidth
	if maxW >= layout.Inf {
		maxW = 1e9
	}

	var stackSizes [16]geom.Size
	var sizes []geom.Size
	if n <= len(stackSizes) {
		sizes = stackSizes[:n]
	} else {
		sizes = make([]geom.Size, n)
	}
	for i, child := range w.Children {
		sizes[i] = cr.Measure(child, c.Loosen(), childPathSlot(i))
	}

	type placement struct{ x, y float64 }
	var stackPositions [16]placement
	var positions []placement
	if n <= len(stackPositions) {
		positions = stackPositions[:n]
	} else {
		positions = make([]placement, n)
	}
	x, y, lineH, totalW := 0.0, 0.0, 0.0, 0.0

	for i, sz := range sizes {
		if x > 0 && x+sz.Width > maxW {
			y += lineH + w.RunSpacing
			x = 0
			lineH = 0
		}
		positions[i] = placement{x, y}
		x += sz.Width + w.Spacing
		if x-w.Spacing > totalW {
			totalW = x - w.Spacing
		}
		if sz.Height > lineH {
			lineH = sz.Height
		}
	}
	totalH := y + lineH

	for i, child := range w.Children {
		p := positions[i]
		sz := sizes[i]
		cr.Render(child, layout.Tight(sz.Width, sz.Height),
			geom.Pt(offset.X+p.x, offset.Y+p.y),
			childPathSlot(i))
	}

	return c.Constrain(geom.Sz(totalW, totalH))
}

// ScrollArea clips a larger child to a fixed viewport size.
// The scroll offset is controlled externally via the Offset field.
// Use [Scroll] for a self-contained stateful scroll view.
type ScrollArea struct {
	Width, Height float64
	Offset        geom.Point
	Axis          layout.Axis
	Both          bool
	Child         Widget
	// OnMeasure and OnViewport are called during layout to report sizes back
	// to a parent [Scroll] widget.
	OnMeasure  func(contentSize geom.Size)
	OnViewport func(viewportSize geom.Size)
}

func (s ScrollArea) RenderChildren(c layout.BoxConstraints, pctx *paint.Context, offset geom.Point, cr ChildRenderer) geom.Size {
	if s.Child == nil {
		return c.Smallest()
	}

	viewC := c
	if s.Width > 0 {
		viewC = viewC.WithTightWidth(s.Width)
	}
	if s.Height > 0 {
		viewC = viewC.WithTightHeight(s.Height)
	}

	viewSize := viewC.Biggest()
	if viewSize.Width <= 0 || viewSize.Width >= layout.Inf {
		viewSize.Width = c.ConstrainWidth(240)
	}
	if viewSize.Height <= 0 || viewSize.Height >= layout.Inf {
		viewSize.Height = c.ConstrainHeight(160)
	}
	viewSize = c.Constrain(viewSize)

	if s.OnViewport != nil {
		s.OnViewport(viewSize)
	}

	measureC := layout.Unconstrained()
	if !s.Both {
		if s.Axis == layout.Horizontal {
			measureC = layout.BoxConstraints{
				MinHeight: viewSize.Height,
				MaxHeight: viewSize.Height,
				MaxWidth:  layout.Inf,
			}
		} else {
			measureC = layout.BoxConstraints{
				MinWidth:  viewSize.Width,
				MaxWidth:  viewSize.Width,
				MaxHeight: layout.Inf,
			}
		}
	}

	contentSize := cr.Measure(s.Child, measureC, "child")
	if s.OnMeasure != nil {
		s.OnMeasure(contentSize)
	}

	scrollOffset := geom.Pt(
		clampFloat64(s.Offset.X, 0, maxf(0, contentSize.Width-viewSize.Width)),
		clampFloat64(s.Offset.Y, 0, maxf(0, contentSize.Height-viewSize.Height)),
	)

	if pctx != nil {
		pctx.Save()
		pctx.ClipRect(geom.NewRect(offset.X, offset.Y, viewSize.Width, viewSize.Height))
	}
	cr.Render(s.Child, layout.Tight(contentSize.Width, contentSize.Height),
		geom.Pt(offset.X-scrollOffset.X, offset.Y-scrollOffset.Y), "child")
	if pctx != nil {
		pctx.Restore()
	}

	return viewSize
}

// Splitter divides available space into two panes at a fixed Ratio.
// The panes are separated by a thin handle line.
type Splitter struct {
	Axis      layout.Axis
	Ratio     float64 // fraction [0,1] at which the handle sits; default 0.5
	Thickness float64 // handle line thickness; default 1
	Gap       float64 // additional gap on each side of the handle
	First     Widget
	Second    Widget
}

func (s Splitter) RenderChildren(c layout.BoxConstraints, pctx *paint.Context, offset geom.Point, cr ChildRenderer) geom.Size {
	ratio := s.Ratio
	if ratio <= 0 || ratio >= 1 {
		ratio = 0.5
	}
	thickness := s.Thickness
	if thickness <= 0 {
		thickness = 1
	}

	full := c.Biggest()
	if full.Width >= layout.Inf {
		full.Width = 0
	}
	if full.Height >= layout.Inf {
		full.Height = 0
	}

	if s.Axis == layout.Vertical {
		handleY := full.Height*ratio - thickness/2
		firstH := math.Max(0, handleY-s.Gap/2)
		secondY := handleY + thickness + s.Gap/2
		secondH := math.Max(0, full.Height-secondY)
		if s.First != nil {
			cr.Render(s.First, layout.Tight(full.Width, firstH), offset, "first")
		}
		if pctx != nil {
			pctx.FillRect(geom.NewRect(offset.X, offset.Y+handleY, full.Width, thickness), color.Black.WithAlpha(0.08))
		}
		if s.Second != nil {
			cr.Render(s.Second, layout.Tight(full.Width, secondH), geom.Pt(offset.X, offset.Y+secondY), "second")
		}
		return c.Constrain(full)
	}

	handleX := full.Width*ratio - thickness/2
	firstW := math.Max(0, handleX-s.Gap/2)
	secondX := handleX + thickness + s.Gap/2
	secondW := math.Max(0, full.Width-secondX)
	if s.First != nil {
		cr.Render(s.First, layout.Tight(firstW, full.Height), offset, "first")
	}
	if pctx != nil {
		pctx.FillRect(geom.NewRect(offset.X+handleX, offset.Y, thickness, full.Height), color.Black.WithAlpha(0.08))
	}
	if s.Second != nil {
		cr.Render(s.Second, layout.Tight(secondW, full.Height), geom.Pt(offset.X+secondX, offset.Y), "second")
	}
	return c.Constrain(full)
}

// SeparatorAxis selects horizontal or vertical orientation.
type SeparatorAxis uint8

const (
	SeparatorHorizontal SeparatorAxis = iota
	SeparatorVertical
)

// Separator is a thin dividing line. Use it between sections in a layout.
// Color defaults to the theme's muted border; Thickness defaults to 1.
type Separator struct {
	Axis      SeparatorAxis
	Color     color.Color // A == 0 means use theme default (resolved in Build)
	Thickness float64     // 0 = 1
	Inset     float64     // inset from both ends
	Length    float64     // 0 = fill available space
}

func (s Separator) Build(ctx BuildContext) Widget {
	col := s.Color
	if col.A == 0 {
		col = ctx.Theme.ColorScheme.OutlineVariant
	}
	thickness := s.Thickness
	if thickness <= 0 {
		thickness = 1
	}
	return separatorLeaf{axis: s.Axis, color: col, thickness: thickness, inset: s.Inset, length: s.Length}
}

type separatorLeaf struct {
	axis      SeparatorAxis
	color     color.Color
	thickness float64
	inset     float64
	length    float64
}

func (s separatorLeaf) Layout(c layout.BoxConstraints) geom.Size {
	if s.axis == SeparatorVertical {
		h := s.length
		if h <= 0 {
			h = c.MaxHeight
			if h >= layout.Inf {
				h = 24
			}
		}
		return c.Constrain(geom.Sz(s.thickness, h))
	}
	w := s.length
	if w <= 0 {
		w = c.MaxWidth
		if w >= layout.Inf {
			w = 0
		}
	}
	return c.Constrain(geom.Sz(w, s.thickness))
}

func (s separatorLeaf) Paint(ctx *paint.Context, offset geom.Point, size geom.Size) {
	if s.axis == SeparatorVertical {
		h := maxf(0, size.Height-s.inset*2)
		if h <= 0 {
			return
		}
		ctx.FillRect(geom.NewRect(offset.X, offset.Y+s.inset, s.thickness, h), s.color)
		return
	}
	w := maxf(0, size.Width-s.inset*2)
	if w <= 0 {
		return
	}
	ctx.FillRect(geom.NewRect(offset.X+s.inset, offset.Y, w, s.thickness), s.color)
}

func (s separatorLeaf) HitTest(_, _ geom.Point, _ geom.Size) bool { return false }

// AspectRatio reserves a box whose width and height maintain a fixed ratio.
// Ratio is width / height (e.g. 16.0/9.0 for widescreen). Default 1.
type AspectRatio struct {
	Ratio float64
	Child Widget
}

func (ar AspectRatio) RenderChildren(c layout.BoxConstraints, _ *paint.Context, offset geom.Point, cr ChildRenderer) geom.Size {
	ratio := ar.Ratio
	if ratio <= 0 {
		ratio = 1
	}

	var width, height float64
	switch {
	case c.HasBoundedWidth():
		width = c.MaxWidth
		height = width / ratio
		if c.HasBoundedHeight() && height > c.MaxHeight {
			height = c.MaxHeight
			width = height * ratio
		}
	case c.HasBoundedHeight():
		height = c.MaxHeight
		width = height * ratio
	default:
		if ar.Child != nil {
			childSz := cr.Measure(ar.Child, c.Loosen(), "child")
			width = childSz.Width
			if width <= 0 {
				width = childSz.Height * ratio
			}
			height = width / ratio
		}
	}

	if width <= 0 || height <= 0 {
		return c.Smallest()
	}
	sz := c.Constrain(geom.Sz(width, height))
	if ar.Child != nil {
		cr.Render(ar.Child, layout.Tight(sz.Width, sz.Height), offset, "child")
	}
	return sz
}

// Bleed lets a child paint outside its layout box without affecting the
// surrounding flow. The Insets control how much each edge bleeds.
type Bleed struct {
	Insets layout.EdgeInsets
	Child  Widget
}

func (b Bleed) RenderChildren(c layout.BoxConstraints, _ *paint.Context, offset geom.Point, cr ChildRenderer) geom.Size {
	if b.Child == nil {
		return c.Smallest()
	}
	childC := c.DeflateBy(-b.Insets.Horizontal(), -b.Insets.Vertical())
	childSz := cr.Measure(b.Child, childC, "child")
	size := c.Constrain(geom.Sz(
		maxf(0, childSz.Width-b.Insets.Horizontal()),
		maxf(0, childSz.Height-b.Insets.Vertical()),
	))
	cr.Render(b.Child, layout.Tight(childSz.Width, childSz.Height),
		geom.Pt(offset.X-b.Insets.Left, offset.Y-b.Insets.Top), "child")
	return size
}

// --- helpers ---

func insertGaps(children []Widget, axis layout.Axis, gap float64) []Widget {
	if len(children) == 0 || gap <= 0 {
		return children
	}
	out := make([]Widget, 0, len(children)*2-1)
	for i, child := range children {
		if i > 0 {
			if axis == layout.Vertical {
				out = append(out, SizedBox{Height: gap})
			} else {
				out = append(out, SizedBox{Width: gap})
			}
		}
		out = append(out, child)
	}
	return out
}

func columnsForWidth(maxWidth, minChildWidth, gap float64) int {
	if minChildWidth <= 0 || maxWidth <= 0 {
		return 1
	}
	cols := int(math.Floor((maxWidth + gap) / (minChildWidth + gap)))
	if cols < 1 {
		return 1
	}
	return cols
}

func sumWithGap(vals []float64, gap float64) float64 {
	total := 0.0
	for i, v := range vals {
		if i > 0 {
			total += gap
		}
		total += v
	}
	return total
}

func cumulative(vals []float64, gap float64) []float64 {
	pos := make([]float64, len(vals))
	cursor := 0.0
	for i, v := range vals {
		if i > 0 {
			cursor += gap
		}
		pos[i] = cursor
		cursor += v
	}
	return pos
}
