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

// Package pixbuf provides a pure-Go CPU-rasterized canvas.Canvas implementation.
package pixbuf

import (
	"image"
	"math"
	"time"

	"avyos.dev/pkg/graphics/canvas"
	"avyos.dev/pkg/graphics/color"
	"avyos.dev/pkg/graphics/geom"
)

// state holds a saved graphics context snapshot.
type state struct {
	transform geom.Matrix
	clip      image.Rectangle
	fill      color.Color
	stroke    color.Color
	lineWidth float64
	lineJoin  canvas.StrokeJoin
	lineCap   canvas.StrokeCap
}

// pathPoint is a point in the current path.
type pathPoint struct {
	p    geom.Point
	move bool // true = MoveTo, false = line/curve endpoint
}

// Canvas is a CPU-rasterized canvas backed by image.RGBA.
type Canvas struct {
	img    *image.RGBA
	width  int
	height int

	stack     []state
	transform geom.Matrix
	clip      image.Rectangle

	fill      color.Color
	stroke    color.Color
	lineWidth float64
	lineJoin  canvas.StrokeJoin
	lineCap   canvas.StrokeCap

	path []pathPoint

	// Scratch buffers reused across calls to avoid per-frame heap allocations.
	scanXs []float64    // scanline x-intersections for fillPolygon
	pts    []geom.Point // raw polygon points (rounded rects, circles, etc.)
	tpts   []geom.Point // transformed polygon points
	spFlat []geom.Point // flat subpath points for Fill/Stroke

	// Damage tracking: union of all pixel-write operations since last ResetDirty.
	dirty image.Rectangle

	renderStatsEnabled bool
	renderStats        canvas.RenderStats
}

// expandDirty unions r into the accumulated dirty rectangle.
func (c *Canvas) expandDirty(r image.Rectangle) {
	if r.Empty() {
		return
	}
	if c.dirty.Empty() {
		c.dirty = r
	} else {
		c.dirty = c.dirty.Union(r)
	}
}

// Dirty returns the accumulated dirty rectangle since the last ResetDirty call.
func (c *Canvas) Dirty() image.Rectangle { return c.dirty }

// ResetDirty clears the accumulated dirty rectangle.
func (c *Canvas) ResetDirty() { c.dirty = image.Rectangle{} }

// SetDirty replaces the accumulated dirty rectangle with r.
func (c *Canvas) SetDirty(r image.Rectangle) { c.dirty = r }

func (c *Canvas) SetRenderStatsEnabled(enabled bool) { c.renderStatsEnabled = enabled }

func (c *Canvas) ResetRenderStats() { c.renderStats = canvas.RenderStats{} }

func (c *Canvas) RenderStats() canvas.RenderStats { return c.renderStats }

// NewCanvas creates a Canvas with the given pixel dimensions.
func NewCanvas(width, height int) *Canvas {
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	return &Canvas{
		img:       img,
		width:     width,
		height:    height,
		transform: geom.Identity(),
		clip:      image.Rect(0, 0, width, height),
		fill:      color.White,
		stroke:    color.Black,
		lineWidth: 1,
	}
}

// NewCanvasFromImage wraps an existing image.RGBA.
func NewCanvasFromImage(img *image.RGBA) *Canvas {
	b := img.Bounds()
	return &Canvas{
		img:       img,
		width:     b.Dx(),
		height:    b.Dy(),
		transform: geom.Identity(),
		clip:      b,
		fill:      color.White,
		stroke:    color.Black,
		lineWidth: 1,
	}
}

// Image returns the underlying image.RGBA.
func (c *Canvas) Image() *image.RGBA { return c.img }

func (c *Canvas) Size() geom.Size { return geom.Sz(float64(c.width), float64(c.height)) }

func (c *Canvas) Save() {
	c.stack = append(c.stack, state{
		transform: c.transform,
		clip:      c.clip,
		fill:      c.fill,
		stroke:    c.stroke,
		lineWidth: c.lineWidth,
		lineJoin:  c.lineJoin,
		lineCap:   c.lineCap,
	})
}

func (c *Canvas) Restore() {
	if len(c.stack) == 0 {
		return
	}
	s := c.stack[len(c.stack)-1]
	c.stack = c.stack[:len(c.stack)-1]
	c.transform = s.transform
	c.clip = s.clip
	c.fill = s.fill
	c.stroke = s.stroke
	c.lineWidth = s.lineWidth
	c.lineJoin = s.lineJoin
	c.lineCap = s.lineCap
}

func (c *Canvas) SetTransform(m geom.Matrix) { c.transform = m }
func (c *Canvas) Transform(m geom.Matrix)    { c.transform = c.transform.Mul(m) }
func (c *Canvas) Translate(dx, dy float64)   { c.Transform(geom.Translate(dx, dy)) }
func (c *Canvas) Scale(sx, sy float64)       { c.Transform(geom.Scale(sx, sy)) }
func (c *Canvas) Rotate(angle float64)       { c.Transform(geom.Rotate(angle)) }

func (c *Canvas) ClipRect(r geom.Rect) {
	c.clip = c.clip.Intersect(c.rectToImage(r))
}

func (c *Canvas) ResetClip() { c.clip = image.Rect(0, 0, c.width, c.height) }

func (c *Canvas) SetFillColor(col color.Color)    { c.fill = col }
func (c *Canvas) SetStrokeColor(col color.Color)  { c.stroke = col }
func (c *Canvas) SetLineWidth(w float64)          { c.lineWidth = w }
func (c *Canvas) SetLineJoin(j canvas.StrokeJoin) { c.lineJoin = j }
func (c *Canvas) SetLineCap(cap canvas.StrokeCap) { c.lineCap = cap }

func (c *Canvas) FillRect(r geom.Rect) {
	done := c.recordRenderStats(&c.renderStats.FillRectCalls, &c.renderStats.FillRectTime)
	defer done()
	if c.transform.IsIdentity() {
		c.fillRectFast(r, c.fill)
		return
	}
	var arr [4]geom.Point
	c.transformRectInto(&arr, r)
	c.fillPolygon(arr[:], c.fill)
}

