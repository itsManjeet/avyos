package widget

import (
	"testing"

	"avyos.dev/lib/graphics/canvas/pixbuf"
	"avyos.dev/lib/graphics/color"
	"avyos.dev/lib/graphics/geom"
	"avyos.dev/lib/graphics/paint"
)

func TestEffectImageCacheReusesAcrossIntegerTranslation(t *testing.T) {
	effectImageCache.mu.Lock()
	effectImageCache.entries = nil
	effectImageCache.usedBytes = 0
	effectImageCache.stamp = 0
	effectImageCache.mu.Unlock()
	t.Cleanup(func() {
		effectImageCache.mu.Lock()
		effectImageCache.entries = nil
		effectImageCache.usedBytes = 0
		effectImageCache.stamp = 0
		effectImageCache.mu.Unlock()
	})

	ctx := paint.NewContext(pixbuf.NewCanvas(320, 240))
	shadow := color.Black.WithAlpha(0.16)
	drawSoftShadow(ctx, geom.NewRect(24, 20, 120, 72), 14, shadow, 16, 4)
	firstLen := effectImageCacheLen()
	if firstLen != 1 {
		t.Fatalf("effect cache len after first shadow = %d, want 1", firstLen)
	}

	drawSoftShadow(ctx, geom.NewRect(80, 64, 120, 72), 14, shadow, 16, 4)
	if got := effectImageCacheLen(); got != 1 {
		t.Fatalf("effect cache len after translated shadow = %d, want 1", got)
	}
}

func effectImageCacheLen() int {
	effectImageCache.mu.Lock()
	defer effectImageCache.mu.Unlock()
	return len(effectImageCache.entries)
}
