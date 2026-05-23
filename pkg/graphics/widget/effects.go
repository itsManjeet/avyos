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

// Internal drawing helpers used by widgets in this package.
// None of these are part of the public API.
package widget

import (
	"image"
	"math"
	"sync"

	"avyos.dev/pkg/graphics/canvas/pixbuf"
	"avyos.dev/pkg/graphics/color"
	"avyos.dev/pkg/graphics/geom"
	"avyos.dev/pkg/graphics/paint"
)

const (
	maxEffectLayers     = 16
	minVisibleAlpha     = 0.5 / 255.0
	shadowAlphaScale    = 0.34
	glowAlphaScale      = 0.26
	maxEffectCacheBytes = 24 << 20
	maxEffectImageBytes = 6 << 20
)

type effectLayer struct {
	growFactor   float64
	offsetFactor float64
	shadowAlpha  float64
	glowAlpha    float64
}

var (
	effectLayersMu sync.Mutex
	effectLayers   [maxEffectLayers + 1][]effectLayer
)

type effectKind uint8

const (
	effectKindShadow effectKind = iota + 1
	effectKindGlow
)

type effectImageCacheKey struct {
	kind      effectKind
	color     uint32
	imageW    int32
	imageH    int32
	localMinX uint64
	localMinY uint64
	localW    uint64
	localH    uint64
	radius    uint64
	spread    uint64
	offsetY   uint64
}

type effectImageCacheEntry struct {
	img   *image.RGBA
	bytes int
	stamp uint64
}

var effectImageCache struct {
	mu        sync.Mutex
	entries   map[effectImageCacheKey]*effectImageCacheEntry
	usedBytes int
	stamp     uint64
}

// drawSoftShadow paints a multi-layer soft drop shadow under r.
func drawSoftShadow(pctx *paint.Context, r geom.Rect, radius float64, col color.Color, spread, offsetY float64) {
	if pctx == nil || col.A <= 0 || spread <= 0 {
		return
	}
	drawEffect(pctx, effectKindShadow, r, radius, col, spread, offsetY, effectLayerSchedule(shadowLayerCount(spread)))
}

// drawGlow paints a multi-layer glow ring around r.
func drawGlow(pctx *paint.Context, r geom.Rect, radius float64, col color.Color, spread float64) {
	if pctx == nil || col.A <= 0 || spread <= 0 {
		return
	}
	drawEffect(pctx, effectKindGlow, r, radius, col, spread, 0, effectLayerSchedule(glowLayerCount(spread)))
}

func layerCount(spread float64, lo, hi int) int {
	n := int(math.Ceil(spread * 0.7))
	if n < lo {
		return lo
	}
	if n > hi {
		return hi
	}
	return n
}

func shadowLayerCount(spread float64) int { return layerCount(spread, 6, 14) }

func glowLayerCount(spread float64) int { return layerCount(spread, 4, 12) }

func spreadCurve(t float64) float64   { return math.Pow(clamp01(t), 1.35) }
func shadowProfile(t float64) float64 { return math.Pow(1-clamp01(t), 2.2) }
func glowProfile(t float64) float64   { return math.Pow(1-clamp01(t), 1.8) }

func effectLayerSchedule(layers int) []effectLayer {
	if layers <= 0 {
		return nil
	}
	if layers > maxEffectLayers {
		layers = maxEffectLayers
	}
	effectLayersMu.Lock()
	defer effectLayersMu.Unlock()
	if cached := effectLayers[layers]; cached != nil {
		return cached
	}
	schedule := make([]effectLayer, 0, layers)
	for i := layers; i >= 1; i-- {
		outer := spreadCurve(float64(i) / float64(layers))
		inner := spreadCurve(float64(i-1) / float64(layers))
		schedule = append(schedule, effectLayer{
			growFactor:   outer,
			offsetFactor: math.Pow(outer, 1.1),
			shadowAlpha:  shadowProfile(inner) - shadowProfile(outer),
			glowAlpha:    glowProfile(inner) - glowProfile(outer),
		})
	}
	effectLayers[layers] = schedule
	return schedule
}

