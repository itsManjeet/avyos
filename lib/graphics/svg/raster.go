package svg

import (
	"image"
	"image/color"
	"math"

	"avyos.dev/lib/graphics/geom"
)

type alphaMask struct {
	bounds image.Rectangle
	pix    []uint8
}

func newMask(b image.Rectangle) *alphaMask { return &alphaMask{b, make([]uint8, b.Dx()*b.Dy())} }
func fullMask(b image.Rectangle) *alphaMask {
	m := newMask(b)
	for i := range m.pix {
		m.pix[i] = 255
	}
	return m
}
func (m *alphaMask) at(x, y int) uint8 {
	if m == nil {
		return 255
	}
	if !image.Pt(x, y).In(m.bounds) {
		return 0
	}
	return m.pix[(y-m.bounds.Min.Y)*m.bounds.Dx()+x-m.bounds.Min.X]
}
func (m *alphaMask) setMax(x, y int, a uint8) {
	if !image.Pt(x, y).In(m.bounds) {
		return
	}
	i := (y-m.bounds.Min.Y)*m.bounds.Dx() + x - m.bounds.Min.X
	if a > m.pix[i] {
		m.pix[i] = a
	}
}
func intersectMasks(a, b *alphaMask) *alphaMask {
	if a == nil {
		return b
	}
	if b == nil {
		return a
	}
	bounds := a.bounds.Intersect(b.bounds)
	o := newMask(a.bounds)
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			av, bv := uint16(a.at(x, y)), uint16(b.at(x, y))
			o.pix[(y-o.bounds.Min.Y)*o.bounds.Dx()+x-o.bounds.Min.X] = uint8((av*bv + 127) / 255)
		}
	}
	return o
}

var sampleOffsets = [8]geom.Point{
	geom.Pt(.125, .375), geom.Pt(.375, .875), geom.Pt(.625, .125), geom.Pt(.875, .625),
	geom.Pt(.125, .875), geom.Pt(.375, .375), geom.Pt(.625, .625), geom.Pt(.875, .125),
}

func rasterFill(p shapePath, bounds image.Rectangle, rule string) *alphaMask {
	m := newMask(bounds)
	bb := p.bounds()
	x0 := maxInt(bounds.Min.X, int(math.Floor(bb.x))-1)
	y0 := maxInt(bounds.Min.Y, int(math.Floor(bb.y))-1)
	x1 := minInt(bounds.Max.X, int(math.Ceil(bb.x+bb.w))+1)
	y1 := minInt(bounds.Max.Y, int(math.Ceil(bb.y+bb.h))+1)
	for y := y0; y < y1; y++ {
		for x := x0; x < x1; x++ {
			inside := 0
			for _, o := range sampleOffsets {
				if pointInPath(p, float64(x)+o.X, float64(y)+o.Y, rule) {
					inside++
				}
			}
			if inside > 0 {
				m.pix[(y-bounds.Min.Y)*bounds.Dx()+x-bounds.Min.X] = uint8((inside*255 + 4) / 8)
			}
		}
	}
	return m
}

func pointInPath(p shapePath, x, y float64, rule string) bool {
	winding := 0
	cross := 0
	for _, s := range p.subs {
		n := len(s.points)
		if n < 2 {
			continue
		}
		// Fill operations implicitly close every open subpath.
		limit := n
		for i := 0; i < limit; i++ {
			a := s.points[i]
			b := s.points[(i+1)%n]
			if (a.Y <= y && b.Y > y) || (a.Y > y && b.Y <= y) {
				ix := a.X + (y-a.Y)*(b.X-a.X)/(b.Y-a.Y)
				if ix > x {
					cross++
					if b.Y > a.Y {
						winding++
					} else {
						winding--
					}
				}
			}
		}
	}
	if stringsEqualFold(rule, "evenodd") {
		return cross%2 == 1
	}
	return winding != 0
}

type strokeOptions struct {
	width     float64
	cap, join string
	miter     float64
	dash      []float64
	offset    float64
}

func rasterStroke(p shapePath, bounds image.Rectangle, o strokeOptions) *alphaMask {
	m := newMask(bounds)
	if o.width <= 0 {
		return m
	}
	half := o.width / 2
	bb := p.bounds()
	pad := half + 2
	x0 := maxInt(bounds.Min.X, int(math.Floor(bb.x-pad)))
	y0 := maxInt(bounds.Min.Y, int(math.Floor(bb.y-pad)))
	x1 := minInt(bounds.Max.X, int(math.Ceil(bb.x+bb.w+pad)))
	y1 := minInt(bounds.Max.Y, int(math.Ceil(bb.y+bb.h+pad)))
	segments := strokeSegments(p, o.dash, o.offset)
	for y := y0; y < y1; y++ {
		for x := x0; x < x1; x++ {
			covered := 0
			for _, sp := range sampleOffsets {
				q := geom.Pt(float64(x)+sp.X, float64(y)+sp.Y)
				if pointOnStroke(q, segments, half, o.cap, o.join) {
					covered++
				}
			}
			if covered > 0 {
				m.pix[(y-bounds.Min.Y)*bounds.Dx()+x-bounds.Min.X] = uint8((covered*255 + 4) / 8)
			}
		}
	}
	return m
}