// fillRectFast fills an axis-aligned rect directly, bypassing the polygon path.
func (c *Canvas) fillRectFast(r geom.Rect, col color.Color) {
	x0 := int(math.Floor(r.Min.X))
	y0 := int(math.Floor(r.Min.Y))
	x1 := int(math.Ceil(r.Max.X))
	y1 := int(math.Ceil(r.Max.Y))
	if x0 < c.clip.Min.X {
		x0 = c.clip.Min.X
	}
	if y0 < c.clip.Min.Y {
		y0 = c.clip.Min.Y
	}
	if x1 > c.clip.Max.X {
		x1 = c.clip.Max.X
	}
	if y1 > c.clip.Max.Y {
		y1 = c.clip.Max.Y
	}
	if x0 >= x1 || y0 >= y1 {
		return
	}
	c.expandDirty(image.Rect(x0, y0, x1, y1))
	sr, sg, sb, sa := col.RGBA8()
	if sa == 0 {
		return
	}
	pix := c.img.Pix
	stride := c.img.Stride
	pixels := x1 - x0
	if sa == 255 {
		// Fully opaque: SIMD fill each row.
		packed := uint32(sr) | uint32(sg)<<8 | uint32(sb)<<16 | 0xFF000000
		for y := y0; y < y1; y++ {
			fillRow(pix, y*stride+x0*4, pixels, packed)
		}
		return
	}
	for y := y0; y < y1; y++ {
		off := y*stride + x0*4
		for x := x0; x < x1; x++ {
			c.blendAt(pix, off, sr, sg, sb, sa)
			off += 4
		}
	}
}

func (c *Canvas) StrokeRect(r geom.Rect) {
	var arr [4]geom.Point
	c.transformRectInto(&arr, r)
	for i := range arr {
		c.drawLineSegment(arr[i], arr[(i+1)%len(arr)], c.stroke, c.lineWidth)
	}
}

func clampRoundedRectRadius(r geom.Rect, radius float64) float64 {
	if radius <= 0 {
		return 0
	}
	w := r.Max.X - r.Min.X
	h := r.Max.Y - r.Min.Y
	if radius > w/2 {
		radius = w / 2
	}
	if radius > h/2 {
		radius = h / 2
	}
	return radius
}

// fillRoundedRectFast fills an axis-aligned rounded rect with anti-aliased
// corners while filling the large center spans directly.
func (c *Canvas) fillRoundedRectFast(r geom.Rect, radius float64, col color.Color) {
	radius = clampRoundedRectRadius(r, radius)
	if radius <= 0 {
		c.fillRectFast(r, col)
		return
	}
	x0 := int(math.Floor(r.Min.X))
	y0 := int(math.Floor(r.Min.Y))
	x1 := int(math.Ceil(r.Max.X))
	y1 := int(math.Ceil(r.Max.Y))
	if x0 < c.clip.Min.X {
		x0 = c.clip.Min.X
	}
	if y0 < c.clip.Min.Y {
		y0 = c.clip.Min.Y
	}
	if x1 > c.clip.Max.X {
		x1 = c.clip.Max.X
	}
	if y1 > c.clip.Max.Y {
		y1 = c.clip.Max.Y
	}
	if x0 >= x1 || y0 >= y1 {
		return
	}
	c.expandDirty(image.Rect(x0, y0, x1, y1))

	innerMinX := r.Min.X + radius
	innerMaxX := r.Max.X - radius
	innerMinY := r.Min.Y + radius
	innerMaxY := r.Max.Y - radius

	sr, sg, sb, sa := col.RGBA8()
	if sa == 0 {
		return
	}
	pix := c.img.Pix
	stride := c.img.Stride
	packed := uint32(sr) | uint32(sg)<<8 | uint32(sb)<<16 | 0xFF000000

	fillSpan := func(y, start, end int) {
		if start < x0 {
			start = x0
		}
		if end > x1 {
			end = x1
		}
		if start >= end {
			return
		}
		off := y*stride + start*4
		if sa == 255 {
			fillRow(pix, off, end-start, packed)
			return
		}
		for x := start; x < end; x++ {
			c.blendAt(pix, off, sr, sg, sb, sa)
			off += 4
		}
	}

	centerStart := int(math.Ceil(innerMinX - 0.5))
	centerEnd := int(math.Floor(innerMaxX-0.5)) + 1

	for y := y0; y < y1; y++ {
		fy := float64(y) + 0.5

		if fy >= innerMinY && fy <= innerMaxY {
			fillSpan(y, x0, x1)
			continue
		}

		fillSpan(y, centerStart, centerEnd)

		ny := innerMinY
		if fy > innerMaxY {
			ny = innerMaxY
		}
		dy := fy - ny
		dy2 := dy * dy

		leftEnd := centerStart
		if leftEnd > x1 {
			leftEnd = x1
		}
		for x := x0; x < leftEnd; x++ {
			fx := float64(x) + 0.5
			dx := fx - innerMinX
			dist := math.Sqrt(dx*dx + dy2)
			cov := radius - dist + 0.5
			if cov <= 0 {
				continue
			}
			alpha := sa
			if cov < 1 {
				alpha = uint8(float64(sa) * cov)
			}
			off := y*stride + x*4
			if alpha == 255 {
				c.storeOpaqueAt(pix, off, sr, sg, sb)
			} else {
				c.blendAt(pix, off, sr, sg, sb, alpha)
			}
		}

		rightStart := centerEnd
		if rightStart < x0 {
			rightStart = x0
		}
		for x := rightStart; x < x1; x++ {
			fx := float64(x) + 0.5
			dx := fx - innerMaxX
			dist := math.Sqrt(dx*dx + dy2)
			cov := radius - dist + 0.5
			if cov <= 0 {
				continue
			}
			alpha := sa
			if cov < 1 {
				alpha = uint8(float64(sa) * cov)
			}
			off := y*stride + x*4
			if alpha == 255 {
				c.storeOpaqueAt(pix, off, sr, sg, sb)
			} else {
				c.blendAt(pix, off, sr, sg, sb, alpha)
			}
		}
	}
}

func (c *Canvas) FillRoundedRect(r geom.Rect, radius float64) {
	done := c.recordRenderStats(&c.renderStats.FillRoundedRectCalls, &c.renderStats.FillRoundedRectTime)
	defer done()
	radius = clampRoundedRectRadius(r, radius)
	if c.transform.IsIdentity() {
		c.fillRoundedRectFast(r, radius, c.fill)
		return
	}
	segs := roundedRectSegs(radius)
	c.pts = roundedRectPointsInto(c.pts[:0], r, radius, segs)
	c.tpts = transformPointsInto(c.tpts[:0], c.pts, c.transform)
	c.fillPolygon(c.tpts, c.fill)
}

