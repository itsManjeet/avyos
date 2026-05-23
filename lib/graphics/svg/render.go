package svg

import (
	"encoding/base64"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/gif"
	"image/jpeg"
	"image/png"
	"math"
	"net/url"
	"strings"

	gcolor "avyos.dev/lib/graphics/color"
	"avyos.dev/lib/graphics/font/bitmap"
	"avyos.dev/lib/graphics/geom"
)

type renderer struct {
	doc    *Document
	image  *image.RGBA
	active map[*xmlNode]int
}
type renderContext struct {
	transform geom.Matrix
	viewport  box
	style     computedStyle
	clip      *alphaMask
}

func newRenderer(d *Document, b image.Rectangle) *renderer {
	return &renderer{d, image.NewRGBA(b), make(map[*xmlNode]int)}
}

func (r *renderer) renderElement(n *xmlNode, ctx renderContext, referenced bool) {
	if n == nil || r.active[n] > 32 {
		return
	}
	style := r.doc.styleFor(n, ctx.style)
	if style["display"] == "none" || style["visibility"] == "hidden" || style["visibility"] == "collapse" {
		return
	}
	name := localName(n.start.Name.Local)
	if !referenced && (name == "defs" || name == "symbol" || name == "marker" || name == "pattern" || name == "mask" || name == "clipPath" || name == "linearGradient" || name == "radialGradient" || name == "filter" || name == "style" || name == "metadata" || name == "title" || name == "desc") {
		return
	}
	ctx.style = style
	ctx.transform = ctx.transform.Mul(parseTransform(style["transform"]))
	opacity := clamp01(parseNumber(style["opacity"], 1))
	needsLayer := opacity < .999999 || style["mask"] != "" || style["filter"] != ""
	if cp := style["clip-path"]; cp != "" && cp != "none" {
		ctx.clip = intersectMasks(ctx.clip, r.referenceMask(cp, n, ctx, false))
	}
	if needsLayer {
		layer := newRenderer(r.doc, r.image.Bounds())
		layer.active = r.active
		layerContext := ctx
		layerContext.clip = fullMask(r.image.Bounds())
		layer.renderCore(n, layerContext, referenced)
		if f := style["filter"]; f != "" && f != "none" {
			layer.image = r.applyFilter(layer.image, f, ctx)
		}
		mask := ctx.clip
		if m := style["mask"]; m != "" && m != "none" {
			mask = intersectMasks(mask, r.referenceMask(m, n, ctx, true))
		}
		composite(r.image, layer.image, mask, opacity)
		return
	}
	r.renderCore(n, ctx, referenced)
}

func (r *renderer) renderCore(n *xmlNode, ctx renderContext, referenced bool) {
	name := localName(n.start.Name.Local)
	r.active[n]++
	defer func() { r.active[n]-- }()
	switch name {
	case "svg":
		if n != r.doc.root {
			r.renderViewport(n, ctx)
		} else {
			r.renderChildren(n, ctx)
		}
	case "g", "a", "switch":
		r.renderChildren(n, ctx)
	case "defs", "symbol":
		if referenced {
			r.renderChildren(n, ctx)
		}
	case "use":
		r.renderUse(n, ctx)
	case "path", "rect", "circle", "ellipse", "line", "polyline", "polygon":
		p := r.elementPath(n, ctx)
		r.drawPath(p, ctx)
		r.drawMarkers(n, p, ctx)
	case "image":
		r.renderImage(n, ctx)
	case "text", "textPath", "tspan":
		r.renderText(n, ctx)
	case "foreignObject": // SVG cannot execute or rasterize foreign markup safely.
	default:
		if referenced {
			r.renderChildren(n, ctx)
		}
	}
}

