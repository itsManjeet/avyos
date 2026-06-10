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

// Package scene defines the retained-mode scene graph.
//
// A scene graph is a tree of Nodes. Each frame the renderer traverses it
// and draws every node onto a canvas.Canvas. This mirrors Flutter's layer tree.
package scene

import (
	"avyos.dev/lib/graphics/canvas"
	"avyos.dev/lib/graphics/color"
	"avyos.dev/lib/graphics/geom"
)

// Node is a drawable element in the scene graph.
type Node interface {
	Draw(c canvas.Canvas)
	Bounds() geom.Rect
}

// Group contains child nodes with an optional transform and clip.
type Group struct {
	Children  []Node
	Transform geom.Matrix
	ClipRect  *geom.Rect
}

// NewGroup creates a Group with an identity transform and the given initial children.
func NewGroup(children ...Node) *Group {
	return &Group{Children: children, Transform: geom.Identity()}
}

// Add appends a child node and returns g, enabling method chaining.
func (g *Group) Add(n Node) *Group {
	g.Children = append(g.Children, n)
	return g
}

func (g *Group) Draw(c canvas.Canvas) {
	c.Save()
	c.Transform(g.Transform)
	if g.ClipRect != nil {
		c.ClipRect(*g.ClipRect)
	}
	for _, child := range g.Children {
		child.Draw(c)
	}
	c.Restore()
}

func (g *Group) Bounds() geom.Rect {
	if len(g.Children) == 0 {
		return geom.Rect{}
	}
	b := g.Children[0].Bounds()
	for _, child := range g.Children[1:] {
		b = b.Union(child.Bounds())
	}
	return b
}

// RectNode draws a rectangle (optionally rounded) with an optional fill and stroke.
// Set Fill true to paint the interior; set LineWidth > 0 to stroke the outline.
type RectNode struct {
	Rect         geom.Rect
	FillColor    color.Color
	StrokeColor  color.Color
	LineWidth    float64
	CornerRadius float64
	Fill         bool
}

// NewRect creates a filled RectNode.
func NewRect(r geom.Rect, c color.Color) *RectNode {
	return &RectNode{Rect: r, FillColor: c, Fill: true}
}

func (n *RectNode) Draw(c canvas.Canvas) {
	if n.Fill {
		c.SetFillColor(n.FillColor)
		if n.CornerRadius > 0 {
			c.FillRoundedRect(n.Rect, n.CornerRadius)
		} else {
			c.FillRect(n.Rect)
		}
	}
	if n.LineWidth > 0 {
		c.SetStrokeColor(n.StrokeColor)
		c.SetLineWidth(n.LineWidth)
		if n.CornerRadius > 0 {
			c.StrokeRoundedRect(n.Rect, n.CornerRadius)
		} else {
			c.StrokeRect(n.Rect)
		}
	}
}

func (n *RectNode) Bounds() geom.Rect { return n.Rect }

// CircleNode draws a circle with an optional fill and stroke.
type CircleNode struct {
	Center      geom.Point
	Radius      float64
	FillColor   color.Color
	StrokeColor color.Color
	LineWidth   float64
	Fill        bool
}

// NewCircle creates a filled CircleNode.
func NewCircle(center geom.Point, radius float64, c color.Color) *CircleNode {
	return &CircleNode{Center: center, Radius: radius, FillColor: c, Fill: true}
}

func (n *CircleNode) Draw(c canvas.Canvas) {
	if n.Fill {
		c.SetFillColor(n.FillColor)
		c.FillCircle(n.Center, n.Radius)
	}
	if n.LineWidth > 0 {
		c.SetStrokeColor(n.StrokeColor)
		c.SetLineWidth(n.LineWidth)
		c.StrokeCircle(n.Center, n.Radius)
	}
}

func (n *CircleNode) Bounds() geom.Rect {
	return geom.NewRect(n.Center.X-n.Radius, n.Center.Y-n.Radius, n.Radius*2, n.Radius*2)
}

// LineNode draws a stroked line segment between two points.
type LineNode struct {
	From, To  geom.Point
	Color     color.Color
	LineWidth float64
}

// NewLine creates a LineNode with LineWidth 1.
func NewLine(from, to geom.Point, c color.Color) *LineNode {
	return &LineNode{From: from, To: to, Color: c, LineWidth: 1}
}

func (n *LineNode) Draw(c canvas.Canvas) {
	c.SetStrokeColor(n.Color)
	c.SetLineWidth(n.LineWidth)
	c.DrawLine(n.From, n.To)
}

func (n *LineNode) Bounds() geom.Rect {
	return geom.FromPoints(
		geom.Pt(fmin(n.From.X, n.To.X), fmin(n.From.Y, n.To.Y)),
		geom.Pt(fmax(n.From.X, n.To.X), fmax(n.From.Y, n.To.Y)),
	)
}

// TextNode draws a string at a position using a given typeface and size.
type TextNode struct {
	Text  string
	Pos   geom.Point
	Color color.Color
	Face  canvas.Typeface
	Size  float64
}

// NewText creates a TextNode.
func NewText(text string, pos geom.Point, face canvas.Typeface, size float64, c color.Color) *TextNode {
	return &TextNode{Text: text, Pos: pos, Face: face, Size: size, Color: c}
}

func (n *TextNode) Draw(c canvas.Canvas) {
	c.SetFillColor(n.Color)
	c.DrawText(n.Text, n.Pos, n.Face, n.Size)
}

