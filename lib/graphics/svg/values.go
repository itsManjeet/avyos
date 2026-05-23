package svg

import (
	"image/color"
	"math"
	"strconv"
	"strings"
	"unicode"
)

type lengthAxis uint8

const (
	lengthX lengthAxis = iota
	lengthY
	lengthOther
	lengthFont
)

type lengthBase func(lengthAxis) float64

func resolveLength(raw string, base lengthBase, axis lengthAxis) (float64, bool) {
	s := strings.TrimSpace(strings.ToLower(raw))
	if s == "" {
		return 0, false
	}
	unit := ""
	i := len(s)
	for i > 0 && (unicode.IsLetter(rune(s[i-1])) || s[i-1] == '%') {
		i--
	}
	unit, s = s[i:], strings.TrimSpace(s[:i])
	v, err := strconv.ParseFloat(s, 64)
	if err != nil || math.IsNaN(v) || math.IsInf(v, 0) {
		return 0, false
	}
	switch unit {
	case "", "px":
	case "%":
		v *= base(axis) / 100
	case "em":
		v *= base(lengthFont)
	case "ex", "ch":
		v *= base(lengthFont) * .5
	case "rem":
		v *= 16
	case "in":
		v *= 96
	case "cm":
		v *= 96 / 2.54
	case "mm":
		v *= 96 / 25.4
	case "q":
		v *= 96 / 101.6
	case "pt":
		v *= 96 / 72
	case "pc":
		v *= 16
	case "vw":
		v *= base(lengthX) / 100
	case "vh":
		v *= base(lengthY) / 100
	case "vmin":
		v *= math.Min(base(lengthX), base(lengthY)) / 100
	case "vmax":
		v *= math.Max(base(lengthX), base(lengthY)) / 100
	default:
		return 0, false
	}
	return v, true
}

func scanNumbers(s string) []float64 {
	var out []float64
	for i := 0; i < len(s); {
		for i < len(s) && (s[i] == ',' || unicode.IsSpace(rune(s[i]))) {
			i++
		}
		if i == len(s) {
			break
		}
		start := i
		if s[i] == '+' || s[i] == '-' {
			i++
		}
		digits := false
		for i < len(s) && s[i] >= '0' && s[i] <= '9' {
			i++
			digits = true
		}
		if i < len(s) && s[i] == '.' {
			i++
			for i < len(s) && s[i] >= '0' && s[i] <= '9' {
				i++
				digits = true
			}
		}
		if !digits {
			i = start + 1
			continue
		}
		if i < len(s) && (s[i] == 'e' || s[i] == 'E') {
			j := i + 1
			if j < len(s) && (s[j] == '+' || s[j] == '-') {
				j++
			}
			k := j
			for j < len(s) && s[j] >= '0' && s[j] <= '9' {
				j++
			}
			if j > k {
				i = j
			}
		}
		if v, err := strconv.ParseFloat(s[start:i], 64); err == nil {
			out = append(out, v)
		}
	}
	return out
}

func parseNumber(raw string, fallback float64) float64 {
	s := strings.TrimSpace(raw)
	if strings.HasSuffix(s, "%") {
		if v, err := strconv.ParseFloat(strings.TrimSpace(strings.TrimSuffix(s, "%")), 64); err == nil {
			return v / 100
		}
	}
	if v, err := strconv.ParseFloat(s, 64); err == nil {
		return v
	}
	return fallback
}

func parseColor(raw string, current color.NRGBA) (color.NRGBA, bool) {
	s := strings.ToLower(strings.TrimSpace(raw))
	if s == "currentcolor" {
		return current, true
	}
	if s == "transparent" {
		return color.NRGBA{}, true
	}
	if strings.HasPrefix(s, "#") {
		h := s[1:]
		var v uint64
		var err error
		v, err = strconv.ParseUint(h, 16, 32)
		if err != nil {
			return color.NRGBA{}, false
		}
		switch len(h) {
		case 3:
			return color.NRGBA{uint8((v>>8)&15) * 17, uint8((v>>4)&15) * 17, uint8(v&15) * 17, 255}, true
		case 4:
			return color.NRGBA{uint8((v>>12)&15) * 17, uint8((v>>8)&15) * 17, uint8((v>>4)&15) * 17, uint8(v&15) * 17}, true
		case 6:
			return color.NRGBA{uint8(v >> 16), uint8(v >> 8), uint8(v), 255}, true
		case 8:
			return color.NRGBA{uint8(v >> 24), uint8(v >> 16), uint8(v >> 8), uint8(v)}, true
		}
	}
	if i := strings.IndexByte(s, '('); i > 0 && strings.HasSuffix(s, ")") {
		fn := s[:i]
		args := strings.NewReplacer(",", " ", "/", " ").Replace(s[i+1 : len(s)-1])
		parts := strings.Fields(args)
		if fn == "rgb" || fn == "rgba" {
			if len(parts) < 3 {
				return color.NRGBA{}, false
			}
			vals := [4]float64{0, 0, 0, 1}
			for j := 0; j < len(parts) && j < 4; j++ {
				vals[j] = parseNumber(parts[j], 0)
				if j < 3 && !strings.HasSuffix(parts[j], "%") {
					vals[j] /= 255
				}
			}
			return nrgba(vals[0], vals[1], vals[2], vals[3]), true
		}
		if fn == "hsl" || fn == "hsla" {
			if len(parts) < 3 {
				return color.NRGBA{}, false
			}
			h := parseAngle(parts[0]) / (2 * math.Pi)
			sat, light := parseNumber(parts[1], 0), parseNumber(parts[2], 0)
			a := 1.0
			if len(parts) > 3 {
				a = parseNumber(parts[3], 1)
			}
			r, g, b := hslToRGB(h, sat, light)
			return nrgba(r, g, b, a), true
		}
	}
	if c, ok := namedColors[s]; ok {
		return c, true
	}
	return color.NRGBA{}, false
}