func (r *renderer) renderChildren(n *xmlNode, ctx renderContext) {
	for _, ch := range n.children {
		if ch.elem != nil {
			r.renderElement(ch.elem, ctx, false)
		}
	}
}
func (r *renderer) base(ctx renderContext) lengthBase {
	return func(axis lengthAxis) float64 {
		switch axis {
		case lengthX:
			return ctx.viewport.w
		case lengthY:
			return ctx.viewport.h
		case lengthOther:
			return math.Hypot(ctx.viewport.w, ctx.viewport.h) / math.Sqrt2
		case lengthFont:
			return parseNumber(ctx.style["font-size"], 16)
		}
		return 0
	}
}
func (r *renderer) length(a map[string]string, key string, ctx renderContext, axis lengthAxis, def float64) float64 {
	if v, ok := resolveLength(a[key], r.base(ctx), axis); ok {
		return v
	}
	return def
}

func (r *renderer) renderViewport(n *xmlNode, ctx renderContext) {
	a := attrMap(n)
	x := r.length(a, "x", ctx, lengthX, 0)
	y := r.length(a, "y", ctx, lengthY, 0)
	w := r.length(a, "width", ctx, lengthX, ctx.viewport.w)
	h := r.length(a, "height", ctx, lengthY, ctx.viewport.h)
	if w <= 0 || h <= 0 {
		return
	}
	vb, ok := parseBox(a["viewBox"])
	if !ok {
		vb = box{w: w, h: h}
	}
	ctx.transform = ctx.transform.Mul(viewportTransform(vb, box{x: x, y: y, w: w, h: h}, parseAspectRatio(a["preserveAspectRatio"])))
	ctx.viewport = vb
	r.renderChildren(n, ctx)
}

func (r *renderer) elementPath(n *xmlNode, ctx renderContext) shapePath {
	a := attrMap(n)
	name := localName(n.start.Name.Local)
	x := func(k string, def float64) float64 { return r.length(a, k, ctx, lengthX, def) }
	y := func(k string, def float64) float64 { return r.length(a, k, ctx, lengthY, def) }
	other := func(k string, def float64) float64 { return r.length(a, k, ctx, lengthOther, def) }
	var p shapePath
	switch name {
	case "path":
		p = parsePathData(a["d"])
	case "rect":
		xx, yy, w, h := x("x", 0), y("y", 0), x("width", 0), y("height", 0)
		if w <= 0 || h <= 0 {
			return p
		}
		rx := x("rx", -1)
		ry := y("ry", -1)
		if rx < 0 {
			rx = ry
		}
		if ry < 0 {
			ry = rx
		}
		if rx <= 0 || ry <= 0 {
			p.subs = []subpath{{points: []geom.Point{geom.Pt(xx, yy), geom.Pt(xx+w, yy), geom.Pt(xx+w, yy+h), geom.Pt(xx, yy+h)}, closed: true}}
		} else {
			rx = math.Min(rx, w/2)
			ry = math.Min(ry, h/2)
			p = roundedRectPath(xx, yy, w, h, rx, ry)
		}
	case "circle":
		cx, cy, rad := x("cx", 0), y("cy", 0), other("r", 0)
		if rad > 0 {
			p = ellipsePath(cx, cy, rad, rad)
		}
	case "ellipse":
		cx, cy, rx, ry := x("cx", 0), y("cy", 0), x("rx", 0), y("ry", 0)
		if rx > 0 && ry > 0 {
			p = ellipsePath(cx, cy, rx, ry)
		}
	case "line":
		p.subs = []subpath{{points: []geom.Point{geom.Pt(x("x1", 0), y("y1", 0)), geom.Pt(x("x2", 0), y("y2", 0))}}}
	case "polyline", "polygon":
		v := scanNumbers(a["points"])
		sp := subpath{closed: name == "polygon"}
		for i := 0; i+1 < len(v); i += 2 {
			sp.points = append(sp.points, geom.Pt(v[i], v[i+1]))
		}
		if len(sp.points) > 0 {
			p.subs = []subpath{sp}
		}
	}
	return p
}

