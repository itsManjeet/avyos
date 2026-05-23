//go:build linux && gpu

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

// Package vulkan provides a GPU-accelerated canvas backed by Vulkan.
// Primitive fills use the Vulkan transfer queue with a host-visible staging
// buffer.  Text, paths, images, and strokes fall back to the embedded
// cpu.Canvas which is composited via vk_canvas_blit_cpu.
package vulkan

/*
#cgo pkg-config: vulkan
#include "vulkan.h"
#include <stdlib.h>
*/
import "C"

import (
	"image"
	"math"
	"unsafe"

	"avyos.dev/pkg/graphics/canvas"
	"avyos.dev/pkg/graphics/canvas/pixbuf"
	"avyos.dev/pkg/graphics/color"
	"avyos.dev/pkg/graphics/geom"
)

// Canvas is a canvas.Canvas backed by a Vulkan offscreen image.
type Canvas struct {
	vc     *C.VkCanvas
	cpu    canvas.Canvas
	dirty  bool
	width  int
	height int
	fill   color.Color

	stack     []gpuState
	transform geom.Matrix
	clip      image.Rectangle
}

type gpuState struct {
	transform geom.Matrix
	clip      image.Rectangle
}

// compile-time interface check
var _ canvas.Canvas = (*Canvas)(nil)

// NewCanvas creates a Vulkan-backed Canvas.  Returns nil if no Vulkan device is found.
func NewCanvas(width, height int) *Canvas {
	vc := C.vk_canvas_create(C.int(width), C.int(height))
	if vc == nil {
		return nil
	}
	return &Canvas{
		vc:        vc,
		cpu:       pixbuf.NewCanvas(width, height),
		width:     width,
		height:    height,
		fill:      color.Color{R: 0, G: 0, B: 0, A: 1},
		transform: geom.Identity(),
		clip:      image.Rect(0, 0, width, height),
	}
}

func (c *Canvas) Size() geom.Size { return geom.Sz(float64(c.width), float64(c.height)) }

func (c *Canvas) Save() {
	c.stack = append(c.stack, gpuState{transform: c.transform, clip: c.clip})
	c.cpu.Save()
}

func (c *Canvas) Restore() {
	if len(c.stack) > 0 {
		s := c.stack[len(c.stack)-1]
		c.stack = c.stack[:len(c.stack)-1]
		c.transform = s.transform
		c.clip = s.clip
	}
	c.cpu.Restore()
}

func (c *Canvas) SetTransform(m geom.Matrix) {
	c.transform = m
	c.cpu.SetTransform(m)
}

func (c *Canvas) Transform(m geom.Matrix) {
	c.transform = c.transform.Mul(m)
	c.cpu.Transform(m)
}

func (c *Canvas) Translate(dx, dy float64) { c.Transform(geom.Translate(dx, dy)) }
func (c *Canvas) Scale(sx, sy float64)     { c.Transform(geom.Scale(sx, sy)) }
func (c *Canvas) Rotate(angle float64)     { c.Transform(geom.Rotate(angle)) }

func (c *Canvas) ClipRect(r geom.Rect) {
	c.clip = c.clip.Intersect(c.rectToImage(r))
	c.cpu.ClipRect(r)
}

func (c *Canvas) ResetClip() {
	c.clip = image.Rect(0, 0, c.width, c.height)
	c.cpu.ResetClip()
}

func (c *Canvas) SetFillColor(col color.Color) {
	c.fill = col
	c.cpu.SetFillColor(col)
}
func (c *Canvas) SetStrokeColor(col color.Color)  { c.cpu.SetStrokeColor(col) }
func (c *Canvas) SetLineWidth(w float64)          { c.cpu.SetLineWidth(w) }
func (c *Canvas) SetLineJoin(j canvas.StrokeJoin) { c.cpu.SetLineJoin(j) }
func (c *Canvas) SetLineCap(cap canvas.StrokeCap) { c.cpu.SetLineCap(cap) }

func (c *Canvas) flushCPU() {
	if !c.dirty {
		return
	}
	pix := c.cpu.Pixels()
	ptr := (*C.uint8_t)(unsafe.Pointer(&pix[0]))
	C.vk_canvas_blit_cpu(c.vc, ptr, C.int(c.width), C.int(c.height), 0, 0)
	c.cpu.Clear(color.Color{})
	c.dirty = false
}

func (c *Canvas) markCPU() { c.dirty = true }

func cf(v float64) C.float { return C.float(v) }

func (c *Canvas) Clear(col color.Color) {
	c.flushCPU()
	C.vk_canvas_clear(c.vc, cf(col.R), cf(col.G), cf(col.B), cf(col.A))
}

func (c *Canvas) FillRect(r geom.Rect) {
	if !c.canAccelerateFill() {
		c.cpu.FillRect(r)
		c.markCPU()
		return
	}
	c.flushCPU()
	fc := c.fill
	C.vk_canvas_fill_rect(c.vc,
		cf(r.Min.X), cf(r.Min.Y), cf(r.Width()), cf(r.Height()),
		cf(fc.R), cf(fc.G), cf(fc.B), cf(fc.A))
}