func (n *TextNode) Bounds() geom.Rect {
	if n.Face == nil {
		return geom.NewRect(n.Pos.X, n.Pos.Y, 0, 0)
	}
	var w float64
	for _, r := range n.Text {
		w += n.Face.RuneAdvance(r, n.Size)
	}
	return geom.NewRect(n.Pos.X, n.Pos.Y, w, n.Face.LineHeight(n.Size))
}

// PathOpKind identifies a path operation.
type PathOpKind int

const (
	OpMoveTo  PathOpKind = iota // move the pen to P1 without drawing
	OpLineTo                    // draw a straight line to P1
	OpQuadTo                    // draw a quadratic Bézier curve through P1 to P2
	OpCubicTo                   // draw a cubic Bézier curve with control points P1, P2 to P3
	OpArcTo                     // draw an elliptical arc
	OpClose                     // close the current sub-path
)

// PathOp records a single path operation.
type PathOp struct {
	Kind                 PathOpKind
	P1, P2, P3           geom.Point
	RX, RY               float64
	StartAngle, EndAngle float64
	Clockwise            bool
}

// PathNode records path operations and replays them each frame.
type PathNode struct {
	Ops         []PathOp
	FillColor   color.Color
	StrokeColor color.Color
	LineWidth   float64
	DoFill      bool
	DoStroke    bool
}

// NewPath creates an empty PathNode with fill enabled and LineWidth 1.
func NewPath() *PathNode { return &PathNode{DoFill: true, LineWidth: 1} }

// MoveTo appends an OpMoveTo operation, moving the pen to p.
func (n *PathNode) MoveTo(p geom.Point) *PathNode {
	n.Ops = append(n.Ops, PathOp{Kind: OpMoveTo, P1: p})
	return n
}
// LineTo appends a straight line from the current pen position to p.
func (n *PathNode) LineTo(p geom.Point) *PathNode {
	n.Ops = append(n.Ops, PathOp{Kind: OpLineTo, P1: p})
	return n
}

// QuadTo appends a quadratic Bézier curve with control point cp ending at end.
func (n *PathNode) QuadTo(cp, end geom.Point) *PathNode {
	n.Ops = append(n.Ops, PathOp{Kind: OpQuadTo, P1: cp, P2: end})
	return n
}

// CubicTo appends a cubic Bézier curve with control points cp1, cp2 ending at end.
func (n *PathNode) CubicTo(cp1, cp2, end geom.Point) *PathNode {
	n.Ops = append(n.Ops, PathOp{Kind: OpCubicTo, P1: cp1, P2: cp2, P3: end})
	return n
}

// Close closes the current sub-path with a straight line back to the last MoveTo point.
func (n *PathNode) Close() *PathNode {
	n.Ops = append(n.Ops, PathOp{Kind: OpClose})
	return n
}

func (n *PathNode) Draw(c canvas.Canvas) {
	c.BeginPath()
	for _, op := range n.Ops {
		switch op.Kind {
		case OpMoveTo:
			c.MoveTo(op.P1)
		case OpLineTo:
			c.LineTo(op.P1)
		case OpQuadTo:
			c.QuadTo(op.P1, op.P2)
		case OpCubicTo:
			c.CubicTo(op.P1, op.P2, op.P3)
		case OpArcTo:
			c.ArcTo(op.P1, op.RX, op.RY, op.StartAngle, op.EndAngle, op.Clockwise)
		case OpClose:
			c.ClosePath()
		}
	}
	if n.DoFill {
		c.SetFillColor(n.FillColor)
		c.Fill()
	}
	if n.DoStroke {
		c.SetStrokeColor(n.StrokeColor)
		c.SetLineWidth(n.LineWidth)
		c.Stroke()
	}
}

func (n *PathNode) Bounds() geom.Rect {
	if len(n.Ops) == 0 {
		return geom.Rect{}
	}
	var minX, minY, maxX, maxY float64
	first := true
	updateBounds := func(p geom.Point) {
		if p == (geom.Point{}) {
			return
		}
		if first {
			minX, minY, maxX, maxY = p.X, p.Y, p.X, p.Y
			first = false
		} else {
			if p.X < minX {
				minX = p.X
			}
			if p.Y < minY {
				minY = p.Y
			}
			if p.X > maxX {
				maxX = p.X
			}
			if p.Y > maxY {
				maxY = p.Y
			}
		}
	}
	for _, op := range n.Ops {
		updateBounds(op.P1)
		updateBounds(op.P2)
		updateBounds(op.P3)
	}
	return geom.FromPoints(geom.Pt(minX, minY), geom.Pt(maxX, maxY))
}

// Scene is the root of the scene graph with a background and viewport dimensions.
type Scene struct {
	Root       *Group
	Background color.Color
	Width      float64
	Height     float64
}

// New creates a Scene with given dimensions and background color.
func New(width, height float64, background color.Color) *Scene {
	return &Scene{
		Root:       NewGroup(),
		Background: background,
		Width:      width,
		Height:     height,
	}
}

// Add appends a node to the root group.
func (s *Scene) Add(n Node) *Scene {
	s.Root.Add(n)
	return s
}

// Draw renders the entire scene onto c.
func (s *Scene) Draw(c canvas.Canvas) {
	c.Clear(s.Background)
	s.Root.Draw(c)
}

func fmin(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}

func fmax(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}
