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

// Text renders a string using a theme text style.
//
// If Style is nil, the theme's BodyMedium style is used.
// Text is a [Buildable] that produces a textLeaf [RenderBox] for painting.
// Text wider than its allocated box is truncated with "…".
//
//	widget.Text{Content: "Hello"}
//	widget.Text{Content: "Title", Style: &ctx.Theme.TextTheme.TitleMedium}
package widget

import (
	"math"
	"reflect"
	"strings"
	"sync"
	"unicode/utf8"

	"avyos.dev/pkg/graphics/canvas"
	"avyos.dev/pkg/graphics/geom"
	"avyos.dev/pkg/graphics/layout"
	"avyos.dev/pkg/graphics/paint"
	"avyos.dev/pkg/graphics/theme"
)

// Text displays a string.
// If Style is nil, the theme's BodyMedium style is used.
type Text struct {
	Content string
	Style   *theme.TextStyle
}

func (t Text) Build(ctx BuildContext) Widget {
	style := t.Style
	if style == nil {
		s := ctx.Theme.TextTheme.BodyMedium
		style = &s
	}
	return textLeaf{content: t.Content, style: *style}
}

// textLeaf is the RenderBox produced by Text.Build.
type textLeaf struct {
	content string
	style   theme.TextStyle
}

const textCacheLimit = 512

type textMeasureCacheKey struct {
	text          string
	faceID        uintptr
	sizeBits      uint64
	letterSpacing uint64
}

type textFitCacheKey struct {
	textMeasureCacheKey
	maxWidth uint64
}

type boundedCache[K comparable, V any] struct {
	mu     sync.Mutex
	order  []K
	values map[K]V
}

var textMeasureCache boundedCache[textMeasureCacheKey, geom.Size]
var textFitCache boundedCache[textFitCacheKey, string]

func (tl textLeaf) Layout(c layout.BoxConstraints) geom.Size {
	if tl.style.Face == nil || tl.content == "" {
		return c.Constrain(geom.Sz(0, textStyleLineHeight(tl.style)))
	}
	return c.Constrain(measureTextStyle(tl.content, tl.style))
}

func (tl textLeaf) Paint(ctx *paint.Context, offset geom.Point, size geom.Size) {
	if tl.content == "" || tl.style.Face == nil || size.Width <= 0 || size.Height <= 0 {
		return
	}
	text := fitTextToWidth(tl.content, tl.style, size.Width)
	if text == "" {
		return
	}
	ctx.Save()
	ctx.ClipRect(geom.NewRect(offset.X, offset.Y, size.Width, size.Height))
	drawTextStyle(ctx, text, offset, tl.style)
	ctx.Restore()
}

func (tl textLeaf) HitTest(_, _ geom.Point, _ geom.Size) bool { return false }

// --- internal text helpers ---

// measureTextStyle returns the bounding size of text rendered with style.
func measureTextStyle(text string, style theme.TextStyle) geom.Size {
	sz := measureTextSpaced(text, style.Face, style.Size, style.LetterSpacing)
	if lh := textStyleLineHeight(style); lh > sz.Height {
		sz.Height = lh
	}
	return sz
}

// measureTextSpaced measures text with optional per-rune letter spacing.
func measureTextSpaced(text string, face canvas.Typeface, size, letterSpacing float64) geom.Size {
	if face == nil || len(text) == 0 {
		return geom.Sz(0, 0)
	}
	key := textMeasureCacheKey{
		text:          text,
		faceID:        faceCacheID(face),
		sizeBits:      math.Float64bits(size),
		letterSpacing: math.Float64bits(letterSpacing),
	}
	if sz, ok := textMeasureCache.get(key); ok {
		return sz
	}
	w := 0.0
	i := 0
	for _, r := range text {
		if i > 0 {
			w += letterSpacing
		}
		w += face.RuneAdvance(r, size)
		i++
	}
	sz := geom.Sz(w, face.LineHeight(size))
	textMeasureCache.set(key, sz)
	return sz
}