// strokeRoundedRectFast strokes an axis-aligned rounded rect using per-pixel
// distance testing: draw pixels where |dist_from_skeleton - radius| ≤ halfLW.
// This avoids the segment-polygon join gaps of the Wu/fillPolygon approach.
func (c *Canvas) strokeRoundedRectFast(r geom.Rect, radius, lineWidth float64, col color.Color) {
	halfLW := lineWidth / 2
	radius = clampRoundedRectRadius(r, radius)
	x0 := int(math.Floor(r.Min.X - halfLW))
	y0 := int(math.Floor(r.Min.Y - halfLW))
	x1 := int(math.Ceil(r.Max.X + halfLW))
	y1 := int(math.Ceil(r.Max.Y + halfLW))
	if x0 < c.clip.Min.X {
		x0 = c.clip.Min.X
	}
	if y0 < c.clip.Min.Y {
		y0 = c.clip.Min.Y
	}
	if x1 > c.clip.Max.X {
		x1 = c.clip.Max.X
	}
	if y1 > c.clip.Max.Y {
		y1 = c.clip.Max.Y
	}
	if x0 >= x1 || y0 >= y1 {
		return
	}
	c.expandDirty(image.Rect(x0, y0, x1, y1))
	// Skeleton inner rect (the rectangle that the corner arcs center on).
	sMinX := r.Min.X + radius
	sMaxX := r.Max.X - radius
	sMinY := r.Min.Y + radius
	sMaxY := r.Max.Y - radius
	sr, sg, sb, sa := col.RGBA8()
	if sa == 0 {
		return
	}
	pix := c.img.Pix
	stride := c.img.Stride
	for y := y0; y < y1; y++ {
		fy := float64(y) + 0.5
		off := y*stride + x0*4
		for x := x0; x < x1; x++ {
			fx := float64(x) + 0.5
			// Nearest point on the skeleton.
			nx := fx
			if nx < sMinX {
				nx = sMinX
			} else if nx > sMaxX {
				nx = sMaxX
			}
			ny := fy
			if ny < sMinY {
				ny = sMinY
			} else if ny > sMaxY {
				ny = sMaxY
			}
			dx := fx - nx
			dy := fy - ny
			dist := math.Sqrt(dx*dx + dy*dy)
			// Distance from the rounded-rect boundary.
			d := dist - radius
			if d < 0 {
				d = -d
			}
			cov := halfLW - d + 0.5
			if cov <= 0 {
				off += 4
				continue
			}
			var alpha uint8
			if cov >= 1 {
				alpha = sa
			} else {
				alpha = uint8(float64(sa) * cov)
			}
			if alpha == 255 {
				pix[off+0] = sr
				pix[off+1] = sg
				pix[off+2] = sb
				pix[off+3] = 255
			} else {
				c.blendAt(pix, off, sr, sg, sb, alpha)
			}
			off += 4
		}
	}
}

func (c *Canvas) StrokeRoundedRect(r geom.Rect, radius float64) {
	done := c.recordRenderStats(&c.renderStats.StrokeRoundedRectCalls, &c.renderStats.StrokeRoundedRectTime)
	defer done()
	if c.transform.IsIdentity() {
		c.strokeRoundedRectFast(r, radius, c.lineWidth, c.stroke)
		return
	}
	segs := roundedRectSegs(radius)
	c.pts = roundedRectPointsInto(c.pts[:0], r, radius, segs)
	n := len(c.pts)
	for i := range c.pts {
		a := c.transform.Transform(c.pts[i])
		b := c.transform.Transform(c.pts[(i+1)%n])
		c.drawLineSegment(a, b, c.stroke, c.lineWidth)
	}
}

func (c *Canvas) FillCircle(center geom.Point, radius float64) {
	if c.transform.IsIdentity() {
		c.fillCircleFast(center, radius, c.fill)
		return
	}
	segs := circleSegs(radius)
	c.pts = circlePointsInto(c.pts[:0], center, radius, segs)
	c.tpts = transformPointsInto(c.tpts[:0], c.pts, c.transform)
	c.fillPolygon(c.tpts, c.fill)
}

// fillCircleFast fills a circle with anti-aliased edges using distance testing.
func (c *Canvas) fillCircleFast(center geom.Point, radius float64, col color.Color) {
	x0 := int(math.Floor(center.X-radius)) - 1
	y0 := int(math.Floor(center.Y-radius)) - 1
	x1 := int(math.Ceil(center.X+radius)) + 1
	y1 := int(math.Ceil(center.Y+radius)) + 1
	if x0 < c.clip.Min.X {
		x0 = c.clip.Min.X
	}
	if y0 < c.clip.Min.Y {
		y0 = c.clip.Min.Y
	}
	if x1 > c.clip.Max.X {
		x1 = c.clip.Max.X
	}
	if y1 > c.clip.Max.Y {
		y1 = c.clip.Max.Y
	}
	if x0 >= x1 || y0 >= y1 {
		return
	}
	c.expandDirty(image.Rect(x0, y0, x1, y1))
	sr, sg, sb, sa := col.RGBA8()
	pix := c.img.Pix
	stride := c.img.Stride
	for y := y0; y < y1; y++ {
		fy := float64(y) + 0.5
		dy := fy - center.Y
		off := y*stride + x0*4
		for x := x0; x < x1; x++ {
			fx := float64(x) + 0.5
			dx := fx - center.X
			dist := math.Sqrt(dx*dx + dy*dy)
			cov := radius - dist + 0.5
			if cov <= 0 {
				off += 4
				continue
			}
			var alpha uint8
			if cov >= 1 {
				alpha = sa
			} else {
				alpha = uint8(float64(sa) * cov)
			}
			if alpha == 255 {
				pix[off+0] = sr
				pix[off+1] = sg
				pix[off+2] = sb
				pix[off+3] = 255
			} else {
				c.blendAt(pix, off, sr, sg, sb, alpha)
			}
			off += 4
		}
	}
}

func (c *Canvas) StrokeCircle(center geom.Point, radius float64) {
	segs := circleSegs(radius)
	c.pts = circlePointsInto(c.pts[:0], center, radius, segs)
	n := len(c.pts)
	for i := range c.pts {
		a := c.transform.Transform(c.pts[i])
		b := c.transform.Transform(c.pts[(i+1)%n])
		c.drawLineSegment(a, b, c.stroke, c.lineWidth)
	}
}

func (c *Canvas) DrawLine(from, to geom.Point) {
	a := c.transform.Transform(from)
	b := c.transform.Transform(to)
	c.drawLineSegment(a, b, c.stroke, c.lineWidth)
}

func (c *Canvas) BeginPath() { c.path = c.path[:0] }

