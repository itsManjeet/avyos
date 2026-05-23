package svg

import (
	"math"
	"strconv"
	"strings"
	"unicode"

	"avyos.dev/lib/graphics/geom"
)

type subpath struct {
	points []geom.Point
	closed bool
}
type shapePath struct{ subs []subpath }

func (p shapePath) transformed(m geom.Matrix) shapePath {
	o := shapePath{subs: make([]subpath, len(p.subs))}
	for i, s := range p.subs {
		o.subs[i].closed = s.closed
		o.subs[i].points = make([]geom.Point, len(s.points))
		for j, q := range s.points {
			o.subs[i].points[j] = m.Transform(q)
		}
	}
	return o
}
func (p shapePath) bounds() box {
	b := box{x: math.Inf(1), y: math.Inf(1)}
	maxX, maxY := math.Inf(-1), math.Inf(-1)
	for _, s := range p.subs {
		for _, q := range s.points {
			b.x = math.Min(b.x, q.X)
			b.y = math.Min(b.y, q.Y)
			maxX = math.Max(maxX, q.X)
			maxY = math.Max(maxY, q.Y)
		}
	}
	if math.IsInf(b.x, 1) {
		return box{}
	}
	b.w = maxX - b.x
	b.h = maxY - b.y
	return b
}

type pathScanner struct {
	s   string
	i   int
	cmd byte
}

func (p *pathScanner) skip() {
	for p.i < len(p.s) && (unicode.IsSpace(rune(p.s[p.i])) || p.s[p.i] == ',') {
		p.i++
	}
}
func (p *pathScanner) more() bool { p.skip(); return p.i < len(p.s) }
func (p *pathScanner) number() (float64, bool) {
	p.skip()
	if p.i >= len(p.s) {
		return 0, false
	}
	start := p.i
	if p.s[p.i] == '+' || p.s[p.i] == '-' {
		p.i++
	}
	digits := false
	for p.i < len(p.s) && p.s[p.i] >= '0' && p.s[p.i] <= '9' {
		p.i++
		digits = true
	}
	if p.i < len(p.s) && p.s[p.i] == '.' {
		p.i++
		for p.i < len(p.s) && p.s[p.i] >= '0' && p.s[p.i] <= '9' {
			p.i++
			digits = true
		}
	}
	if !digits {
		p.i = start
		return 0, false
	}
	if p.i < len(p.s) && (p.s[p.i] == 'e' || p.s[p.i] == 'E') {
		j := p.i
		p.i++
		if p.i < len(p.s) && (p.s[p.i] == '+' || p.s[p.i] == '-') {
			p.i++
		}
		k := p.i
		for p.i < len(p.s) && p.s[p.i] >= '0' && p.s[p.i] <= '9' {
			p.i++
		}
		if p.i == k {
			p.i = j
		}
	}
	v, e := strconv.ParseFloat(p.s[start:p.i], 64)
	return v, e == nil
}