// fitTextToWidth returns text (possibly truncated with "...") that fits maxWidth.
func fitTextToWidth(text string, style theme.TextStyle, maxWidth float64) string {
	face := style.Face
	size := style.Size
	if face == nil || text == "" || maxWidth <= 0 {
		return ""
	}
	key := textFitCacheKey{
		textMeasureCacheKey: textMeasureCacheKey{
			text:          text,
			faceID:        faceCacheID(face),
			sizeBits:      math.Float64bits(size),
			letterSpacing: math.Float64bits(style.LetterSpacing),
		},
		maxWidth: math.Float64bits(maxWidth),
	}
	if fitted, ok := textFitCache.get(key); ok {
		return fitted
	}
	ellipsis := "..."
	ellipsisW := measureTextSpaced(ellipsis, face, size, style.LetterSpacing).Width
	w := 0.0
	end := 0
	count := 0
	for end < len(text) {
		r, bytes := utf8.DecodeRuneInString(text[end:])
		adv := face.RuneAdvance(r, size)
		if count > 0 {
			adv += style.LetterSpacing
		}
		nextW := w + adv
		if nextW > maxWidth {
			break
		}
		w = nextW
		end += bytes
		count++
	}
	if end == len(text) {
		textFitCache.set(key, text)
		return text
	}
	if ellipsisW > maxWidth {
		fitted := fitEllipsis(face, size, style.LetterSpacing, maxWidth)
		textFitCache.set(key, fitted)
		return fitted
	}

	limit := maxWidth - ellipsisW
	w = 0
	end = 0
	count = 0
	for end < len(text) {
		r, bytes := utf8.DecodeRuneInString(text[end:])
		adv := face.RuneAdvance(r, size)
		if count > 0 {
			adv += style.LetterSpacing
		}
		if w+adv > limit {
			break
		}
		w += adv
		end += bytes
		count++
	}
	trimmed := strings.TrimRight(text[:end], " ")
	if trimmed == "" {
		fitted := fitEllipsis(face, size, style.LetterSpacing, maxWidth)
		textFitCache.set(key, fitted)
		return fitted
	}
	fitted := trimmed + ellipsis
	textFitCache.set(key, fitted)
	return fitted
}

// fitEllipsis returns as many dots as fit in maxWidth.
func fitEllipsis(face canvas.Typeface, size, letterSpacing, maxWidth float64) string {
	w := 0.0
	count := 0
	for i, r := range "..." {
		adv := face.RuneAdvance(r, size)
		if i > 0 {
			adv += letterSpacing
		}
		if w+adv > maxWidth {
			break
		}
		w += adv
		count++
	}
	return "..."[:count]
}

// drawTextStyle draws text at pos using the given style.
// Handles letter spacing by drawing rune by rune when non-zero.
func drawTextStyle(ctx *paint.Context, text string, pos geom.Point, style theme.TextStyle) {
	if ctx == nil || style.Face == nil || text == "" {
		return
	}
	if style.LetterSpacing == 0 {
		ctx.DrawText(text, pos, style.Face, style.Size, style.Color)
		return
	}
	ctx.Canvas.SetFillColor(style.Color)
	cursor := pos.X
	i := 0
	if rc, ok := ctx.Canvas.(interface {
		DrawRune(r rune, pos geom.Point, face canvas.Typeface, size float64)
	}); ok {
		for _, r := range text {
			if i > 0 {
				cursor += style.LetterSpacing
			}
			rc.DrawRune(r, geom.Pt(cursor, pos.Y), style.Face, style.Size)
			cursor += style.Face.RuneAdvance(r, style.Size)
			i++
		}
		return
	}
	for _, r := range text {
		if i > 0 {
			cursor += style.LetterSpacing
		}
		ctx.Canvas.DrawText(string(r), geom.Pt(cursor, pos.Y), style.Face, style.Size)
		cursor += style.Face.RuneAdvance(r, style.Size)
		i++
	}
}

func textStyleLineHeight(style theme.TextStyle) float64 {
	if style.LineHeight > 0 {
		return style.LineHeight
	}
	if style.Face == nil {
		return 0
	}
	return style.Face.LineHeight(style.Size)
}

func faceCacheID(face canvas.Typeface) uintptr {
	v := reflect.ValueOf(face)
	if !v.IsValid() {
		return 0
	}
	switch v.Kind() {
	case reflect.Pointer, reflect.UnsafePointer, reflect.Map, reflect.Func, reflect.Slice:
		return v.Pointer()
	default:
		return 0
	}
}

func (c *boundedCache[K, V]) get(key K) (V, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.values == nil {
		var zero V
		return zero, false
	}
	v, ok := c.values[key]
	return v, ok
}

func (c *boundedCache[K, V]) set(key K, value V) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.values == nil {
		c.values = make(map[K]V, textCacheLimit)
	}
	if _, ok := c.values[key]; ok {
		c.values[key] = value
		return
	}
	if len(c.values) >= textCacheLimit && len(c.order) > 0 {
		evictCount := len(c.order) / 2
		if evictCount < 1 {
			evictCount = 1
		}
		for i := 0; i < evictCount; i++ {
			delete(c.values, c.order[i])
		}
		c.order = append(c.order[:0], c.order[evictCount:]...)
	}
	c.values[key] = value
	c.order = append(c.order, key)
}