func (c *Canvas) MoveTo(p geom.Point) { c.path = append(c.path, pathPoint{p, true}) }
func (c *Canvas) LineTo(p geom.Point) { c.path = append(c.path, pathPoint{p, false}) }

func (c *Canvas) QuadTo(cp, end geom.Point) {
	const steps = 16
	start := c.currentPoint()
	for i := 1; i <= steps; i++ {
		t := float64(i) / steps
		p01 := start.Lerp(cp, t)
		p12 := cp.Lerp(end, t)
		c.path = append(c.path, pathPoint{p01.Lerp(p12, t), false})
	}
}

func (c *Canvas) CubicTo(cp1, cp2, end geom.Point) {
	const steps = 24
	start := c.currentPoint()
	for i := 1; i <= steps; i++ {
		t := float64(i) / steps
		p01 := start.Lerp(cp1, t)
		p12 := cp1.Lerp(cp2, t)
		p23 := cp2.Lerp(end, t)
		p := p01.Lerp(p12, t).Lerp(p12.Lerp(p23, t), t)
		c.path = append(c.path, pathPoint{p, false})
	}
}

func (c *Canvas) ArcTo(center geom.Point, rx, ry, startAngle, endAngle float64, clockwise bool) {
	steps := int(math.Abs(endAngle-startAngle)/(math.Pi/32)) + 8
	delta := (endAngle - startAngle) / float64(steps)
	if clockwise {
		delta = -delta
	}
	for i := 0; i <= steps; i++ {
		a := startAngle + float64(i)*delta
		p := geom.Pt(center.X+rx*math.Cos(a), center.Y+ry*math.Sin(a))
		move := i == 0 && len(c.path) == 0
		c.path = append(c.path, pathPoint{p, move})
	}
}

func (c *Canvas) ClosePath() {
	for i := len(c.path) - 1; i >= 0; i-- {
		if c.path[i].move {
			c.path = append(c.path, pathPoint{c.path[i].p, false})
			return
		}
	}
}

func (c *Canvas) Fill() {
	c.iterSubpaths(func(poly []geom.Point) {
		c.tpts = transformPointsInto(c.tpts[:0], poly, c.transform)
		c.fillPolygon(c.tpts, c.fill)
	})
}

func (c *Canvas) Stroke() {
	c.iterSubpaths(func(poly []geom.Point) {
		for i := 1; i < len(poly); i++ {
			a := c.transform.Transform(poly[i-1])
			b := c.transform.Transform(poly[i])
			c.drawLineSegment(a, b, c.stroke, c.lineWidth)
		}
	})
}

func (c *Canvas) currentPoint() geom.Point {
	if len(c.path) > 0 {
		return c.path[len(c.path)-1].p
	}
	return geom.Point{}
}

// iterSubpaths walks the current path and calls fn for each subpath,
// reusing c.spFlat to avoid per-subpath heap allocations.
func (c *Canvas) iterSubpaths(fn func([]geom.Point)) {
	c.spFlat = c.spFlat[:0]
	start := 0
	for _, pp := range c.path {
		if pp.move && len(c.spFlat) > start {
			fn(c.spFlat[start:])
			start = len(c.spFlat)
		}
		c.spFlat = append(c.spFlat, pp.p)
	}
	if len(c.spFlat) > start {
		fn(c.spFlat[start:])
	}
}

func (c *Canvas) DrawImage(img image.Image, dst geom.Rect) {
	c.drawImage(img, dst, false)
}

func (c *Canvas) DrawOpaqueImage(img image.Image, dst geom.Rect) {
	c.drawImage(img, dst, true)
}

