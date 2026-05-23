package svg

import (
	"bytes"
	"fmt"
	"image"
	"image/draw"
	"io"
	"math"
	"strings"

	"avyos.dev/lib/graphics/geom"
)

// Document is a parsed SVG document. A Document is immutable after parsing and
// can be rendered repeatedly, including concurrently.
type Document struct {
	root   *xmlNode
	ids    map[string]*xmlNode
	rules  []cssRule
	width  float64
	height float64
	view   box
	par    aspectRatio
}

type box struct{ x, y, w, h float64 }

// Parse reads and validates an SVG document. It builds the element and CSS
// indexes once so repeated renders do not need to parse XML again.
func Parse(r io.Reader) (*Document, error) {
	data, err := io.ReadAll(io.LimitReader(r, 128<<20))
	if err != nil {
		return nil, err
	}
	roots, err := parseXMLTree(data)
	if err != nil {
		return nil, fmt.Errorf("svg: parse XML: %w", err)
	}
	var root *xmlNode
	for _, child := range roots {
		if child.elem != nil {
			if localName(child.elem.start.Name.Local) != "svg" {
				return nil, fmt.Errorf("svg: root element is <%s>, want <svg>", child.elem.start.Name.Local)
			}
			root = child.elem
			break
		}
	}
	if root == nil {
		return nil, fmt.Errorf("svg: missing root <svg> element")
	}
	d := &Document{root: root, ids: make(map[string]*xmlNode)}
	d.index(root)
	d.collectStyles(root)

	attrs := attrMap(root)
	vb, hasView := parseBox(attrs["viewBox"])
	w, hasW := resolveLength(attrs["width"], func(lengthAxis) float64 { return 0 }, lengthX)
	h, hasH := resolveLength(attrs["height"], func(lengthAxis) float64 { return 0 }, lengthY)
	if hasView && (!hasW || w <= 0) {
		w, hasW = vb.w, true
	}
	if hasView && (!hasH || h <= 0) {
		h, hasH = vb.h, true
	}
	if !hasW && hasH {
		w, hasW = h, true
	}
	if !hasH && hasW {
		h, hasH = w, true
	}
	if !hasW || !hasH || w <= 0 || h <= 0 {
		return nil, fmt.Errorf("svg: width/height or a valid viewBox is required")
	}
	if !hasView {
		vb = box{w: w, h: h}
	}
	d.width, d.height, d.view = w, h, vb
	d.par = parseAspectRatio(attrs["preserveAspectRatio"])
	return d, nil
}

func (d *Document) index(n *xmlNode) {
	if id, ok := attrValue(n.start.Attr, "id"); ok && id != "" {
		d.ids[id] = n
	}
	for _, child := range n.children {
		if child.elem != nil {
			d.index(child.elem)
		}
	}
}

func (d *Document) collectStyles(n *xmlNode) {
	if localName(n.start.Name.Local) == "style" {
		d.rules = append(d.rules, parseStylesheet(nodeText(n))...)
	}
	for _, child := range n.children {
		if child.elem != nil {
			d.collectStyles(child.elem)
		}
	}
}

// Size returns the intrinsic CSS-pixel dimensions of the document.
func (d *Document) Size() (width, height float64) { return d.width, d.height }

// Render rasterizes the document into dst, scaling its viewport to dst.Bounds.
func (d *Document) Render(dst draw.Image) error { return d.render(dst, true) }

func (d *Document) render(dst draw.Image, fit bool) error {
	if dst == nil {
		return fmt.Errorf("svg: nil destination image")
	}
	b := dst.Bounds()
	if b.Empty() {
		return fmt.Errorf("svg: empty destination bounds")
	}
	targetW, targetH := d.width, d.height
	if fit {
		targetW, targetH = float64(b.Dx()), float64(b.Dy())
	}
	base := viewportTransform(d.view, box{w: targetW, h: targetH}, d.par)
	base = geom.Translate(float64(b.Min.X), float64(b.Min.Y)).Mul(base)
	r := newRenderer(d, b)
	rootStyle := defaultStyle()
	ctx := renderContext{
		transform: base,
		viewport:  box{w: d.view.w, h: d.view.h},
		style:     rootStyle,
		clip:      fullMask(b),
	}
	r.renderElement(d.root, ctx, false)
	draw.Draw(dst, b, r.image, b.Min, draw.Src)
	return nil
}

type aspectRatio struct {
	none   bool
	slice  bool
	ax, ay float64
}

func parseAspectRatio(s string) aspectRatio {
	p := aspectRatio{ax: .5, ay: .5}
	f := strings.Fields(strings.TrimSpace(s))
	if len(f) == 0 {
		return p
	}
	i := 0
	if f[0] == "defer" {
		i++
	}
	if i >= len(f) {
		return p
	}
	if f[i] == "none" {
		p.none = true
		return p
	}
	a := f[i]
	if strings.HasPrefix(a, "xMin") {
		p.ax = 0
	} else if strings.HasPrefix(a, "xMax") {
		p.ax = 1
	}
	if strings.HasSuffix(a, "YMin") {
		p.ay = 0
	} else if strings.HasSuffix(a, "YMax") {
		p.ay = 1
	}
	if i+1 < len(f) && f[i+1] == "slice" {
		p.slice = true
	}
	return p
}

func viewportTransform(src, dst box, par aspectRatio) geom.Matrix {
	if src.w <= 0 || src.h <= 0 {
		return geom.Identity()
	}
	sx, sy := dst.w/src.w, dst.h/src.h
	if par.none {
		return geom.Translate(dst.x-src.x*sx, dst.y-src.y*sy).Mul(geom.Scale(sx, sy))
	}
	s := math.Min(sx, sy)
	if par.slice {
		s = math.Max(sx, sy)
	}
	x := dst.x + (dst.w-src.w*s)*par.ax - src.x*s
	y := dst.y + (dst.h-src.h*s)*par.ay - src.y*s
	return geom.Translate(x, y).Mul(geom.Scale(s, s))
}

func parseBox(s string) (box, bool) {
	n := scanNumbers(s)
	if len(n) != 4 || n[2] <= 0 || n[3] <= 0 {
		return box{}, false
	}
	return box{n[0], n[1], n[2], n[3]}, true
}

func attrMap(n *xmlNode) map[string]string {
	m := make(map[string]string, len(n.start.Attr))
	for _, a := range n.start.Attr {
		m[localName(a.Name.Local)] = strings.TrimSpace(a.Value)
	}
	return m
}

func cloneRGBA(src *image.RGBA) *image.RGBA {
	dst := image.NewRGBA(src.Bounds())
	copy(dst.Pix, src.Pix)
	return dst
}

func dataReader(s string) *bytes.Reader { return bytes.NewReader([]byte(s)) }
