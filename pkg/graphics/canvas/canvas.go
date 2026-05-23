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

// Package canvas defines the 2D drawing API for the graphics framework.
//
// The coordinate system has (0,0) at the top-left with X right and Y down.
// All coordinates are in logical pixels. State (transform, clip, style) is
// managed as a stack via Save/Restore.
package canvas

import (
	"image"
	"time"

	"avyos.dev/pkg/graphics/color"
	"avyos.dev/pkg/graphics/geom"
)

// Typeface is the minimal interface Canvas uses for text rendering.
// Implemented by font packages (e.g. font/bitmap).
type Typeface interface {
	DrawRune(r rune, size float64, dst *image.RGBA, x, y float64, col color.Color)
	RuneAdvance(r rune, size float64) float64
	LineHeight(size float64) float64
}

// StrokeJoin controls how line segments are joined.
type StrokeJoin int

const (
	JoinMiter StrokeJoin = iota
	JoinRound
	JoinBevel
)

// StrokeCap controls how line endpoints are drawn.
type StrokeCap int

const (
	CapButt StrokeCap = iota
	CapRound
	CapSquare
)

// Canvas is a stateful 2D drawing context backed by a pixel buffer.
type Canvas interface {
	// Size returns the canvas dimensions in logical pixels.
	Size() geom.Size

	// Save pushes the current graphics state onto the stack.
	Save()
	// Restore pops the last saved graphics state.
	Restore()

	// SetTransform replaces the current transform.
	SetTransform(m geom.Matrix)
	// Transform pre-multiplies the current transform with m.
	Transform(m geom.Matrix)
	Translate(dx, dy float64)
	Scale(sx, sy float64)
	Rotate(angle float64)

	// ClipRect intersects the clip region with r.
	ClipRect(r geom.Rect)
	ResetClip()

	SetFillColor(c color.Color)
	SetStrokeColor(c color.Color)
	SetLineWidth(w float64)
	SetLineJoin(j StrokeJoin)
	SetLineCap(c StrokeCap)

	FillRect(r geom.Rect)
	StrokeRect(r geom.Rect)
	FillRoundedRect(r geom.Rect, radius float64)
	StrokeRoundedRect(r geom.Rect, radius float64)
	FillCircle(center geom.Point, radius float64)
	StrokeCircle(center geom.Point, radius float64)
	DrawLine(from, to geom.Point)

	// Path API
	BeginPath()
	MoveTo(p geom.Point)
	LineTo(p geom.Point)
	QuadTo(cp, end geom.Point)
	CubicTo(cp1, cp2, end geom.Point)
	ArcTo(center geom.Point, rx, ry, startAngle, endAngle float64, clockwise bool)
	ClosePath()
	Fill()
	Stroke()

	DrawImage(img image.Image, dst geom.Rect)
	DrawText(text string, pos geom.Point, face Typeface, size float64)
	MeasureText(text string, face Typeface, size float64) geom.Size

	// Clear fills the entire surface ignoring clip and transform.
	Clear(c color.Color)
	// Pixels returns the raw RGBA pixel data (4 bytes per pixel, row-major).
	Pixels() []byte
}

// DirtyTracker is an optional Canvas extension that records the minimal bounding
// rectangle of all pixels written since the last ResetDirty call. Backends can
// use this to skip blitting unchanged regions of the framebuffer.
//
// Not all Canvas implementations expose this interface; callers must type-assert.
type DirtyTracker interface {
	// Dirty returns the accumulated dirty rectangle.
	// Returns an empty rectangle if nothing has been drawn since the last reset.
	Dirty() image.Rectangle
	// ResetDirty clears the accumulated dirty rectangle.
	ResetDirty()
	// SetDirty replaces the accumulated dirty rectangle with r.
	// Use this to override the automatically-tracked rect with a tighter hint.
	SetDirty(r image.Rectangle)
}

// PixelSaver is an optional Canvas extension for saving and restoring
// rectangular pixel regions. Used to implement transparent rounded-corner
// clipping without a full compositing pipeline.
//
// Not all Canvas implementations expose this interface; callers must type-assert.
type PixelSaver interface {
	// SavePixels copies the pixels under the logical rect r and returns them.
	// Returns nil if r is empty or outside the canvas bounds.
	SavePixels(r geom.Rect) []byte
	// RestorePixels writes previously saved pixel data back under r.
	// data must have been produced by SavePixels for the same r.
	RestorePixels(r geom.Rect, data []byte)
}

// OpaqueImageDrawer is an optional Canvas extension for callers that can
// guarantee the source image is fully opaque. Implementations may skip
// per-pixel alpha blending and use faster copy paths.
type OpaqueImageDrawer interface {
	DrawOpaqueImage(img image.Image, dst geom.Rect)
}

// RenderStats captures coarse timing and call counts for common drawing primitives.
// Canvas implementations may expose this for frame-level tracing.
type RenderStats struct {
	FillRectCalls          int
	FillRectTime           time.Duration
	FillRoundedRectCalls   int
	FillRoundedRectTime    time.Duration
	StrokeRoundedRectCalls int
	StrokeRoundedRectTime  time.Duration
	DrawTextCalls          int
	DrawTextTime           time.Duration
	DrawRuneCalls          int
	DrawRuneTime           time.Duration
	DrawImageCalls         int
	DrawImageTime          time.Duration
	ClearCalls             int
	ClearTime              time.Duration
}

// RenderStatsProvider is an optional Canvas extension for collecting primitive-level
// render timings during a frame. Callers must type-assert.
type RenderStatsProvider interface {
	SetRenderStatsEnabled(enabled bool)
	ResetRenderStats()
	RenderStats() RenderStats
}