func (c *Canvas) drawImage(img image.Image, dst geom.Rect, opaque bool) {
	done := c.recordRenderStats(&c.renderStats.DrawImageCalls, &c.renderStats.DrawImageTime)
	defer done()
	src := img.Bounds()
	if src.Empty() {
		return
	}
	dstFull := c.rectToImage(dst)
	dstR := dstFull.Intersect(c.clip)
	if dstR.Empty() {
		return
	}
	c.expandDirty(dstR)
	// Use the full (unclipped) destination dimensions for source mapping so
	// that partial off-screen drawing shows the correct portion of the source
	// rather than stretching/squeezing it to fit the clipped area.
	dw := float64(dstFull.Dx())
	dh := float64(dstFull.Dy())
	sw := float64(src.Dx())
	sh := float64(src.Dy())
	if dw <= 0 || dh <= 0 {
		return
	}

	if dstFull.Dx() == src.Dx() && dstFull.Dy() == src.Dy() {
		if rgba, ok := img.(*image.RGBA); ok {
			if opaque {
				c.copyOpaqueRows(rgba.Pix, rgba.Stride, dstR, dstFull, src.Min)
				return
			}
			srcX0 := dstR.Min.X - dstFull.Min.X + src.Min.X
			spix := rgba.Pix
			dp := c.img.Pix
			pixels := dstR.Dx()
			for py := dstR.Min.Y; py < dstR.Max.Y; py++ {
				sy := py - dstFull.Min.Y + src.Min.Y
				doff := c.img.PixOffset(dstR.Min.X, py)
				soff := rgba.PixOffset(srcX0, sy)
				for i := 0; i < pixels; i++ {
					sr, sg, sb, sa := spix[soff], spix[soff+1], spix[soff+2], spix[soff+3]
					if sa == 255 {
						dp[doff+0] = sr
						dp[doff+1] = sg
						dp[doff+2] = sb
						dp[doff+3] = 255
					} else if sa != 0 {
						c.blendAt(dp, doff, sr, sg, sb, sa)
					}
					doff += 4
					soff += 4
				}
			}
			return
		}
		if nrgba, ok := img.(*image.NRGBA); ok {
			if opaque {
				c.copyOpaqueRows(nrgba.Pix, nrgba.Stride, dstR, dstFull, src.Min)
				return
			}
			srcX0 := dstR.Min.X - dstFull.Min.X + src.Min.X
			spix := nrgba.Pix
			dp := c.img.Pix
			pixels := dstR.Dx()
			for py := dstR.Min.Y; py < dstR.Max.Y; py++ {
				sy := py - dstFull.Min.Y + src.Min.Y
				doff := c.img.PixOffset(dstR.Min.X, py)
				soff := nrgba.PixOffset(srcX0, sy)
				for i := 0; i < pixels; i++ {
					sr, sg, sb, sa := spix[soff], spix[soff+1], spix[soff+2], spix[soff+3]
					if sa == 255 {
						dp[doff+0] = sr
						dp[doff+1] = sg
						dp[doff+2] = sb
						dp[doff+3] = 255
					} else if sa != 0 {
						c.blendAt(dp, doff, sr, sg, sb, sa)
					}
					doff += 4
					soff += 4
				}
			}
			return
		}

		for py := dstR.Min.Y; py < dstR.Max.Y; py++ {
			sy := py - dstFull.Min.Y + src.Min.Y
			doff := c.img.PixOffset(dstR.Min.X, py)
			for px := dstR.Min.X; px < dstR.Max.X; px++ {
				sx := px - dstFull.Min.X + src.Min.X
				rc, gc, bc, ac := img.At(sx, sy).RGBA()
				if opaque {
					c.storeOpaqueAt(c.img.Pix, doff, uint8(rc>>8), uint8(gc>>8), uint8(bc>>8))
				} else {
					c.blendAt(c.img.Pix, doff, uint8(rc>>8), uint8(gc>>8), uint8(bc>>8), uint8(ac>>8))
				}
				doff += 4
			}
		}
		return
	}

	if rgba, ok := img.(*image.RGBA); ok {
		for py := dstR.Min.Y; py < dstR.Max.Y; py++ {
			sy := sampleCoord(py, dstFull.Min.Y, dh, src.Min.Y, sh)
			doff := c.img.PixOffset(dstR.Min.X, py)
			for px := dstR.Min.X; px < dstR.Max.X; px++ {
				sx := sampleCoord(px, dstFull.Min.X, dw, src.Min.X, sw)
				sr, sg, sb, sa := bilinearSampleRGBA(rgba, sx, sy)
				if opaque {
					c.storeOpaqueAt(c.img.Pix, doff, sr, sg, sb)
				} else {
					c.blendAt(c.img.Pix, doff, sr, sg, sb, sa)
				}
				doff += 4
			}
		}
		return
	}
	if nrgba, ok := img.(*image.NRGBA); ok {
		for py := dstR.Min.Y; py < dstR.Max.Y; py++ {
			sy := sampleCoord(py, dstFull.Min.Y, dh, src.Min.Y, sh)
			doff := c.img.PixOffset(dstR.Min.X, py)
			for px := dstR.Min.X; px < dstR.Max.X; px++ {
				sx := sampleCoord(px, dstFull.Min.X, dw, src.Min.X, sw)
				sr, sg, sb, sa := bilinearSampleNRGBA(nrgba, sx, sy)
				if opaque {
					c.storeOpaqueAt(c.img.Pix, doff, sr, sg, sb)
				} else {
					c.blendAt(c.img.Pix, doff, sr, sg, sb, sa)
				}
				doff += 4
			}
		}
		return
	}

	for py := dstR.Min.Y; py < dstR.Max.Y; py++ {
		sy := sampleCoord(py, dstFull.Min.Y, dh, src.Min.Y, sh)
		doff := c.img.PixOffset(dstR.Min.X, py)
		for px := dstR.Min.X; px < dstR.Max.X; px++ {
			sx := sampleCoord(px, dstFull.Min.X, dw, src.Min.X, sw)
			sr, sg, sb, sa := bilinearSample(img, src, sx, sy)
			if opaque {
				c.storeOpaqueAt(c.img.Pix, doff, sr, sg, sb)
			} else {
				c.blendAt(c.img.Pix, doff, sr, sg, sb, sa)
			}
			doff += 4
		}
	}
}

func (c *Canvas) copyOpaqueRows(spix []byte, stride int, dstR, dstFull image.Rectangle, srcMin image.Point) {
	srcX0 := dstR.Min.X - dstFull.Min.X + srcMin.X
	span := dstR.Dx() * 4
	dp := c.img.Pix
	for py := dstR.Min.Y; py < dstR.Max.Y; py++ {
		sy := py - dstFull.Min.Y + srcMin.Y
		doff := c.img.PixOffset(dstR.Min.X, py)
		soff := sy*stride + srcX0*4
		copy(dp[doff:doff+span], spix[soff:soff+span])
	}
}

func sampleCoord(dstPos, dstMin int, dstSize float64, srcMin int, srcSize float64) float64 {
	return (float64(dstPos-dstMin)+0.5)*(srcSize/dstSize) - 0.5 + float64(srcMin)
}

func bilinearSampleRGBA(img *image.RGBA, fx, fy float64) (uint8, uint8, uint8, uint8) {
	return bilinearSampleAt(img.Bounds(), fx, fy, func(x, y int) (uint8, uint8, uint8, uint8) {
		off := img.PixOffset(x, y)
		return img.Pix[off], img.Pix[off+1], img.Pix[off+2], img.Pix[off+3]
	})
}

func bilinearSampleNRGBA(img *image.NRGBA, fx, fy float64) (uint8, uint8, uint8, uint8) {
	return bilinearSampleAt(img.Bounds(), fx, fy, func(x, y int) (uint8, uint8, uint8, uint8) {
		off := img.PixOffset(x, y)
		return img.Pix[off], img.Pix[off+1], img.Pix[off+2], img.Pix[off+3]
	})
}

func bilinearSample(img image.Image, bounds image.Rectangle, fx, fy float64) (uint8, uint8, uint8, uint8) {
	return bilinearSampleAt(bounds, fx, fy, func(x, y int) (uint8, uint8, uint8, uint8) {
		r, g, b, a := img.At(x, y).RGBA()
		return uint8(r >> 8), uint8(g >> 8), uint8(b >> 8), uint8(a >> 8)
	})
}

func bilinearSampleAt(bounds image.Rectangle, fx, fy float64, at func(x, y int) (uint8, uint8, uint8, uint8)) (uint8, uint8, uint8, uint8) {
	minX := bounds.Min.X
	minY := bounds.Min.Y
	maxX := bounds.Max.X - 1
	maxY := bounds.Max.Y - 1

	if fx < float64(minX) {
		fx = float64(minX)
	} else if fx > float64(maxX) {
		fx = float64(maxX)
	}
	if fy < float64(minY) {
		fy = float64(minY)
	} else if fy > float64(maxY) {
		fy = float64(maxY)
	}

	x0 := int(math.Floor(fx))
	y0 := int(math.Floor(fy))
	x1 := x0 + 1
	y1 := y0 + 1
	if x1 > maxX {
		x1 = maxX
	}
	if y1 > maxY {
		y1 = maxY
	}

	tx := fx - float64(x0)
	ty := fy - float64(y0)

	r00, g00, b00, a00 := at(x0, y0)
	r10, g10, b10, a10 := at(x1, y0)
	r01, g01, b01, a01 := at(x0, y1)
	r11, g11, b11, a11 := at(x1, y1)

	w00 := (1 - tx) * (1 - ty)
	w10 := tx * (1 - ty)
	w01 := (1 - tx) * ty
	w11 := tx * ty

	return premulWeightedColor(
		r00, g00, b00, a00, w00,
		r10, g10, b10, a10, w10,
		r01, g01, b01, a01, w01,
		r11, g11, b11, a11, w11,
	)
}