func parsePathData(d string) shapePath {
	s := pathScanner{s: d}
	var out shapePath
	var cur, start, lastC, lastQ geom.Point
	haveC, haveQ := false, false
	newSub := func(q geom.Point) { out.subs = append(out.subs, subpath{points: []geom.Point{q}}); start = q }
	line := func(q geom.Point) {
		if len(out.subs) == 0 {
			newSub(cur)
		}
		sp := &out.subs[len(out.subs)-1]
		if len(sp.points) == 0 || distance(sp.points[len(sp.points)-1], q) > 1e-12 {
			sp.points = append(sp.points, q)
		}
		cur = q
	}
	for s.more() {
		if isPathLetter(s.s[s.i]) {
			s.cmd = s.s[s.i]
			s.i++
		} else if s.cmd == 0 {
			break
		}
		cmd := s.cmd
		commandStart := s.i
		rel := cmd >= 'a' && cmd <= 'z'
		up := cmd
		if rel {
			up -= 32
		}
		read := func() (float64, bool) { return s.number() }
		point := func() (geom.Point, bool) {
			x, ok := read()
			if !ok {
				return geom.Point{}, false
			}
			y, ok := read()
			if !ok {
				return geom.Point{}, false
			}
			q := geom.Pt(x, y)
			if rel {
				q = q.Add(cur)
			}
			return q, true
		}
		switch up {
		case 'M':
			q, ok := point()
			if !ok {
				return out
			}
			cur = q
			newSub(q)
			haveC = false
			haveQ = false
			if rel {
				s.cmd = 'l'
			} else {
				s.cmd = 'L'
			}
			for {
				save := s.i
				q, ok = point()
				if !ok {
					s.i = save
					break
				}
				line(q)
			}
		case 'L':
			for {
				save := s.i
				q, ok := point()
				if !ok {
					s.i = save
					break
				}
				line(q)
			}
			haveC = false
			haveQ = false
		case 'H':
			for {
				save := s.i
				x, ok := read()
				if !ok {
					s.i = save
					break
				}
				if rel {
					x += cur.X
				}
				line(geom.Pt(x, cur.Y))
			}
			haveC = false
			haveQ = false
		case 'V':
			for {
				save := s.i
				y, ok := read()
				if !ok {
					s.i = save
					break
				}
				if rel {
					y += cur.Y
				}
				line(geom.Pt(cur.X, y))
			}
			haveC = false
			haveQ = false
		case 'C':
			for {
				save := s.i
				c1, ok := point()
				if !ok {
					s.i = save
					break
				}
				c2, ok := point()
				if !ok {
					s.i = save
					break
				}
				q, ok := point()
				if !ok {
					s.i = save
					break
				}
				flattenCubic(&out.subs[len(out.subs)-1].points, cur, c1, c2, q)
				cur = q
				lastC = c2
				haveC = true
				haveQ = false
			}
		case 'S':
			for {
				save := s.i
				c2, ok := point()
				if !ok {
					s.i = save
					break
				}
				q, ok := point()
				if !ok {
					s.i = save
					break
				}
				c1 := cur
				if haveC {
					c1 = cur.Scale(2).Sub(lastC)
				}
				flattenCubic(&out.subs[len(out.subs)-1].points, cur, c1, c2, q)
				cur = q
				lastC = c2
				haveC = true
				haveQ = false
			}
		case 'Q':
			for {
				save := s.i
				c, ok := point()
				if !ok {
					s.i = save
					break
				}
				q, ok := point()
				if !ok {
					s.i = save
					break
				}
				flattenQuad(&out.subs[len(out.subs)-1].points, cur, c, q)
				cur = q
				lastQ = c
				haveQ = true
				haveC = false
			}
		case 'T':
			for {
				save := s.i
				q, ok := point()
				if !ok {
					s.i = save
					break
				}
				c := cur
				if haveQ {
					c = cur.Scale(2).Sub(lastQ)
				}
				flattenQuad(&out.subs[len(out.subs)-1].points, cur, c, q)
				cur = q
				lastQ = c
				haveQ = true
				haveC = false
			}
		case 'A':
			for {
				save := s.i
				rx, ok := read()
				if !ok {
					s.i = save
					break
				}
				ry, ok := read()
				if !ok {
					s.i = save
					break
				}
				rot, ok := read()
				if !ok {
					s.i = save
					break
				}
				large, ok := read()
				if !ok {
					s.i = save
					break
				}
				sweep, ok := read()
				if !ok {
					s.i = save
					break
				}
				q, ok := point()
				if !ok {
					s.i = save
					break
				}
				flattenArc(&out.subs[len(out.subs)-1].points, cur, q, math.Abs(rx), math.Abs(ry), rot*math.Pi/180, large != 0, sweep != 0)
				cur = q
			}
			haveC = false
			haveQ = false
		case 'Z':
			if len(out.subs) > 0 {
				sp := &out.subs[len(out.subs)-1]
				sp.closed = true
				cur = start
			}
			haveC = false
			haveQ = false
			s.cmd = 0
		default:
			return out
		}
		if up != 'Z' && s.i == commandStart && s.more() {
			// Invalid command arguments must not trap the parser in an infinite loop.
			s.i++
			s.cmd = 0
		}
	}
	return out
}

func isPathLetter(c byte) bool         { return strings.ContainsRune("MmZzLlHhVvCcSsQqTtAa", rune(c)) }
func distance(a, b geom.Point) float64 { return math.Hypot(a.X-b.X, a.Y-b.Y) }

func flattenQuad(dst *[]geom.Point, p0, p1, p2 geom.Point) {
	steps := int(math.Ceil((distance(p0, p1) + distance(p1, p2)) / 4))
	if steps < 2 {
		steps = 2
	}
	if steps > 128 {
		steps = 128
	}
	for i := 1; i <= steps; i++ {
		t := float64(i) / float64(steps)
		u := 1 - t
		*dst = append(*dst, geom.Pt(u*u*p0.X+2*u*t*p1.X+t*t*p2.X, u*u*p0.Y+2*u*t*p1.Y+t*t*p2.Y))
	}
}
func flattenCubic(dst *[]geom.Point, p0, p1, p2, p3 geom.Point) {
	steps := int(math.Ceil((distance(p0, p1) + distance(p1, p2) + distance(p2, p3)) / 4))
	if steps < 3 {
		steps = 3
	}
	if steps > 256 {
		steps = 256
	}
	for i := 1; i <= steps; i++ {
		t := float64(i) / float64(steps)
		u := 1 - t
		*dst = append(*dst, geom.Pt(u*u*u*p0.X+3*u*u*t*p1.X+3*u*t*t*p2.X+t*t*t*p3.X, u*u*u*p0.Y+3*u*u*t*p1.Y+3*u*t*t*p2.Y+t*t*t*p3.Y))
	}
}

