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

// Package paint provides the PaintContext used by RenderObjects to draw.
//
// PaintContext wraps canvas.Canvas and tracks the accumulated offset so
// each RenderObject can paint in its own local coordinate space.
package paint

import (
	"avyos.dev/lib/graphics/canvas"
	"avyos.dev/lib/graphics/color"
	"avyos.dev/lib/graphics/geom"
)

// Context wraps a canvas.Canvas for use during the paint phase.
// Each RenderObject receives a Context and a local offset; it should
// call Save/Restore around its own painting to preserve state.
type Context struct {
	Canvas canvas.Canvas
}

// NewContext wraps a Canvas in a PaintContext.
func NewContext(c canvas.Canvas) *Context { return &Context{Canvas: c} }

// Save pushes the canvas state.
func (ctx *Context) Save() { ctx.Canvas.Save() }

// Restore pops the canvas state.
func (ctx *Context) Restore() { ctx.Canvas.Restore() }

// Translate moves the canvas origin by offset.
func (ctx *Context) Translate(offset geom.Point) { ctx.Canvas.Translate(offset.X, offset.Y) }

// ClipRect clips subsequent drawing to r (in current local coords).
func (ctx *Context) ClipRect(r geom.Rect) { ctx.Canvas.ClipRect(r) }

// FillRect fills r with c.
func (ctx *Context) FillRect(r geom.Rect, c color.Color) {
	ctx.Canvas.SetFillColor(c)
	ctx.Canvas.FillRect(r)
}

// FillRoundedRect fills r with corner radius and color.
func (ctx *Context) FillRoundedRect(r geom.Rect, radius float64, c color.Color) {
	ctx.Canvas.SetFillColor(c)
	ctx.Canvas.FillRoundedRect(r, radius)
}

// StrokeRect strokes r with width and color.
func (ctx *Context) StrokeRect(r geom.Rect, width float64, c color.Color) {
	ctx.Canvas.SetStrokeColor(c)
	ctx.Canvas.SetLineWidth(width)
	ctx.Canvas.StrokeRect(r)
}

// StrokeRoundedRect strokes a rounded rect.
func (ctx *Context) StrokeRoundedRect(r geom.Rect, radius, width float64, c color.Color) {
	ctx.Canvas.SetStrokeColor(c)
	ctx.Canvas.SetLineWidth(width)
	ctx.Canvas.StrokeRoundedRect(r, radius)
}

// DrawText renders text at pos with face and size.
func (ctx *Context) DrawText(text string, pos geom.Point, face canvas.Typeface, size float64, c color.Color) {
	ctx.Canvas.SetFillColor(c)
	ctx.Canvas.DrawText(text, pos, face, size)
}

// MeasureText returns the bounding size of text.
func (ctx *Context) MeasureText(text string, face canvas.Typeface, size float64) geom.Size {
	return ctx.Canvas.MeasureText(text, face, size)
}