func premulWeightedColor(
	r00, g00, b00, a00 uint8, w00 float64,
	r10, g10, b10, a10 uint8, w10 float64,
	r01, g01, b01, a01 uint8, w01 float64,
	r11, g11, b11, a11 uint8, w11 float64,
) (uint8, uint8, uint8, uint8) {
	a00w := float64(a00) * w00
	a10w := float64(a10) * w10
	a01w := float64(a01) * w01
	a11w := float64(a11) * w11

	pr := float64(r00)*a00w + float64(r10)*a10w + float64(r01)*a01w + float64(r11)*a11w
	pg := float64(g00)*a00w + float64(g10)*a10w + float64(g01)*a01w + float64(g11)*a11w
	pb := float64(b00)*a00w + float64(b10)*a10w + float64(b01)*a01w + float64(b11)*a11w
	pa := a00w + a10w + a01w + a11w

	if pa <= 0 {
		return 0, 0, 0, 0
	}

	return clampUint8(pr / pa), clampUint8(pg / pa), clampUint8(pb / pa), clampUint8(pa)
}

func clampUint8(v float64) uint8 {
	if v <= 0 {
		return 0
	}
	if v >= 255 {
		return 255
	}
	return uint8(math.Round(v))
}

func (c *Canvas) DrawText(text string, pos geom.Point, face canvas.Typeface, size float64) {
	done := c.recordRenderStats(&c.renderStats.DrawTextCalls, &c.renderStats.DrawTextTime)
	defer done()
	if face == nil {
		return
	}
	// Scale font size by the canvas y-scale so characters render at the correct
	// physical pixel size rather than the logical size.
	scaleY := math.Sqrt(c.transform[2]*c.transform[2] + c.transform[3]*c.transform[3])
	if scaleY <= 0 {
		scaleY = 1
	}
	physSize := size * scaleY

	tp := c.transform.Transform(pos)
	dst := c.textTarget()
	if dst == nil {
		return
	}
	cursor := tp.X
	for _, r := range text {
		face.DrawRune(r, physSize, dst, cursor, tp.Y, c.fill)
		cursor += face.RuneAdvance(r, physSize)
	}
	// Dirty tracking: bounding box of the rendered text.
	c.expandDirty(image.Rect(
		int(tp.X)-1, int(tp.Y-physSize)-1,
		int(cursor)+2, int(tp.Y+physSize)+2,
	).Intersect(c.clip))
}

func (c *Canvas) DrawRune(r rune, pos geom.Point, face canvas.Typeface, size float64) {
	done := c.recordRenderStats(&c.renderStats.DrawRuneCalls, &c.renderStats.DrawRuneTime)
	defer done()
	if face == nil {
		return
	}
	scaleY := math.Sqrt(c.transform[2]*c.transform[2] + c.transform[3]*c.transform[3])
	if scaleY <= 0 {
		scaleY = 1
	}
	tp := c.transform.Transform(pos)
	dst := c.textTarget()
	if dst == nil {
		return
	}
	face.DrawRune(r, size*scaleY, dst, tp.X, tp.Y, c.fill)
}

func (c *Canvas) textTarget() *image.RGBA {
	clip := c.clip.Intersect(c.img.Bounds())
	if clip.Empty() {
		return nil
	}
	if clip == c.img.Bounds() {
		return c.img
	}
	dst, ok := c.img.SubImage(clip).(*image.RGBA)
	if !ok {
		return c.img
	}
	return dst
}

func (c *Canvas) MeasureText(text string, face canvas.Typeface, size float64) geom.Size {
	if face == nil {
		return geom.Size{}
	}
	var w float64
	for _, r := range text {
		w += face.RuneAdvance(r, size)
	}
	return geom.Sz(w, face.LineHeight(size))
}

func (c *Canvas) Clear(col color.Color) {
	done := c.recordRenderStats(&c.renderStats.ClearCalls, &c.renderStats.ClearTime)
	defer done()
	r, g, b, a := col.RGBA8()
	packed := uint32(r) | uint32(g)<<8 | uint32(b)<<16 | uint32(a)<<24
	fillRow(c.img.Pix, 0, c.width*c.height, packed)
	c.expandDirty(image.Rect(0, 0, c.width, c.height))
}

func (c *Canvas) Pixels() []byte { return c.img.Pix }

func (c *Canvas) recordRenderStats(calls *int, total *time.Duration) func() {
	if !c.renderStatsEnabled {
		return func() {}
	}
	start := time.Now()
	return func() {
		*calls = *calls + 1
		*total += time.Since(start)
	}
}

// blendAt blends src (sr,sg,sb,sa) over the pixel at pix[off] using
// integer Porter-Duff source-over. Caller must ensure off is in bounds.
// Fast path when the background is fully opaque (da==255).
func (c *Canvas) blendAt(pix []byte, off int, sr, sg, sb, sa uint8) {
	if sa == 0 {
		return
	}
	if sa == 255 {
		c.storeOpaqueAt(pix, off, sr, sg, sb)
		return
	}
	da := uint32(pix[off+3])
	if da == 0 {
		// Transparent background: just write src.
		pix[off+0] = sr
		pix[off+1] = sg
		pix[off+2] = sb
		pix[off+3] = sa
		return
	}
	// Opaque background fast path: oa = sa + da*(1-sa/255) = 255 when da=255.
	if da == 255 {
		ia := 255 - uint32(sa)
		pix[off+0] = uint8((uint32(sr)*uint32(sa) + uint32(pix[off+0])*ia + 127) >> 8)
		pix[off+1] = uint8((uint32(sg)*uint32(sa) + uint32(pix[off+1])*ia + 127) >> 8)
		pix[off+2] = uint8((uint32(sb)*uint32(sa) + uint32(pix[off+2])*ia + 127) >> 8)
		// alpha stays 255
		return
	}
	// General case.
	ia := 255 - uint32(sa)
	oa := uint32(sa) + (da*ia+127)/255
	if oa == 0 {
		return
	}
	pix[off+0] = uint8((uint32(sr)*uint32(sa) + uint32(pix[off+0])*da*ia/255 + oa/2) / oa)
	pix[off+1] = uint8((uint32(sg)*uint32(sa) + uint32(pix[off+1])*da*ia/255 + oa/2) / oa)
	pix[off+2] = uint8((uint32(sb)*uint32(sa) + uint32(pix[off+2])*da*ia/255 + oa/2) / oa)
	pix[off+3] = uint8(oa)
}