func roundedRectPath(x, y, w, h, rx, ry float64) shapePath {
	k := .5522847498307936
	sp := subpath{closed: true}
	sp.points = []geom.Point{geom.Pt(x+rx, y), geom.Pt(x+w-rx, y)}
	flattenCubic(&sp.points, sp.points[len(sp.points)-1], geom.Pt(x+w-rx+k*rx, y), geom.Pt(x+w, y+ry-k*ry), geom.Pt(x+w, y+ry))
	sp.points = append(sp.points, geom.Pt(x+w, y+h-ry))
	flattenCubic(&sp.points, sp.points[len(sp.points)-1], geom.Pt(x+w, y+h-ry+k*ry), geom.Pt(x+w-rx+k*rx, y+h), geom.Pt(x+w-rx, y+h))
	sp.points = append(sp.points, geom.Pt(x+rx, y+h))
	flattenCubic(&sp.points, sp.points[len(sp.points)-1], geom.Pt(x+rx-k*rx, y+h), geom.Pt(x, y+h-ry+k*ry), geom.Pt(x, y+h-ry))
	sp.points = append(sp.points, geom.Pt(x, y+ry))
	flattenCubic(&sp.points, sp.points[len(sp.points)-1], geom.Pt(x, y+ry-k*ry), geom.Pt(x+rx-k*rx, y), geom.Pt(x+rx, y))
	return shapePath{subs: []subpath{sp}}
}
func ellipsePath(cx, cy, rx, ry float64) shapePath {
	n := maxInt(16, minInt(256, int(math.Ceil(math.Max(rx, ry)*math.Pi/2))))
	sp := subpath{closed: true, points: make([]geom.Point, n)}
	for i := range n {
		a := 2 * math.Pi * float64(i) / float64(n)
		sp.points[i] = geom.Pt(cx+rx*math.Cos(a), cy+ry*math.Sin(a))
	}
	return shapePath{subs: []subpath{sp}}
}

func (r *renderer) drawPath(local shapePath, ctx renderContext) {
	if len(local.subs) == 0 {
		return
	}
	device := local.transformed(ctx.transform)
	bounds := local.bounds()
	fill := r.resolvePaint(ctx.style["fill"], ctx.style, bounds, ctx)
	stroke := r.resolvePaint(ctx.style["stroke"], ctx.style, bounds, ctx)
	fillOp := clamp01(parseNumber(ctx.style["fill-opacity"], 1))
	strokeOp := clamp01(parseNumber(ctx.style["stroke-opacity"], 1))
	order := ctx.style["paint-order"]
	drawFill := func() {
		if fill != nil {
			paintMask(r.image, rasterFill(device, r.image.Bounds(), ctx.style["fill-rule"]), ctx.clip, fill, fillOp)
		}
	}
	drawStroke := func() {
		if stroke == nil {
			return
		}
		w, _ := resolveLength(ctx.style["stroke-width"], r.base(ctx), lengthOther)
		if ctx.style["vector-effect"] != "non-scaling-stroke" {
			w *= matrixScale(ctx.transform)
		}
		dash := scanNumbers(ctx.style["stroke-dasharray"])
		scale := matrixScale(ctx.transform)
		for i := range dash {
			dash[i] *= scale
		}
		off := parseNumber(ctx.style["stroke-dashoffset"], 0) * scale
		opts := strokeOptions{w, ctx.style["stroke-linecap"], ctx.style["stroke-linejoin"], parseNumber(ctx.style["stroke-miterlimit"], 4), dash, off}
		paintMask(r.image, rasterStroke(device, r.image.Bounds(), opts), ctx.clip, stroke, strokeOp)
	}
	if strings.HasPrefix(strings.TrimSpace(order), "stroke") {
		drawStroke()
		drawFill()
	} else {
		drawFill()
		drawStroke()
	}
}

