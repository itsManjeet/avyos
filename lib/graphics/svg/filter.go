package svg

import (
	"image"
	"image/color"
	"math"
	"strings"
)

// applyFilter implements the SVG filter graph primitives that operate on
// raster inputs. Unsupported lighting/image primitives leave their input
// unchanged, which is preferable to dropping the filtered element entirely.
func (r *renderer) applyFilter(src *image.RGBA, raw string, ctx renderContext) *image.RGBA {
	id, _ := paintURL(raw)
	if id == "" {
		id = strings.TrimPrefix(strings.TrimSpace(raw), "#")
	}
	f := r.doc.ids[id]
	if f == nil || localName(f.start.Name.Local) != "filter" {
		return src
	}
	results := map[string]*image.RGBA{"SourceGraphic": src, "SourceAlpha": alphaImage(src)}
	last := src
	get := func(name string) *image.RGBA {
		if name == "" {
			return last
		}
		if v := results[name]; v != nil {
			return v
		}
		return last
	}
	for _, child := range f.children {
		if child.elem == nil {
			continue
		}
		n, a := child.elem, attrMap(child.elem)
		in := get(a["in"])
		var out *image.RGBA
		switch localName(n.start.Name.Local) {
		case "feGaussianBlur":
			v := scanNumbers(a["stdDeviation"])
			sx, sy := 0.0, 0.0
			if len(v) > 0 {
				sx, sy = v[0], v[0]
			}
			if len(v) > 1 {
				sy = v[1]
			}
			out = gaussianBlur(in, sx*matrixScale(ctx.transform), sy*matrixScale(ctx.transform))
		case "feOffset":
			dx := parseNumber(a["dx"], 0) * matrixScale(ctx.transform)
			dy := parseNumber(a["dy"], 0) * matrixScale(ctx.transform)
			out = offsetImage(in, int(math.Round(dx)), int(math.Round(dy)))
		case "feColorMatrix":
			out = colorMatrix(in, a["type"], scanNumbers(a["values"]))
		case "feFlood":
			st := r.doc.styleFor(n, ctx.style)
			c, _ := parseColor(st["flood-color"], color.NRGBA{0, 0, 0, 255})
			c.A = uint8(float64(c.A) * clamp01(parseNumber(st["flood-opacity"], 1)))
			out = solidImage(src.Bounds(), c)
		case "feBlend":
			out = blendImages(in, get(a["in2"]), a["mode"])
		case "feComposite":
			out = compositeImages(in, get(a["in2"]), a)
		case "feMerge":
			out = image.NewRGBA(src.Bounds())
			for _, mc := range n.children {
				if mc.elem != nil && localName(mc.elem.start.Name.Local) == "feMergeNode" {
					composite(out, get(attrMap(mc.elem)["in"]), nil, 1)
				}
			}
		case "feComponentTransfer":
			out = componentTransfer(in, n)
		default:
			out = cloneRGBA(in)
		}
		if out == nil {
			out = cloneRGBA(in)
		}
		last = out
		if name := a["result"]; name != "" {
			results[name] = out
		}
	}
	return last
}

func alphaImage(src *image.RGBA) *image.RGBA {
	o := image.NewRGBA(src.Bounds())
	for y := src.Bounds().Min.Y; y < src.Bounds().Max.Y; y++ {
		for x := src.Bounds().Min.X; x < src.Bounds().Max.X; x++ {
			i := src.PixOffset(x, y)
			o.Pix[i+3] = src.Pix[i+3]
		}
	}
	return o
}
func solidImage(b image.Rectangle, c color.NRGBA) *image.RGBA {
	o := image.NewRGBA(b)
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			i := o.PixOffset(x, y)
			o.Pix[i], o.Pix[i+1], o.Pix[i+2], o.Pix[i+3] = c.R, c.G, c.B, c.A
		}
	}
	return o
}
func offsetImage(src *image.RGBA, dx, dy int) *image.RGBA {
	o := image.NewRGBA(src.Bounds())
	drawRect := src.Bounds().Add(image.Pt(dx, dy)).Intersect(o.Bounds())
	for y := drawRect.Min.Y; y < drawRect.Max.Y; y++ {
		for x := drawRect.Min.X; x < drawRect.Max.X; x++ {
			si, di := src.PixOffset(x-dx, y-dy), o.PixOffset(x, y)
			copy(o.Pix[di:di+4], src.Pix[si:si+4])
		}
	}
	return o
}