func (c *Canvas) storeOpaqueAt(pix []byte, off int, sr, sg, sb uint8) {
	pix[off+0] = sr
	pix[off+1] = sg
	pix[off+2] = sb
	pix[off+3] = 255
}

// blendPixel blends src over the pixel at (x,y), with clip checking.
func (c *Canvas) blendPixel(x, y int, src color.Color) {
	if x < c.clip.Min.X || x >= c.clip.Max.X || y < c.clip.Min.Y || y >= c.clip.Max.Y {
		return
	}
	sr, sg, sb, sa := src.RGBA8()
	c.blendAt(c.img.Pix, c.img.PixOffset(x, y), sr, sg, sb, sa)
}

func (c *Canvas) drawLineSegment(a, b geom.Point, col color.Color, width float64) {
	if width <= 1 {
		c.wuLine(a.X, a.Y, b.X, b.Y, col)
		return
	}
	dx, dy := b.X-a.X, b.Y-a.Y
	l := math.Sqrt(dx*dx + dy*dy)
	if l == 0 {
		return
	}
	nx, ny := -dy/l*(width/2), dx/l*(width/2)
	// Stack-allocated array: no heap allocation for the 4-point polygon.
	var poly [4]geom.Point
	poly[0] = geom.Point{X: a.X + nx, Y: a.Y + ny}
	poly[1] = geom.Point{X: b.X + nx, Y: b.Y + ny}
	poly[2] = geom.Point{X: b.X - nx, Y: b.Y - ny}
	poly[3] = geom.Point{X: a.X - nx, Y: a.Y - ny}
	c.fillPolygon(poly[:], col)
}

// wuLine draws an anti-aliased line using Xiaolin Wu's algorithm.
func (c *Canvas) wuLine(x0, y0, x1, y1 float64, col color.Color) {
	// Expand dirty by the bounding box of the line (1-pixel margin for AA).
	lx := max(int(math.Floor(math.Min(x0, x1)))-1, c.clip.Min.X)
	ly := max(int(math.Floor(math.Min(y0, y1)))-1, c.clip.Min.Y)
	hx := min(int(math.Ceil(math.Max(x0, x1)))+1, c.clip.Max.X)
	hy := min(int(math.Ceil(math.Max(y0, y1)))+1, c.clip.Max.Y)
	c.expandDirty(image.Rect(lx, ly, hx, hy))

	steep := math.Abs(y1-y0) > math.Abs(x1-x0)
	if steep {
		x0, y0 = y0, x0
		x1, y1 = y1, x1
	}
	if x0 > x1 {
		x0, x1 = x1, x0
		y0, y1 = y1, y0
	}
	dx := x1 - x0
	if dx == 0 {
		c.blendPixel(int(x0), int(y0), col)
		return
	}
	dy := y1 - y0
	gradient := dy / dx

	xend := math.Round(x0)
	yend := y0 + gradient*(xend-x0)
	xgap := rfpart(x0 + 0.5)
	xpxl1 := int(xend)
	ypxl1 := ipart(yend)
	c.plotAA(steep, xpxl1, ypxl1, rfpart(yend)*xgap, col)
	c.plotAA(steep, xpxl1, ypxl1+1, fpart(yend)*xgap, col)
	intery := yend + gradient

	xend = math.Round(x1)
	yend = y1 + gradient*(xend-x1)
	xgap = fpart(x1 + 0.5)
	xpxl2 := int(xend)
	ypxl2 := ipart(yend)
	for x := xpxl1 + 1; x < xpxl2; x++ {
		c.plotAA(steep, x, ipart(intery), rfpart(intery), col)
		c.plotAA(steep, x, ipart(intery)+1, fpart(intery), col)
		intery += gradient
	}
	c.plotAA(steep, xpxl2, ypxl2, rfpart(yend)*xgap, col)
	c.plotAA(steep, xpxl2, ypxl2+1, fpart(yend)*xgap, col)
}

func (c *Canvas) plotAA(steep bool, x, y int, alpha float64, col color.Color) {
	col.A *= alpha
	if steep {
		c.blendPixel(y, x, col)
	} else {
		c.blendPixel(x, y, col)
	}
}

// fillPolygon fills a screen-space polygon using scanline fill with
// coverage-based horizontal anti-aliasing (O(1) per pixel, no pointInPolygon).
func (c *Canvas) fillPolygon(pts []geom.Point, col color.Color) {
	if len(pts) < 3 {
		return
	}
	minY, maxY := pts[0].Y, pts[0].Y
	minX, maxX := pts[0].X, pts[0].X
	for _, p := range pts[1:] {
		if p.Y < minY {
			minY = p.Y
		}
		if p.Y > maxY {
			maxY = p.Y
		}
		if p.X < minX {
			minX = p.X
		}
		if p.X > maxX {
			maxX = p.X
		}
	}
	iy0 := int(math.Ceil(minY))
	iy1 := int(math.Floor(maxY))
	if iy0 < c.clip.Min.Y {
		iy0 = c.clip.Min.Y
	}
	if iy1 >= c.clip.Max.Y {
		iy1 = c.clip.Max.Y - 1
	}
	px0 := max(int(math.Floor(minX)), c.clip.Min.X)
	px1 := min(int(math.Ceil(maxX)), c.clip.Max.X)
	if iy0 <= iy1 && px0 < px1 {
		c.expandDirty(image.Rect(px0, iy0, px1, iy1+1))
	}

	sr, sg, sb, sa := col.RGBA8()
	packed := uint32(sr) | uint32(sg)<<8 | uint32(sb)<<16 | 0xFF000000
	pix := c.img.Pix
	stride := c.img.Stride
	n := len(pts)
	if cap(c.scanXs) < n {
		c.scanXs = make([]float64, 0, n)
	}

	for y := iy0; y <= iy1; y++ {
		fy := float64(y) + 0.5
		xs := c.scanXs[:0]
		for i := range n {
			a := pts[i]
			b := pts[(i+1)%n]
			if (a.Y <= fy && b.Y > fy) || (b.Y <= fy && a.Y > fy) {
				t := (fy - a.Y) / (b.Y - a.Y)
				xs = append(xs, a.X+t*(b.X-a.X))
			}
		}
		// Insertion sort (typically 2-4 values).
		for i := 1; i < len(xs); i++ {
			for j := i; j > 0 && xs[j-1] > xs[j]; j-- {
				xs[j-1], xs[j] = xs[j], xs[j-1]
			}
		}

		for k := 0; k+1 < len(xs); k += 2 {
			leftX := xs[k]
			rightX := xs[k+1]

			// Pixel x has coverage = clamp(min(x+1,rightX) - max(x,leftX), 0, 1)
			// For pixels fully inside [leftX+1, rightX-1], coverage = 1.
			fullX0 := int(math.Ceil(leftX))
			fullX1 := int(math.Floor(rightX))
			// Clamp to clip.
			if fullX0 < c.clip.Min.X {
				fullX0 = c.clip.Min.X
			}
			if fullX1 > c.clip.Max.X {
				fullX1 = c.clip.Max.X
			}

			// Left partial pixel.
			lx := int(math.Floor(leftX))
			if lx >= c.clip.Min.X && lx < c.clip.Max.X && lx < fullX0 {
				cov := (math.Min(float64(lx)+1, rightX) - leftX)
				if cov > 0 {
					alpha := uint8(float64(sa) * cov)
					off := y*stride + lx*4
					c.blendAt(pix, off, sr, sg, sb, alpha)
				}
			}

			// Interior pixels: full coverage, write directly.
			if fullX0 < fullX1 {
				if sa == 255 {
					fillRow(pix, y*stride+fullX0*4, fullX1-fullX0, packed)
				} else {
					off := y*stride + fullX0*4
					for x := fullX0; x < fullX1; x++ {
						c.blendAt(pix, off, sr, sg, sb, sa)
						off += 4
					}
				}
			}

			// Right partial pixel.
			rx := fullX1
			if rx >= c.clip.Min.X && rx < c.clip.Max.X && float64(rx) < rightX {
				cov := rightX - float64(rx)
				if cov > 0 && cov < 1 {
					alpha := uint8(float64(sa) * cov)
					off := y*stride + rx*4
					c.blendAt(pix, off, sr, sg, sb, alpha)
				}
			}
		}
		// Save back grown capacity.
		c.scanXs = xs
	}
}