func (r *renderer) renderUse(n *xmlNode, ctx renderContext) {
	a := attrMap(n)
	href := a["href"]
	target := r.doc.ids[strings.TrimPrefix(strings.TrimSpace(href), "#")]
	if target == nil {
		return
	}
	x := r.length(a, "x", ctx, lengthX, 0)
	y := r.length(a, "y", ctx, lengthY, 0)
	ctx.transform = ctx.transform.Mul(geom.Translate(x, y))
	if localName(target.start.Name.Local) == "symbol" {
		ta := attrMap(target)
		w := r.length(a, "width", ctx, lengthX, ctx.viewport.w)
		h := r.length(a, "height", ctx, lengthY, ctx.viewport.h)
		vb, ok := parseBox(ta["viewBox"])
		if ok && w > 0 && h > 0 {
			ctx.transform = ctx.transform.Mul(viewportTransform(vb, box{w: w, h: h}, parseAspectRatio(ta["preserveAspectRatio"])))
			ctx.viewport = vb
		}
	}
	r.renderElement(target, ctx, true)
}

func (r *renderer) referenceMask(raw string, target *xmlNode, ctx renderContext, luminance bool) *alphaMask {
	id, _ := paintURL(raw)
	if id == "" {
		id = strings.TrimPrefix(strings.TrimSpace(raw), "#")
	}
	n := r.doc.ids[id]
	if n == nil {
		return fullMask(r.image.Bounds())
	}
	tmp := newRenderer(r.doc, r.image.Bounds())
	a := attrMap(n)
	mc := ctx
	mc.style = r.doc.styleFor(n, defaultStyle())
	mc.clip = fullMask(r.image.Bounds())
	if (localName(n.start.Name.Local) == "clipPath" && a["clipPathUnits"] == "objectBoundingBox") || (localName(n.start.Name.Local) == "mask" && a["maskContentUnits"] == "objectBoundingBox") {
		b := r.nodeBounds(target, ctx)
		mc.transform = ctx.transform.Mul(geom.Translate(b.x, b.y)).Mul(geom.Scale(b.w, b.h))
	}
	for _, ch := range n.children {
		if ch.elem != nil {
			tmp.renderElement(ch.elem, mc, true)
		}
	}
	m := newMask(r.image.Bounds())
	for y := m.bounds.Min.Y; y < m.bounds.Max.Y; y++ {
		for x := m.bounds.Min.X; x < m.bounds.Max.X; x++ {
			i := tmp.image.PixOffset(x, y)
			a := tmp.image.Pix[i+3]
			if luminance {
				lum := (uint32(tmp.image.Pix[i])*54 + uint32(tmp.image.Pix[i+1])*183 + uint32(tmp.image.Pix[i+2])*19) / 256
				a = uint8(lum * uint32(a) / 255)
			}
			m.pix[(y-m.bounds.Min.Y)*m.bounds.Dx()+x-m.bounds.Min.X] = a
		}
	}
	return m
}
func (r *renderer) nodeBounds(n *xmlNode, ctx renderContext) box {
	p := r.elementPath(n, ctx)
	if len(p.subs) > 0 {
		return p.bounds()
	}
	return box{ctx.viewport.x, ctx.viewport.y, ctx.viewport.w, ctx.viewport.h}
}

