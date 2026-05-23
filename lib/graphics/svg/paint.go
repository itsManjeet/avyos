package svg

import (
	"image"
	"image/color"
	"math"
	"sort"
	"strings"

	"avyos.dev/lib/graphics/geom"
)

type colorStop struct {
	offset float64
	c      color.NRGBA
}
type gradientPaint struct {
	kind   string
	inv    geom.Matrix
	v      [6]float64
	stops  []colorStop
	spread string
}

func (g gradientPaint) at(x, y float64) color.NRGBA {
	q := g.inv.Transform(geom.Pt(x, y))
	t := 0.0
	if g.kind == "linear" {
		dx, dy := g.v[2]-g.v[0], g.v[3]-g.v[1]
		den := dx*dx + dy*dy
		if den > 0 {
			t = ((q.X-g.v[0])*dx + (q.Y-g.v[1])*dy) / den
		}
	} else {
		cx, cy, r, fx, fy := g.v[0], g.v[1], g.v[2], g.v[3], g.v[4]
		dx, dy := q.X-fx, q.Y-fy
		ox, oy := fx-cx, fy-cy
		a := dx*dx + dy*dy
		b := 2 * (ox*dx + oy*dy)
		c := ox*ox + oy*oy - r*r
		disc := b*b - 4*a*c
		if a > 0 && disc >= 0 {
			root := (-b + math.Sqrt(disc)) / (2 * a)
			if root > 0 {
				t = 1 / root
			}
		}
	}
	switch g.spread {
	case "repeat":
		t -= math.Floor(t)
	case "reflect":
		t = math.Mod(t, 2)
		if t < 0 {
			t += 2
		}
		if t > 1 {
			t = 2 - t
		}
	default:
		t = clamp01(t)
	}
	return sampleStops(g.stops, t)
}
func sampleStops(s []colorStop, t float64) color.NRGBA {
	if len(s) == 0 {
		return color.NRGBA{}
	}
	if t <= s[0].offset {
		return s[0].c
	}
	for i := 1; i < len(s); i++ {
		if t <= s[i].offset {
			d := s[i].offset - s[i-1].offset
			if d <= 0 {
				return s[i].c
			}
			u := (t - s[i-1].offset) / d
			lerp := func(a, b uint8) uint8 { return uint8(math.Round(float64(a) + (float64(b)-float64(a))*u)) }
			return color.NRGBA{lerp(s[i-1].c.R, s[i].c.R), lerp(s[i-1].c.G, s[i].c.G), lerp(s[i-1].c.B, s[i].c.B), lerp(s[i-1].c.A, s[i].c.A)}
		}
	}
	return s[len(s)-1].c
}

func (r *renderer) resolvePaint(raw string, style computedStyle, bounds box, ctx renderContext) paintSampler {
	s := strings.TrimSpace(raw)
	if s == "" || strings.EqualFold(s, "none") {
		return nil
	}
	current, _ := parseColor(style["color"], color.NRGBA{0, 0, 0, 255})
	if c, ok := parseColor(s, current); ok {
		return solidPaint{c}
	}
	id, fallback := paintURL(s)
	if id == "" {
		return nil
	}
	n := r.doc.ids[id]
	if n == nil {
		if c, ok := parseColor(fallback, current); ok {
			return solidPaint{c}
		}
		return nil
	}
	switch localName(n.start.Name.Local) {
	case "linearGradient", "radialGradient":
		return r.gradient(n, bounds, ctx, current)
	case "pattern":
		return r.pattern(n, bounds, ctx)
	}
	return nil
}
func paintURL(s string) (string, string) {
	i := strings.Index(strings.ToLower(s), "url(")
	if i < 0 {
		return "", ""
	}
	j := strings.IndexByte(s[i+4:], ')')
	if j < 0 {
		return "", ""
	}
	inner := strings.Trim(strings.TrimSpace(s[i+4:i+4+j]), "\"'")
	return strings.TrimPrefix(inner, "#"), strings.TrimSpace(s[i+4+j+1:])
}

func (r *renderer) gradient(n *xmlNode, bounds box, ctx renderContext, current color.NRGBA) paintSampler {
	chain := gradientChain(r.doc, n, map[*xmlNode]bool{})
	attrs := map[string]string{}
	var stops []colorStop
	for i := 0; i < len(chain); i++ {
		for k, v := range attrMap(chain[i]) {
			if _, ok := attrs[k]; !ok {
				attrs[k] = v
			}
		}
		if len(stops) == 0 {
			stops = r.gradientStops(chain[i], current)
		}
	}
	if len(stops) == 0 {
		stops = []colorStop{{0, color.NRGBA{0, 0, 0, 255}}, {1, color.NRGBA{0, 0, 0, 255}}}
	}
	kind := localName(n.start.Name.Local)
	units := attrs["gradientUnits"]
	object := units != "userSpaceOnUse"
	base := func(axis lengthAxis) float64 {
		if object {
			return 1
		}
		switch axis {
		case lengthX:
			return ctx.viewport.w
		case lengthY:
			return ctx.viewport.h
		default:
			return math.Hypot(ctx.viewport.w, ctx.viewport.h) / math.Sqrt2
		case lengthFont:
			return parseNumber(ctx.style["font-size"], 16)
		}
	}
	val := func(key, def string, axis lengthAxis) float64 {
		raw := attrs[key]
		if raw == "" {
			raw = def
		}
		v, _ := resolveLength(raw, base, axis)
		return v
	}
	gp := gradientPaint{kind: "linear", stops: stops, spread: attrs["spreadMethod"]}
	if kind == "linearGradient" {
		gp.v = [6]float64{val("x1", "0%", lengthX), val("y1", "0%", lengthY), val("x2", "100%", lengthX), val("y2", "0%", lengthY)}
	} else {
		gp.kind = "radial"
		cx := val("cx", "50%", lengthX)
		cy := val("cy", "50%", lengthY)
		rad := val("r", "50%", lengthOther)
		fx := cx
		fy := cy
		if attrs["fx"] != "" {
			fx = val("fx", attrs["cx"], lengthX)
		}
		if attrs["fy"] != "" {
			fy = val("fy", attrs["cy"], lengthY)
		}
		gp.v = [6]float64{cx, cy, rad, fx, fy, val("fr", "0", lengthOther)}
	}
	m := ctx.transform
	if object {
		m = m.Mul(geom.Translate(bounds.x, bounds.y)).Mul(geom.Scale(bounds.w, bounds.h))
	}
	m = m.Mul(parseTransform(attrs["gradientTransform"]))
	inv, ok := inverse(m)
	if !ok {
		return nil
	}
	gp.inv = inv
	return gp
}