func drawEffect(pctx *paint.Context, kind effectKind, r geom.Rect, radius float64, col color.Color, spread, offsetY float64, schedule []effectLayer) {
	if img, dst, ok := cachedEffectImage(kind, r, radius, col, spread, offsetY, schedule); ok {
		if img != nil {
			pctx.Canvas.DrawImage(img, dst)
		}
		return
	}
	renderEffectLayers(pctx, kind, r, radius, col, spread, offsetY, schedule)
}

func cachedEffectImage(kind effectKind, r geom.Rect, radius float64, col color.Color, spread, offsetY float64, schedule []effectLayer) (*image.RGBA, geom.Rect, bool) {
	bounds, localRect, ok := effectImageLayout(kind, r, col, spread, offsetY, schedule)
	if !ok {
		return nil, geom.Rect{}, true
	}
	width := bounds.Dx()
	height := bounds.Dy()
	if width <= 0 || height <= 0 {
		return nil, geom.Rect{}, true
	}
	bytes := width * height * 4
	if bytes <= 0 || bytes > maxEffectImageBytes {
		return nil, geom.Rect{}, false
	}
	key := effectImageCacheKey{
		kind:      kind,
		color:     col.ARGB32(),
		imageW:    int32(width),
		imageH:    int32(height),
		localMinX: math.Float64bits(localRect.Min.X),
		localMinY: math.Float64bits(localRect.Min.Y),
		localW:    math.Float64bits(localRect.Width()),
		localH:    math.Float64bits(localRect.Height()),
		radius:    math.Float64bits(radius),
		spread:    math.Float64bits(spread),
		offsetY:   math.Float64bits(offsetY),
	}
	dst := geom.NewRect(float64(bounds.Min.X), float64(bounds.Min.Y), float64(width), float64(height))
	if img := loadEffectImage(key); img != nil {
		return img, dst, true
	}
	cv := pixbuf.NewCanvas(width, height)
	renderEffectLayers(paint.NewContext(cv), kind, localRect, radius, col, spread, offsetY, schedule)
	return storeEffectImage(key, cv.Image()), dst, true
}

func effectImageLayout(kind effectKind, r geom.Rect, col color.Color, spread, offsetY float64, schedule []effectLayer) (image.Rectangle, geom.Rect, bool) {
	var bounds geom.Rect
	hasVisible := false
	for _, layerInfo := range schedule {
		alpha := effectLayerAlpha(kind, col.A, layerInfo)
		if alpha <= 0 || alpha < minVisibleAlpha {
			continue
		}
		layerRect, _ := effectLayerGeometry(kind, r, spread, offsetY, layerInfo)
		if !hasVisible {
			bounds = layerRect
			hasVisible = true
			continue
		}
		bounds = bounds.Union(layerRect)
	}
	if !hasVisible || bounds.Empty() {
		return image.Rectangle{}, geom.Rect{}, false
	}
	minX := int(math.Floor(bounds.Min.X))
	minY := int(math.Floor(bounds.Min.Y))
	maxX := int(math.Ceil(bounds.Max.X))
	maxY := int(math.Ceil(bounds.Max.Y))
	if maxX <= minX || maxY <= minY {
		return image.Rectangle{}, geom.Rect{}, false
	}
	return image.Rect(minX, minY, maxX, maxY), r.Translate(-float64(minX), -float64(minY)), true
}

func renderEffectLayers(pctx *paint.Context, kind effectKind, r geom.Rect, radius float64, col color.Color, spread, offsetY float64, schedule []effectLayer) {
	for _, layerInfo := range schedule {
		alpha := effectLayerAlpha(kind, col.A, layerInfo)
		if alpha <= 0 || alpha < minVisibleAlpha {
			continue
		}
		layerRect, grow := effectLayerGeometry(kind, r, spread, offsetY, layerInfo)
		pctx.FillRoundedRect(layerRect, radius+grow, col.WithAlpha(alpha))
	}
}