func gaussianBlur(src *image.RGBA, sx, sy float64) *image.RGBA {
	rx := minInt(128, int(math.Ceil(math.Max(0, sx)*3)))
	ry := minInt(128, int(math.Ceil(math.Max(0, sy)*3)))
	if rx == 0 && ry == 0 {
		return cloneRGBA(src)
	}
	tmp := blurAxis(src, rx, true)
	return blurAxis(tmp, ry, false)
}
func blurAxis(src *image.RGBA, radius int, horizontal bool) *image.RGBA {
	if radius <= 0 {
		return cloneRGBA(src)
	}
	o := image.NewRGBA(src.Bounds())
	sigma := float64(radius) / 3
	if sigma < .5 {
		sigma = .5
	}
	weights := make([]float64, 2*radius+1)
	sum := 0.0
	for i := -radius; i <= radius; i++ {
		w := math.Exp(-float64(i*i) / (2 * sigma * sigma))
		weights[i+radius] = w
		sum += w
	}
	for i := range weights {
		weights[i] /= sum
	}
	b := src.Bounds()
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			var v [4]float64
			for k := -radius; k <= radius; k++ {
				xx, yy := x, y
				if horizontal {
					xx += k
				} else {
					yy += k
				}
				if !image.Pt(xx, yy).In(b) {
					continue
				}
				si := src.PixOffset(xx, yy)
				w := weights[k+radius]
				for c := 0; c < 4; c++ {
					v[c] += float64(src.Pix[si+c]) * w
				}
			}
			di := o.PixOffset(x, y)
			for c := 0; c < 4; c++ {
				o.Pix[di+c] = uint8(math.Round(math.Min(255, v[c])))
			}
		}
	}
	return o
}

func colorMatrix(src *image.RGBA, kind string, v []float64) *image.RGBA {
	m := [20]float64{1, 0, 0, 0, 0, 0, 1, 0, 0, 0, 0, 0, 1, 0, 0, 0, 0, 0, 1, 0}
	switch kind {
	case "saturate":
		s := 1.0
		if len(v) > 0 {
			s = v[0]
		}
		m = [20]float64{.213 + .787*s, .715 - .715*s, .072 - .072*s, 0, 0, .213 - .213*s, .715 + .285*s, .072 - .072*s, 0, 0, .213 - .213*s, .715 - .715*s, .072 + .928*s, 0, 0, 0, 0, 0, 1, 0}
	case "hueRotate":
		a := 0.0
		if len(v) > 0 {
			a = v[0] * math.Pi / 180
		}
		c, s := math.Cos(a), math.Sin(a)
		m = [20]float64{.213 + .787*c - .213*s, .715 - .715*c - .715*s, .072 - .072*c + .928*s, 0, 0, .213 - .213*c + .143*s, .715 + .285*c + .140*s, .072 - .072*c - .283*s, 0, 0, .213 - .213*c - .787*s, .715 - .715*c + .715*s, .072 + .928*c + .072*s, 0, 0, 0, 0, 0, 1, 0}
	case "luminanceToAlpha":
		m = [20]float64{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, .2126, .7152, .0722, 0, 0}
	default:
		if len(v) >= 20 {
			copy(m[:], v[:20])
		}
	}
	o := image.NewRGBA(src.Bounds())
	for y := src.Bounds().Min.Y; y < src.Bounds().Max.Y; y++ {
		for x := src.Bounds().Min.X; x < src.Bounds().Max.X; x++ {
			i := src.PixOffset(x, y)
			p := [4]float64{float64(src.Pix[i]) / 255, float64(src.Pix[i+1]) / 255, float64(src.Pix[i+2]) / 255, float64(src.Pix[i+3]) / 255}
			for row := 0; row < 4; row++ {
				z := m[row*5+4]
				for k := 0; k < 4; k++ {
					z += m[row*5+k] * p[k]
				}
				o.Pix[i+row] = uint8(math.Round(clamp01(z) * 255))
			}
		}
	}
	return o
}