func (r *renderer) renderImage(n *xmlNode, ctx renderContext) {
	a := attrMap(n)
	href := a["href"]
	img, err := decodeDataImage(href)
	if err != nil {
		return
	}
	x, y := r.length(a, "x", ctx, lengthX, 0), r.length(a, "y", ctx, lengthY, 0)
	w, h := r.length(a, "width", ctx, lengthX, float64(img.Bounds().Dx())), r.length(a, "height", ctx, lengthY, float64(img.Bounds().Dy()))
	if w <= 0 || h <= 0 {
		return
	}
	src := box{w: float64(img.Bounds().Dx()), h: float64(img.Bounds().Dy())}
	m := ctx.transform.Mul(viewportTransform(src, box{x: x, y: y, w: w, h: h}, parseAspectRatio(a["preserveAspectRatio"])))
	inv, ok := inverse(m)
	if !ok {
		return
	}
	db := r.image.Bounds()
	for py := db.Min.Y; py < db.Max.Y; py++ {
		for px := db.Min.X; px < db.Max.X; px++ {
			if ctx.clip.at(px, py) == 0 {
				continue
			}
			q := inv.Transform(geom.Pt(float64(px)+.5, float64(py)+.5))
			sx, sy := int(math.Floor(q.X)), int(math.Floor(q.Y))
			if !image.Pt(sx, sy).In(img.Bounds()) {
				continue
			}
			c := color.NRGBAModel.Convert(img.At(sx+img.Bounds().Min.X, sy+img.Bounds().Min.Y)).(color.NRGBA)
			c.A = uint8(uint32(c.A) * uint32(ctx.clip.at(px, py)) / 255)
			blendPixel(r.image, px, py, c)
		}
	}
}
func decodeDataImage(href string) (image.Image, error) {
	if !strings.HasPrefix(strings.ToLower(strings.TrimSpace(href)), "data:") {
		return nil, fmt.Errorf("svg: external image references are not available without a base URI")
	}
	i := strings.IndexByte(href, ',')
	if i < 0 {
		return nil, fmt.Errorf("svg: malformed data URI")
	}
	meta, data := href[5:i], href[i+1:]
	var raw []byte
	var err error
	if strings.HasSuffix(strings.ToLower(meta), ";base64") {
		raw, err = base64.StdEncoding.DecodeString(strings.Map(func(r rune) rune {
			if r == '\n' || r == '\r' || r == '\t' || r == ' ' {
				return -1
			}
			return r
		}, data))
	} else {
		var s string
		s, err = url.PathUnescape(data)
		raw = []byte(s)
	}
	if err != nil {
		return nil, err
	}
	rd := dataReader(string(raw))
	switch {
	case strings.Contains(meta, "png"):
		return png.Decode(rd)
	case strings.Contains(meta, "jpeg") || strings.Contains(meta, "jpg"):
		return jpeg.Decode(rd)
	case strings.Contains(meta, "gif"):
		return gif.Decode(rd)
	default:
		img, _, e := image.Decode(rd)
		return img, e
	}
}

func (r *renderer) renderText(n *xmlNode, ctx renderContext) {
	a := attrMap(n)
	text := strings.Join(strings.Fields(nodeText(n)), " ")
	if text == "" {
		return
	}
	size := r.length(map[string]string{"v": ctx.style["font-size"]}, "v", ctx, lengthFont, 16)
	if size <= 0 {
		return
	}
	x, y := r.length(a, "x", ctx, lengthX, 0), r.length(a, "y", ctx, lengthY, 0)
	x += r.length(a, "dx", ctx, lengthX, 0)
	y += r.length(a, "dy", ctx, lengthY, 0)
	face := bitmap.New()
	advance := face.RuneAdvance('M', size)
	width := advance * float64(len([]rune(text)))
	switch ctx.style["text-anchor"] {
	case "middle":
		x -= width / 2
	case "end":
		x -= width
	}
	tmp := image.NewRGBA(image.Rect(0, 0, maxInt(1, int(math.Ceil(width))), maxInt(1, int(math.Ceil(size*1.25)))))
	c, ok := parseColor(ctx.style["fill"], color.NRGBA{0, 0, 0, 255})
	if !ok {
		return
	}
	gc := gcolor.FromRGBA8(c.R, c.G, c.B, uint8(float64(c.A)*clamp01(parseNumber(ctx.style["fill-opacity"], 1))))
	xx := 0.0
	for _, ch := range text {
		face.DrawRune(ch, size, tmp, xx, 0, gc)
		xx += face.RuneAdvance(ch, size)
	}
	m := ctx.transform.Mul(geom.Translate(x, y-size))
	inv, ok := inverse(m)
	if !ok {
		return
	}
	for py := r.image.Bounds().Min.Y; py < r.image.Bounds().Max.Y; py++ {
		for px := r.image.Bounds().Min.X; px < r.image.Bounds().Max.X; px++ {
			q := inv.Transform(geom.Pt(float64(px)+.5, float64(py)+.5))
			sx, sy := int(q.X), int(q.Y)
			if !image.Pt(sx, sy).In(tmp.Bounds()) {
				continue
			}
			i := tmp.PixOffset(sx, sy)
			cc := color.NRGBA{tmp.Pix[i], tmp.Pix[i+1], tmp.Pix[i+2], uint8(uint32(tmp.Pix[i+3]) * uint32(ctx.clip.at(px, py)) / 255)}
			blendPixel(r.image, px, py, cc)
		}
	}
}

