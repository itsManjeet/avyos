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

// Package ttf provides anti-aliased TrueType/OpenType text rendering
// implementing canvas.Typeface via golang.org/x/image/font/opentype.
package ttf

import (
	"image"
	"math"
	"sync"

	"avyos.dev/lib/graphics/color"
	"golang.org/x/image/font"
	"golang.org/x/image/font/gofont/gomono"
	"golang.org/x/image/font/gofont/goregular"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/math/fixed"
)

// Face is a TrueType/OpenType typeface implementing canvas.Typeface.
// Font faces are cached by quantized size so parsing happens only once per size.
type Face struct {
	sfnt    *opentype.Font
	dpi     float64
	hinting font.Hinting
	mu      sync.Mutex
	cache   map[int]*cachedFace // key = round(quantized_size*1000)
}

type cachedFace struct {
	face     font.Face
	metrics  font.Metrics
	mu       sync.Mutex
	advances map[rune]float64
}

var (
	defaultOnce sync.Once
	defaultFace *Face
	monoOnce    sync.Once
	monoFace    *Face
)

// Default returns the built-in Go Regular typeface.
func Default() *Face {
	defaultOnce.Do(func() {
		f, err := New(goregular.TTF)
		if err != nil {
			panic("ttf: parse embedded font: " + err.Error())
		}
		defaultFace = f
	})
	return defaultFace
}

// DefaultMono returns the built-in Go Mono typeface.
func DefaultMono() *Face {
	monoOnce.Do(func() {
		f, err := New(gomono.TTF)
		if err != nil {
			panic("ttf: parse embedded mono font: " + err.Error())
		}
		monoFace = f
	})
	return monoFace
}

// New parses a TrueType/OpenType font from raw bytes and returns a Face
// that renders at screen DPI 96 (1 point ≈ 1.33 physical pixels).
func New(data []byte) (*Face, error) {
	f, err := opentype.Parse(data)
	if err != nil {
		return nil, err
	}
	return &Face{sfnt: f, dpi: 96, hinting: font.HintingFull, cache: make(map[int]*cachedFace)}, nil
}

// WithDPI returns a copy of the Face configured for the given DPI.
func (tf *Face) WithDPI(dpi float64) *Face {
	return &Face{sfnt: tf.sfnt, dpi: dpi, hinting: tf.hinting, cache: make(map[int]*cachedFace)}
}

// WithHinting returns a copy of the Face configured for the given hinting mode.
func (tf *Face) WithHinting(h font.Hinting) *Face {
	return &Face{sfnt: tf.sfnt, dpi: tf.dpi, hinting: h, cache: make(map[int]*cachedFace)}
}

func (tf *Face) faceForSize(size float64) *cachedFace {
	size = tf.rasterSize(size)
	key := int(math.Round(size * 1000))
	tf.mu.Lock()
	defer tf.mu.Unlock()
	if f, ok := tf.cache[key]; ok {
		return f
	}
	f, err := opentype.NewFace(tf.sfnt, &opentype.FaceOptions{
		Size:    size,
		DPI:     tf.dpi,
		Hinting: tf.hinting,
	})
	if err != nil {
		return nil
	}
	cached := &cachedFace{
		face:     f,
		metrics:  f.Metrics(),
		advances: make(map[rune]float64, 128),
	}
	tf.cache[key] = cached
	return cached
}

func (tf *Face) rasterSize(size float64) float64 {
	if size <= 0 {
		return size
	}
	if tf == nil || tf.hinting == font.HintingNone || tf.dpi <= 0 {
		return size
	}
	pixels := math.Round(size * tf.dpi / 72.0)
	if pixels < 1 {
		pixels = 1
	}
	return pixels * 72.0 / tf.dpi
}

func (tf *Face) snapDot(x, baseline float64) (float64, float64) {
	if tf == nil || tf.hinting == font.HintingNone {
		return x, baseline
	}
	return math.Round(x), math.Round(baseline)
}

// srgbLUT maps 8-bit sRGB values [0,255] → linear light [0,1].
// Built once at package init; avoids math.Pow inside the per-pixel loop.
var srgbLUT [256]float64

const linearToSRGBSteps = 4096

var linearToSRGBLUT [linearToSRGBSteps + 1]uint8

func init() {
	for i := range srgbLUT {
		v := float64(i) / 255.0
		if v <= 0.04045 {
			srgbLUT[i] = v / 12.92
		} else {
			srgbLUT[i] = math.Pow((v+0.055)/1.055, 2.4)
		}
	}
	for i := range linearToSRGBLUT {
		v := float64(i) / float64(linearToSRGBSteps)
		var s float64
		if v <= 0.0031308 {
			s = 12.92 * v
		} else {
			s = 1.055*math.Pow(v, 1.0/2.4) - 0.055
		}
		linearToSRGBLUT[i] = uint8(s*255 + 0.5)
	}
}