func blendImages(a, b *image.RGBA, mode string) *image.RGBA {
	o := image.NewRGBA(a.Bounds())
	for y := o.Bounds().Min.Y; y < o.Bounds().Max.Y; y++ {
		for x := o.Bounds().Min.X; x < o.Bounds().Max.X; x++ {
			ia, ib := a.PixOffset(x, y), b.PixOffset(x, y)
			ca, cb := color.NRGBA{a.Pix[ia], a.Pix[ia+1], a.Pix[ia+2], a.Pix[ia+3]}, color.NRGBA{b.Pix[ib], b.Pix[ib+1], b.Pix[ib+2], b.Pix[ib+3]}
			blend := func(x, y uint8) uint8 {
				switch mode {
				case "multiply":
					return uint8(uint16(x) * uint16(y) / 255)
				case "screen":
					return 255 - uint8(uint16(255-x)*uint16(255-y)/255)
				case "darken":
					if x < y {
						return x
					}
					return y
				case "lighten":
					if x > y {
						return x
					}
					return y
				default:
					return x
				}
			}
			c := color.NRGBA{blend(ca.R, cb.R), blend(ca.G, cb.G), blend(ca.B, cb.B), ca.A}
			blendPixel(o, x, y, cb)
			blendPixel(o, x, y, c)
		}
	}
	return o
}
func compositeImages(a, b *image.RGBA, attrs map[string]string) *image.RGBA {
	o := image.NewRGBA(a.Bounds())
	op := attrs["operator"]
	if op == "" || op == "over" {
		composite(o, b, nil, 1)
		composite(o, a, nil, 1)
		return o
	}
	for y := o.Bounds().Min.Y; y < o.Bounds().Max.Y; y++ {
		for x := o.Bounds().Min.X; x < o.Bounds().Max.X; x++ {
			ia, ib := a.PixOffset(x, y), b.PixOffset(x, y)
			aa, ba := float64(a.Pix[ia+3])/255, float64(b.Pix[ib+3])/255
			fa, fb := 1.0, 0.0
			switch op {
			case "in":
				fa = ba
			case "out":
				fa = 1 - ba
			case "atop":
				fa = ba
				fb = 1 - aa
			case "xor":
				fa = 1 - ba
				fb = 1 - aa
			case "arithmetic":
				k := scanNumbers(attrs["k1"] + " " + attrs["k2"] + " " + attrs["k3"] + " " + attrs["k4"])
				if len(k) < 4 {
					k = []float64{0, 0, 0, 0}
				}
				for c := 0; c < 4; c++ {
					av, bv := float64(a.Pix[ia+c])/255, float64(b.Pix[ib+c])/255
					o.Pix[ia+c] = uint8(math.Round(clamp01(k[0]*av*bv+k[1]*av+k[2]*bv+k[3]) * 255))
				}
				continue
			}
			for c := 0; c < 4; c++ {
				o.Pix[ia+c] = uint8(math.Round(clamp01(float64(a.Pix[ia+c])/255*fa+float64(b.Pix[ib+c])/255*fb) * 255))
			}
		}
	}
	return o
}

func componentTransfer(src *image.RGBA, n *xmlNode) *image.RGBA {
	o := cloneRGBA(src)
	for _, ch := range n.children {
		if ch.elem == nil {
			continue
		}
		name := localName(ch.elem.start.Name.Local)
		channel := -1
		switch name {
		case "feFuncR":
			channel = 0
		case "feFuncG":
			channel = 1
		case "feFuncB":
			channel = 2
		case "feFuncA":
			channel = 3
		}
		if channel < 0 {
			continue
		}
		a := attrMap(ch.elem)
		table := scanNumbers(a["tableValues"])
		for y := o.Bounds().Min.Y; y < o.Bounds().Max.Y; y++ {
			for x := o.Bounds().Min.X; x < o.Bounds().Max.X; x++ {
				i := o.PixOffset(x, y)
				v := float64(o.Pix[i+channel]) / 255
				switch a["type"] {
				case "linear":
					v = parseNumber(a["slope"], 1)*v + parseNumber(a["intercept"], 0)
				case "gamma":
					v = parseNumber(a["amplitude"], 1)*math.Pow(v, parseNumber(a["exponent"], 1)) + parseNumber(a["offset"], 0)
				case "table":
					if len(table) > 1 {
						p := v * float64(len(table)-1)
						j := minInt(len(table)-2, int(p))
						v = table[j] + (table[j+1]-table[j])*(p-float64(j))
					}
				case "discrete":
					if len(table) > 0 {
						v = table[minInt(len(table)-1, int(v*float64(len(table))))]
					}
				}
				o.Pix[i+channel] = uint8(math.Round(clamp01(v) * 255))
			}
		}
	}
	return o
}