func (c *Canvas) FillRoundedRect(r geom.Rect, radius float64) {
	if !c.canAccelerateFill() {
		c.cpu.FillRoundedRect(r, radius)
		c.markCPU()
		return
	}
	c.flushCPU()
	fc := c.fill
	C.vk_canvas_fill_rounded_rect(c.vc,
		cf(r.Min.X), cf(r.Min.Y), cf(r.Width()), cf(r.Height()), cf(radius),
		cf(fc.R), cf(fc.G), cf(fc.B), cf(fc.A))
}

func (c *Canvas) FillCircle(center geom.Point, radius float64) {
	if !c.canAccelerateFill() {
		c.cpu.FillCircle(center, radius)
		c.markCPU()
		return
	}
	c.flushCPU()
	fc := c.fill
	C.vk_canvas_fill_circle(c.vc,
		cf(center.X), cf(center.Y), cf(radius),
		cf(fc.R), cf(fc.G), cf(fc.B), cf(fc.A))
}

func (c *Canvas) StrokeRect(r geom.Rect) { c.cpu.StrokeRect(r); c.markCPU() }
func (c *Canvas) StrokeRoundedRect(r geom.Rect, rad float64) {
	c.cpu.StrokeRoundedRect(r, rad)
	c.markCPU()
}
func (c *Canvas) StrokeCircle(center geom.Point, rad float64) {
	c.cpu.StrokeCircle(center, rad)
	c.markCPU()
}
func (c *Canvas) DrawLine(from, to geom.Point) { c.cpu.DrawLine(from, to); c.markCPU() }

func (c *Canvas) BeginPath()                { c.cpu.BeginPath() }
func (c *Canvas) MoveTo(p geom.Point)       { c.cpu.MoveTo(p) }
func (c *Canvas) LineTo(p geom.Point)       { c.cpu.LineTo(p) }
func (c *Canvas) QuadTo(cp, end geom.Point) { c.cpu.QuadTo(cp, end) }
func (c *Canvas) CubicTo(cp1, cp2, end geom.Point) {
	c.cpu.CubicTo(cp1, cp2, end)
}
func (c *Canvas) ArcTo(center geom.Point, rx, ry, sa, ea float64, cw bool) {
	c.cpu.ArcTo(center, rx, ry, sa, ea, cw)
}
func (c *Canvas) ClosePath() { c.cpu.ClosePath() }
func (c *Canvas) Fill()      { c.cpu.Fill(); c.markCPU() }
func (c *Canvas) Stroke()    { c.cpu.Stroke(); c.markCPU() }

func (c *Canvas) DrawImage(img image.Image, dst geom.Rect) {
	c.cpu.DrawImage(img, dst)
	c.markCPU()
}
func (c *Canvas) DrawOpaqueImage(img image.Image, dst geom.Rect) {
	if oc, ok := c.cpu.(canvas.OpaqueImageDrawer); ok {
		oc.DrawOpaqueImage(img, dst)
	} else {
		c.cpu.DrawImage(img, dst)
	}
	c.markCPU()
}
func (c *Canvas) DrawText(text string, pos geom.Point, face canvas.Typeface, size float64) {
	c.cpu.DrawText(text, pos, face, size)
	c.markCPU()
}
func (c *Canvas) DrawRune(r rune, pos geom.Point, face canvas.Typeface, size float64) {
	if rc, ok := c.cpu.(interface {
		DrawRune(r rune, pos geom.Point, face canvas.Typeface, size float64)
	}); ok {
		rc.DrawRune(r, pos, face, size)
	} else {
		c.cpu.DrawText(string(r), pos, face, size)
	}
	c.markCPU()
}
func (c *Canvas) MeasureText(text string, face canvas.Typeface, size float64) geom.Size {
	return c.cpu.MeasureText(text, face, size)
}

// Pixels resolves the Vulkan image to a flat RGBA byte slice via readback.
func (c *Canvas) Pixels() []byte {
	c.flushCPU()
	out := make([]byte, c.width*c.height*4)
	ptr := (*C.uint8_t)(unsafe.Pointer(&out[0]))
	C.vk_canvas_pixels(c.vc, ptr)
	return out
}

// Destroy releases Vulkan resources.
func (c *Canvas) Destroy() {
	if c.vc != nil {
		C.vk_canvas_destroy(c.vc)
		c.vc = nil
	}
}

func (c *Canvas) canAccelerateFill() bool {
	return c.transform.IsIdentity() && c.clip == image.Rect(0, 0, c.width, c.height)
}

func (c *Canvas) rectToImage(r geom.Rect) image.Rectangle {
	tp := c.transform.Transform(r.Min)
	tq := c.transform.Transform(r.Max)
	x0 := int(math.Min(tp.X, tq.X))
	y0 := int(math.Min(tp.Y, tq.Y))
	x1 := int(math.Ceil(math.Max(tp.X, tq.X)))
	y1 := int(math.Ceil(math.Max(tp.Y, tq.Y)))
	return image.Rect(x0, y0, x1, y1)
}