// linearToSRGB8 converts a linear-light value to an 8-bit sRGB byte.
func linearToSRGB8(v float64) uint8 {
	if v <= 0 {
		return 0
	}
	if v >= 1 {
		return 255
	}
	idx := int(v*float64(linearToSRGBSteps) + 0.5)
	if idx < 0 {
		idx = 0
	} else if idx > linearToSRGBSteps {
		idx = linearToSRGBSteps
	}
	return linearToSRGBLUT[idx]
}

// DrawRune renders r anti-aliased at (x,y) into dst using the fill color col.
// When hinting is enabled, the glyph size and origin are snapped to whole
// device pixels before rasterization to keep stems and edges crisp.
// Blending is performed in linear light to avoid the "too dark" gamma artifact
// common when compositing anti-aliased text in sRGB space.
// y is the top-of-line position (same convention as bitmap font).
func (tf *Face) DrawRune(r rune, size float64, dst *image.RGBA, x, y float64, col color.Color) {
	cached := tf.faceForSize(size)
	if cached == nil {
		return
	}
	// face.Glyph expects dot at the baseline; convert from top-of-line by
	// adding the font ascent so glyphs sit within the widget's bounding box.
	ascent := float64(cached.metrics.Ascent) / 64.0
	x, baseline := tf.snapDot(x, y+ascent)
	dot := fixed.Point26_6{
		X: fixed.Int26_6(x * 64),
		Y: fixed.Int26_6(baseline * 64),
	}
	dr, mask, maskp, _, ok := cached.face.Glyph(dot, r)
	if !ok || mask == nil {
		return
	}

	// Pre-linearise the text color once outside the pixel loop.
	tr := srgbLUT[uint8(col.R*255+0.5)]
	tg := srgbLUT[uint8(col.G*255+0.5)]
	tb := srgbLUT[uint8(col.B*255+0.5)]
	blendGlyphMask(dst, dr, mask, maskp, col.A, tr, tg, tb)
}

// RuneAdvance returns the advance width of r at the given size in pixels.
func (tf *Face) RuneAdvance(r rune, size float64) float64 {
	cached := tf.faceForSize(size)
	if cached == nil {
		return size * 0.6
	}
	cached.mu.Lock()
	defer cached.mu.Unlock()
	if adv, ok := cached.advances[r]; ok {
		return adv
	}
	adv, ok := cached.face.GlyphAdvance(r)
	if !ok {
		return size * 0.6
	}
	width := float64(adv) / 64.0
	cached.advances[r] = width
	return width
}

// LineHeight returns the line height (ascent+descent+leading) at the given size.
func (tf *Face) LineHeight(size float64) float64 {
	cached := tf.faceForSize(size)
	if cached == nil {
		return size * 1.2
	}
	return float64(cached.metrics.Height) / 64.0
}

func blendGlyphMask(dst *image.RGBA, dr image.Rectangle, mask image.Image, maskp image.Point, colorAlpha, tr, tg, tb float64) {
	bounds := dst.Bounds()
	for py := dr.Min.Y; py < dr.Max.Y; py++ {
		for px := dr.Min.X; px < dr.Max.X; px++ {
			if !image.Pt(px, py).In(bounds) {
				continue
			}
			mx := maskp.X + (px - dr.Min.X)
			my := maskp.Y + (py - dr.Min.Y)
			_, _, _, maskA := mask.At(mx, my).RGBA()
			if maskA == 0 {
				continue
			}
			alpha := float64(maskA) / 0xffff * colorAlpha
			if alpha <= 0 {
				continue
			}
			blendLinearRGBA(dst, px, py, alpha, tr, tg, tb)
		}
	}
}

func blendLinearRGBA(dst *image.RGBA, px, py int, alpha, tr, tg, tb float64) {
	if alpha <= 0 {
		return
	}
	off := dst.PixOffset(px, py)
	br := srgbLUT[dst.Pix[off]]
	bg := srgbLUT[dst.Pix[off+1]]
	bb := srgbLUT[dst.Pix[off+2]]
	ba := float64(dst.Pix[off+3]) / 255.0

	oa := alpha + ba*(1-alpha)
	if oa <= 0 {
		return
	}
	inv := ba * (1 - alpha) / oa
	dst.Pix[off] = linearToSRGB8(tr*alpha/oa + br*inv)
	dst.Pix[off+1] = linearToSRGB8(tg*alpha/oa + bg*inv)
	dst.Pix[off+2] = linearToSRGB8(tb*alpha/oa + bb*inv)
	dst.Pix[off+3] = uint8(oa*255 + 0.5)
}