func gradientChain(d *Document, n *xmlNode, seen map[*xmlNode]bool) []*xmlNode {
	if n == nil || seen[n] {
		return nil
	}
	seen[n] = true
	out := []*xmlNode{n}
	a := attrMap(n)
	href := a["href"]
	if href != "" {
		if base := d.ids[strings.TrimPrefix(strings.TrimSpace(href), "#")]; base != nil {
			out = append(out, gradientChain(d, base, seen)...)
		}
	}
	return out
}
func (r *renderer) gradientStops(n *xmlNode, current color.NRGBA) []colorStop {
	var out []colorStop
	last := 0.0
	for _, ch := range n.children {
		if ch.elem == nil || localName(ch.elem.start.Name.Local) != "stop" {
			continue
		}
		st := r.doc.styleFor(ch.elem, defaultStyle())
		a := attrMap(ch.elem)
		off := parseNumber(a["offset"], 0)
		off = math.Max(last, clamp01(off))
		last = off
		c, ok := parseColor(st["stop-color"], current)
		if !ok {
			c = current
		}
		c.A = uint8(float64(c.A) * clamp01(parseNumber(st["stop-opacity"], 1)))
		out = append(out, colorStop{off, c})
	}
	if len(out) == 0 {
		return nil
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].offset < out[j].offset })
	return out
}

type patternPaint struct {
	tile       *renderer
	inv        geom.Matrix
	x, y, w, h float64
}

func (p patternPaint) at(x, y float64) color.NRGBA {
	if p.tile == nil || p.w <= 0 || p.h <= 0 {
		return color.NRGBA{}
	}
	q := p.inv.Transform(geom.Pt(x, y))
	u := math.Mod(q.X-p.x, p.w)
	v := math.Mod(q.Y-p.y, p.h)
	if u < 0 {
		u += p.w
	}
	if v < 0 {
		v += p.h
	}
	ix := p.tile.image.Bounds().Min.X + int(u/p.w*float64(p.tile.image.Bounds().Dx()))
	iy := p.tile.image.Bounds().Min.Y + int(v/p.h*float64(p.tile.image.Bounds().Dy()))
	if !imagePointIn(ix, iy, p.tile.image.Bounds()) {
		return color.NRGBA{}
	}
	i := p.tile.image.PixOffset(ix, iy)
	return color.NRGBA{p.tile.image.Pix[i], p.tile.image.Pix[i+1], p.tile.image.Pix[i+2], p.tile.image.Pix[i+3]}
}
func (r *renderer) pattern(n *xmlNode, bounds box, ctx renderContext) paintSampler {
	a := attrMap(n)
	object := a["patternUnits"] != "userSpaceOnUse"
	base := func(axis lengthAxis) float64 {
		if object {
			return 1
		}
		if axis == lengthX {
			return ctx.viewport.w
		}
		return ctx.viewport.h
	}
	x, _ := resolveLength(a["x"], base, lengthX)
	y, _ := resolveLength(a["y"], base, lengthY)
	w, _ := resolveLength(a["width"], base, lengthX)
	h, _ := resolveLength(a["height"], base, lengthY)
	if object {
		x = bounds.x + x*bounds.w
		y = bounds.y + y*bounds.h
		w *= bounds.w
		h *= bounds.h
	}
	if w <= 0 || h <= 0 {
		return nil
	}
	pw := maxInt(1, minInt(1024, int(math.Ceil(w*matrixScale(ctx.transform)))))
	ph := maxInt(1, minInt(1024, int(math.Ceil(h*matrixScale(ctx.transform)))))
	tile := newRenderer(r.doc, image.Rect(0, 0, pw, ph))
	vb, has := parseBox(a["viewBox"])
	m := geom.Scale(float64(pw)/w, float64(ph)/h).Mul(geom.Translate(-x, -y))
	if a["patternContentUnits"] == "objectBoundingBox" {
		m = m.Mul(geom.Translate(bounds.x, bounds.y)).Mul(geom.Scale(bounds.w, bounds.h))
	}
	if has {
		m = viewportTransform(vb, box{w: float64(pw), h: float64(ph)}, parseAspectRatio(a["preserveAspectRatio"]))
	}
	pc := ctx
	pc.transform = m
	pc.clip = fullMask(tile.image.Bounds())
	pc.viewport = box{x, y, w, h}
	for _, ch := range n.children {
		if ch.elem != nil {
			tile.renderElement(ch.elem, pc, true)
		}
	}
	dev := ctx.transform.Mul(parseTransform(a["patternTransform"]))
	inv, ok := inverse(dev)
	if !ok {
		return nil
	}
	return patternPaint{tile, inv, x, y, w, h}
}

func imagePointIn(x, y int, b image.Rectangle) bool {
	return x >= b.Min.X && x < b.Max.X && y >= b.Min.Y && y < b.Max.Y
}