func (c *Canvas) transformRectInto(dst *[4]geom.Point, r geom.Rect) {
	dst[0] = c.transform.Transform(r.Min)
	dst[1] = c.transform.Transform(geom.Pt(r.Max.X, r.Min.Y))
	dst[2] = c.transform.Transform(r.Max)
	dst[3] = c.transform.Transform(geom.Pt(r.Min.X, r.Max.Y))
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

func roundedRectSegs(radius float64) int {
	segs := min(max(int(radius*0.6), 6), 32)
	return segs
}

func roundedRectPointsInto(dst []geom.Point, r geom.Rect, radius float64, segs int) []geom.Point {
	if radius <= 0 {
		dst = resizePointSlice(dst, 4)
		dst[0] = r.Min
		dst[1] = geom.Pt(r.Max.X, r.Min.Y)
		dst[2] = r.Max
		dst[3] = geom.Pt(r.Min.X, r.Max.Y)
		return dst
	}
	if radius > (r.Max.X-r.Min.X)/2 {
		radius = (r.Max.X - r.Min.X) / 2
	}
	if radius > (r.Max.Y-r.Min.Y)/2 {
		radius = (r.Max.Y - r.Min.Y) / 2
	}
	dst = resizePointSlice(dst, 4*(segs+1))
	index := 0
	addArc := func(cx, cy, start float64) {
		for i := 0; i <= segs; i++ {
			a := start + float64(i)/float64(segs)*(math.Pi/2)
			dst[index] = geom.Pt(cx+radius*math.Cos(a), cy+radius*math.Sin(a))
			index++
		}
	}
	addArc(r.Max.X-radius, r.Min.Y+radius, -math.Pi/2)
	addArc(r.Max.X-radius, r.Max.Y-radius, 0)
	addArc(r.Min.X+radius, r.Max.Y-radius, math.Pi/2)
	addArc(r.Min.X+radius, r.Min.Y+radius, math.Pi)
	return dst
}

func transformPointsInto(dst, src []geom.Point, m geom.Matrix) []geom.Point {
	dst = resizePointSlice(dst, len(src))
	for i, p := range src {
		dst[i] = m.Transform(p)
	}
	return dst
}

func circleSegs(radius float64) int {
	segs := min(int(radius*0.75)+16, 128)
	return segs
}

func circlePointsInto(dst []geom.Point, center geom.Point, radius float64, segments int) []geom.Point {
	dst = resizePointSlice(dst, segments)
	for i := range segments {
		a := float64(i) / float64(segments) * 2 * math.Pi
		dst[i] = geom.Pt(center.X+radius*math.Cos(a), center.Y+radius*math.Sin(a))
	}
	return dst
}

func resizePointSlice(dst []geom.Point, n int) []geom.Point {
	if cap(dst) < n {
		return make([]geom.Point, n)
	}
	return dst[:n]
}

func ipart(x float64) int      { return int(math.Floor(x)) }
func fpart(x float64) float64  { return x - math.Floor(x) }
func rfpart(x float64) float64 { return 1 - fpart(x) }

// SavePixels copies the physical pixels under the logical rect r.
func (c *Canvas) SavePixels(r geom.Rect) []byte {
	ir := c.rectToImage(r).Intersect(c.img.Bounds())
	if ir.Empty() {
		return nil
	}
	w, h := ir.Dx(), ir.Dy()
	buf := make([]byte, w*h*4)
	for y := 0; y < h; y++ {
		srcOff := (ir.Min.Y+y)*c.img.Stride + ir.Min.X*4
		copy(buf[y*w*4:(y+1)*w*4], c.img.Pix[srcOff:srcOff+w*4])
	}
	return buf
}

// RestorePixels writes pixels previously saved with SavePixels back to rect r.
func (c *Canvas) RestorePixels(r geom.Rect, data []byte) {
	if len(data) == 0 {
		return
	}
	ir := c.rectToImage(r).Intersect(c.img.Bounds())
	if ir.Empty() {
		return
	}
	w := ir.Dx()
	for y := 0; y < ir.Dy(); y++ {
		dstOff := (ir.Min.Y+y)*c.img.Stride + ir.Min.X*4
		copy(c.img.Pix[dstOff:dstOff+w*4], data[y*w*4:(y+1)*w*4])
	}
}

var _ canvas.Canvas = (*Canvas)(nil)
var _ canvas.DirtyTracker = (*Canvas)(nil)
var _ canvas.PixelSaver = (*Canvas)(nil)