func (r *renderer) drawMarkers(n *xmlNode, p shapePath, ctx renderContext) {
	startRef, midRef, endRef := ctx.style["marker-start"], ctx.style["marker-mid"], ctx.style["marker-end"]
	if shorthand := ctx.style["marker"]; shorthand != "" && shorthand != "none" {
		if startRef == "" {
			startRef = shorthand
		}
		if midRef == "" {
			midRef = shorthand
		}
		if endRef == "" {
			endRef = shorthand
		}
	}
	for _, sp := range p.subs {
		if len(sp.points) < 2 {
			continue
		}
		for i, q := range sp.points {
			ref := midRef
			if i == 0 {
				ref = startRef
			} else if i == len(sp.points)-1 {
				ref = endRef
			}
			if ref == "" || ref == "none" {
				continue
			}
			prev, next := sp.points[maxInt(0, i-1)], sp.points[minInt(len(sp.points)-1, i+1)]
			angle := math.Atan2(next.Y-prev.Y, next.X-prev.X)
			r.renderMarker(ref, q, angle, ctx)
		}
	}
}

func (r *renderer) renderMarker(raw string, at geom.Point, angle float64, ctx renderContext) {
	id, _ := paintURL(raw)
	if id == "" {
		id = strings.TrimPrefix(strings.TrimSpace(raw), "#")
	}
	n := r.doc.ids[id]
	if n == nil || localName(n.start.Name.Local) != "marker" {
		return
	}
	a := attrMap(n)
	mw := r.length(a, "markerWidth", ctx, lengthX, 3)
	mh := r.length(a, "markerHeight", ctx, lengthY, 3)
	refX := r.length(a, "refX", ctx, lengthX, 0)
	refY := r.length(a, "refY", ctx, lengthY, 0)
	if mw <= 0 || mh <= 0 {
		return
	}
	if orient := strings.TrimSpace(a["orient"]); orient != "" && orient != "auto" && orient != "auto-start-reverse" {
		angle = parseAngle(orient)
	}
	unit := 1.0
	if a["markerUnits"] != "userSpaceOnUse" {
		unit, _ = resolveLength(ctx.style["stroke-width"], r.base(ctx), lengthOther)
	}
	vb, ok := parseBox(a["viewBox"])
	if !ok {
		vb = box{w: mw, h: mh}
	}
	vm := viewportTransform(vb, box{w: mw, h: mh}, parseAspectRatio(a["preserveAspectRatio"]))
	ref := vm.Transform(geom.Pt(refX, refY))
	m := ctx.transform.Mul(geom.Translate(at.X, at.Y)).Mul(geom.Rotate(angle)).Mul(geom.Scale(unit, unit)).Mul(geom.Translate(-ref.X, -ref.Y)).Mul(vm)
	mc := ctx
	mc.transform = m
	mc.viewport = vb
	for _, ch := range n.children {
		if ch.elem != nil {
			r.renderElement(ch.elem, mc, true)
		}
	}
}

var _ draw.Image = (*image.RGBA)(nil)