func effectLayerAlpha(kind effectKind, baseAlpha float64, layerInfo effectLayer) float64 {
	switch kind {
	case effectKindGlow:
		return baseAlpha * glowAlphaScale * layerInfo.glowAlpha
	default:
		return baseAlpha * shadowAlphaScale * layerInfo.shadowAlpha
	}
}

func effectLayerGeometry(kind effectKind, r geom.Rect, spread, offsetY float64, layerInfo effectLayer) (geom.Rect, float64) {
	grow := spread * layerInfo.growFactor
	layerRect := r.Inset(-grow, -grow)
	if kind == effectKindShadow {
		layerRect = layerRect.Translate(0, offsetY*layerInfo.offsetFactor)
	}
	return layerRect, grow
}

func loadEffectImage(key effectImageCacheKey) *image.RGBA {
	effectImageCache.mu.Lock()
	defer effectImageCache.mu.Unlock()
	if entry := effectImageCache.entries[key]; entry != nil {
		effectImageCache.stamp++
		entry.stamp = effectImageCache.stamp
		return entry.img
	}
	return nil
}

func storeEffectImage(key effectImageCacheKey, img *image.RGBA) *image.RGBA {
	if img == nil || len(img.Pix) == 0 || len(img.Pix) > maxEffectImageBytes {
		return img
	}
	effectImageCache.mu.Lock()
	defer effectImageCache.mu.Unlock()
	if effectImageCache.entries == nil {
		effectImageCache.entries = make(map[effectImageCacheKey]*effectImageCacheEntry)
	}
	if entry := effectImageCache.entries[key]; entry != nil {
		effectImageCache.stamp++
		entry.stamp = effectImageCache.stamp
		return entry.img
	}
	effectImageCache.stamp++
	effectImageCache.entries[key] = &effectImageCacheEntry{
		img:   img,
		bytes: len(img.Pix),
		stamp: effectImageCache.stamp,
	}
	effectImageCache.usedBytes += len(img.Pix)
	for effectImageCache.usedBytes > maxEffectCacheBytes && len(effectImageCache.entries) > 0 {
		var oldestKey effectImageCacheKey
		var oldest *effectImageCacheEntry
		for key, entry := range effectImageCache.entries {
			if oldest == nil || entry.stamp < oldest.stamp {
				oldestKey = key
				oldest = entry
			}
		}
		if oldest == nil {
			break
		}
		delete(effectImageCache.entries, oldestKey)
		effectImageCache.usedBytes -= oldest.bytes
	}
	return img
}

// insetCornerRadius shrinks a border-radius by an inset amount,
// returning 0 if the result would be negative.
func insetCornerRadius(radius, inset float64) float64 {
	if r := radius - inset; r > 0 {
		return r
	}
	return 0
}

// mixColor linearly interpolates between from and to by t ∈ [0,1].
func mixColor(from, to color.Color, t float64) color.Color {
	return from.Lerp(to, clamp01(t))
}

// lightenColor blends c toward white by amount ∈ [0,1].
func lightenColor(c color.Color, amount float64) color.Color {
	return mixColor(c, color.White, amount)
}

// boolToFloat returns 1 if v is true, 0 otherwise.
func boolToFloat(v bool) float64 {
	if v {
		return 1
	}
	return 0
}

// interactionTarget maps an InteractionState to a target animation value.
// 0 = idle, 0.6 = hovered, 1 = pressed.
func interactionTarget(s InteractionState) float64 {
	if s.Pressed {
		return 1
	}
	if s.Hovered {
		return 0.6
	}
	return 0
}

// clampFloat64 clamps v to [lo, hi].
func clampFloat64(v, lo, hi float64) float64 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func minf(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}

func maxf(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}