func nrgba(r, g, b, a float64) color.NRGBA {
	byteOf := func(v float64) uint8 { return uint8(math.Round(clamp01(v) * 255)) }
	return color.NRGBA{byteOf(r), byteOf(g), byteOf(b), byteOf(a)}
}

func hslToRGB(h, s, l float64) (float64, float64, float64) {
	h -= math.Floor(h)
	s = clamp01(s)
	l = clamp01(l)
	if s == 0 {
		return l, l, l
	}
	q := l * (1 + s)
	if l < .5 {
		q = l * (1 + s)
	} else {
		q = l + s - l*s
	}
	p := 2*l - q
	f := func(t float64) float64 {
		t -= math.Floor(t)
		if t < 1.0/6 {
			return p + (q-p)*6*t
		}
		if t < .5 {
			return q
		}
		if t < 2.0/3 {
			return p + (q-p)*(2.0/3-t)*6
		}
		return p
	}
	return f(h + 1.0/3), f(h), f(h - 1.0/3)
}

func parseAngle(s string) float64 {
	s = strings.TrimSpace(strings.ToLower(s))
	f := 1.0
	switch {
	case strings.HasSuffix(s, "deg"):
		f = math.Pi / 180
		s = strings.TrimSuffix(s, "deg")
	case strings.HasSuffix(s, "grad"):
		f = math.Pi / 200
		s = strings.TrimSuffix(s, "grad")
	case strings.HasSuffix(s, "turn"):
		f = 2 * math.Pi
		s = strings.TrimSuffix(s, "turn")
	case strings.HasSuffix(s, "rad"):
		s = strings.TrimSuffix(s, "rad")
	}
	v, _ := strconv.ParseFloat(strings.TrimSpace(s), 64)
	return v * f
}

var namedColors = map[string]color.NRGBA{
	"black": {0, 0, 0, 255}, "silver": {192, 192, 192, 255}, "gray": {128, 128, 128, 255}, "white": {255, 255, 255, 255},
	"maroon": {128, 0, 0, 255}, "red": {255, 0, 0, 255}, "purple": {128, 0, 128, 255}, "fuchsia": {255, 0, 255, 255},
	"green": {0, 128, 0, 255}, "lime": {0, 255, 0, 255}, "olive": {128, 128, 0, 255}, "yellow": {255, 255, 0, 255},
	"navy": {0, 0, 128, 255}, "blue": {0, 0, 255, 255}, "teal": {0, 128, 128, 255}, "aqua": {0, 255, 255, 255},
	"orange": {255, 165, 0, 255}, "aliceblue": {240, 248, 255, 255}, "antiquewhite": {250, 235, 215, 255},
	"brown": {165, 42, 42, 255}, "chocolate": {210, 105, 30, 255}, "coral": {255, 127, 80, 255}, "crimson": {220, 20, 60, 255},
	"darkblue": {0, 0, 139, 255}, "darkcyan": {0, 139, 139, 255}, "darkgray": {169, 169, 169, 255}, "darkgreen": {0, 100, 0, 255},
	"gold": {255, 215, 0, 255}, "indigo": {75, 0, 130, 255}, "khaki": {240, 230, 140, 255}, "lavender": {230, 230, 250, 255},
	"lightblue": {173, 216, 230, 255}, "lightgray": {211, 211, 211, 255}, "magenta": {255, 0, 255, 255}, "orchid": {218, 112, 214, 255},
	"pink": {255, 192, 203, 255}, "plum": {221, 160, 221, 255}, "rebeccapurple": {102, 51, 153, 255}, "salmon": {250, 128, 114, 255},
	"skyblue": {135, 206, 235, 255}, "tan": {210, 180, 140, 255}, "tomato": {255, 99, 71, 255}, "violet": {238, 130, 238, 255},
}