func flattenArc(dst *[]geom.Point, p0, p1 geom.Point, rx, ry, phi float64, large, sweep bool) {
	if rx == 0 || ry == 0 || distance(p0, p1) < 1e-12 {
		*dst = append(*dst, p1)
		return
	}
	cp, sp := math.Cos(phi), math.Sin(phi)
	dx := (p0.X - p1.X) / 2
	dy := (p0.Y - p1.Y) / 2
	xp := cp*dx + sp*dy
	yp := -sp*dx + cp*dy
	lam := xp*xp/(rx*rx) + yp*yp/(ry*ry)
	if lam > 1 {
		k := math.Sqrt(lam)
		rx *= k
		ry *= k
	}
	sign := 1.0
	if large == sweep {
		sign = -1
	}
	num := rx*rx*ry*ry - rx*rx*yp*yp - ry*ry*xp*xp
	den := rx*rx*yp*yp + ry*ry*xp*xp
	k := 0.0
	if den > 0 {
		k = sign * math.Sqrt(math.Max(0, num/den))
	}
	cxp := k * rx * yp / ry
	cyp := -k * ry * xp / rx
	cx := cp*cxp - sp*cyp + (p0.X+p1.X)/2
	cy := sp*cxp + cp*cyp + (p0.Y+p1.Y)/2
	angle := func(ux, uy, vx, vy float64) float64 { v := math.Atan2(ux*vy-uy*vx, ux*vx+uy*vy); return v }
	ux := (xp - cxp) / rx
	uy := (yp - cyp) / ry
	vx := (-xp - cxp) / rx
	vy := (-yp - cyp) / ry
	start := math.Atan2(uy, ux)
	delta := angle(ux, uy, vx, vy)
	if !sweep && delta > 0 {
		delta -= 2 * math.Pi
	}
	if sweep && delta < 0 {
		delta += 2 * math.Pi
	}
	steps := int(math.Ceil(math.Abs(delta) * math.Max(rx, ry) / 4))
	if steps < 4 {
		steps = 4
	}
	if steps > 512 {
		steps = 512
	}
	for i := 1; i <= steps; i++ {
		a := start + delta*float64(i)/float64(steps)
		x := rx * math.Cos(a)
		y := ry * math.Sin(a)
		*dst = append(*dst, geom.Pt(cx+cp*x-sp*y, cy+sp*x+cp*y))
	}
}

func parseTransform(s string) geom.Matrix {
	m := geom.Identity()
	for {
		s = strings.TrimSpace(s)
		if s == "" {
			break
		}
		i := strings.IndexByte(s, '(')
		if i < 0 {
			break
		}
		name := strings.ToLower(strings.TrimSpace(s[:i]))
		depth := 1
		j := i + 1
		for j < len(s) && depth > 0 {
			if s[j] == '(' {
				depth++
			}
			if s[j] == ')' {
				depth--
			}
			j++
		}
		if depth != 0 {
			break
		}
		v := scanNumbers(s[i+1 : j-1])
		t := geom.Identity()
		switch name {
		case "matrix":
			if len(v) >= 6 {
				t = geom.Matrix{v[0], v[1], v[2], v[3], v[4], v[5]}
			}
		case "translate":
			if len(v) >= 1 {
				y := 0.0
				if len(v) > 1 {
					y = v[1]
				}
				t = geom.Translate(v[0], y)
			}
		case "scale":
			if len(v) >= 1 {
				y := v[0]
				if len(v) > 1 {
					y = v[1]
				}
				t = geom.Scale(v[0], y)
			}
		case "rotate":
			if len(v) >= 1 {
				t = geom.Rotate(v[0] * math.Pi / 180)
				if len(v) >= 3 {
					t = geom.Translate(v[1], v[2]).Mul(t).Mul(geom.Translate(-v[1], -v[2]))
				}
			}
		case "skewx":
			if len(v) >= 1 {
				t = geom.Matrix{1, 0, math.Tan(v[0] * math.Pi / 180), 1, 0, 0}
			}
		case "skewy":
			if len(v) >= 1 {
				t = geom.Matrix{1, math.Tan(v[0] * math.Pi / 180), 0, 1, 0, 0}
			}
		}
		m = m.Mul(t)
		s = s[j:]
	}
	return m
}

func inverse(m geom.Matrix) (geom.Matrix, bool) {
	det := m[0]*m[3] - m[1]*m[2]
	if math.Abs(det) < 1e-14 {
		return geom.Matrix{}, false
	}
	return geom.Matrix{m[3] / det, -m[1] / det, -m[2] / det, m[0] / det, (m[2]*m[5] - m[3]*m[4]) / det, (m[1]*m[4] - m[0]*m[5]) / det}, true
}
func matrixScale(m geom.Matrix) float64 {
	return math.Sqrt((m[0]*m[0] + m[1]*m[1] + m[2]*m[2] + m[3]*m[3]) / 2)
}