type lineSegment struct {
	a, b       geom.Point
	start, end bool
}

func strokeSegments(p shapePath, dash []float64, offset float64) []lineSegment {
	var out []lineSegment
	var pattern float64
	for _, v := range dash {
		if v > 0 {
			pattern += v
		}
	}
	if len(dash)%2 == 1 {
		dash = append(append([]float64{}, dash...), dash...)
		pattern *= 2
	}
	for _, sp := range p.subs {
		n := len(sp.points)
		if n < 2 {
			continue
		}
		count := n - 1
		if sp.closed {
			count = n
		}
		if pattern <= 0 {
			for i := 0; i < count; i++ {
				out = append(out, lineSegment{sp.points[i], sp.points[(i+1)%n], !sp.closed && i == 0, !sp.closed && i == count-1})
			}
			continue
		}
		pos := math.Mod(offset, pattern)
		if pos < 0 {
			pos += pattern
		}
		di := 0
		for pos >= dash[di] && dash[di] > 0 {
			pos -= dash[di]
			di = (di + 1) % len(dash)
		}
		remain := dash[di] - pos
		on := di%2 == 0
		for i := 0; i < count; i++ {
			a, b := sp.points[i], sp.points[(i+1)%n]
			l := distance(a, b)
			if l == 0 {
				continue
			}
			used := 0.0
			for used < l-1e-9 {
				take := math.Min(remain, l-used)
				if on && take > 0 {
					q0 := a.Lerp(b, used/l)
					q1 := a.Lerp(b, (used+take)/l)
					out = append(out, lineSegment{q0, q1, true, true})
				}
				used += take
				remain -= take
				if remain <= 1e-9 {
					di = (di + 1) % len(dash)
					remain = dash[di]
					on = di%2 == 0
				}
			}
		}
	}
	return out
}

func pointOnStroke(p geom.Point, segs []lineSegment, half float64, cap, join string) bool {
	for _, s := range segs {
		a, b := s.a, s.b
		v := b.Sub(a)
		l2 := v.Dot(v)
		if l2 == 0 {
			if distance(p, a) <= half {
				return true
			}
			continue
		}
		t := p.Sub(a).Dot(v) / l2
		if cap == "square" {
			ext := half / math.Sqrt(l2)
			t = math.Max(-ext, math.Min(1+ext, t))
		} else {
			t = math.Max(0, math.Min(1, t))
		}
		q := a.Add(v.Scale(t))
		if distance(p, q) <= half {
			if cap == "butt" && ((s.start && p.Sub(a).Dot(v) < 0) || (s.end && p.Sub(b).Dot(v) > 0)) {
				continue
			}
			return true
		}
	}
	return false
}

type paintSampler interface {
	at(x, y float64) color.NRGBA
}
type solidPaint struct{ c color.NRGBA }

func (s solidPaint) at(x, y float64) color.NRGBA { return s.c }

func paintMask(dst *image.RGBA, m, clip *alphaMask, p paintSampler, opacity float64) {
	if m == nil || p == nil || opacity <= 0 {
		return
	}
	b := dst.Bounds().Intersect(m.bounds)
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			a := uint32(m.at(x, y))
			if clip != nil {
				a = a * uint32(clip.at(x, y)) / 255
			}
			if a == 0 {
				continue
			}
			src := p.at(float64(x)+.5, float64(y)+.5)
			a = a * uint32(src.A) / 255
			a = uint32(float64(a) * clamp01(opacity))
			if a == 0 {
				continue
			}
			blendPixel(dst, x, y, color.NRGBA{src.R, src.G, src.B, uint8(a)})
		}
	}
}
func blendPixel(dst *image.RGBA, x, y int, s color.NRGBA) {
	i := dst.PixOffset(x, y)
	sa := uint32(s.A)
	da := uint32(dst.Pix[i+3])
	oa := sa + da*(255-sa)/255
	if oa == 0 {
		return
	}
	for k, v := range []uint8{s.R, s.G, s.B} {
		num := uint32(v)*sa + uint32(dst.Pix[i+k])*da*(255-sa)/255
		dst.Pix[i+k] = uint8(num / oa)
	}
	dst.Pix[i+3] = uint8(oa)
}
func composite(dst, src *image.RGBA, clip *alphaMask, opacity float64) {
	b := dst.Bounds().Intersect(src.Bounds())
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			i := src.PixOffset(x, y)
			a := float64(src.Pix[i+3]) / 255 * opacity
			if clip != nil {
				a *= float64(clip.at(x, y)) / 255
			}
			if a <= 0 {
				continue
			}
			blendPixel(dst, x, y, color.NRGBA{src.Pix[i], src.Pix[i+1], src.Pix[i+2], uint8(math.Round(a * 255))})
		}
	}
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
func stringsEqualFold(a, b string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		x, y := a[i], b[i]
		if x >= 'A' && x <= 'Z' {
			x += 32
		}
		if y >= 'A' && y <= 'Z' {
			y += 32
		}
		if x != y {
			return false
		}
	}
	return true
}
