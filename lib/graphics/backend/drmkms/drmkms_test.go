package drmkms

import (
	"image"
	"testing"

	"avyos.dev/lib/graphics/canvas/pixbuf"
)

func TestBackendScaleReportsPreferredRenderScale(t *testing.T) {
	backend := New("")
	if got := backend.Scale(); got != defaultScale {
		t.Fatalf("scale = %.1f, want %.1f", got, defaultScale)
	}
}

func TestBestModePrefersPreferredMode(t *testing.T) {
	mode, ok := bestMode([]drmModeModeInfo{
		{Hdisplay: 2560, Vdisplay: 1440, Vrefresh: 60},
		{Hdisplay: 1920, Vdisplay: 1080, Vrefresh: 60, Type: drmModeTypePref},
	})
	if !ok {
		t.Fatalf("bestMode() reported no mode")
	}
	if mode.Hdisplay != 1920 || mode.Vdisplay != 1080 {
		t.Fatalf("bestMode() = %+v, want preferred 1920x1080", mode)
	}
}

func TestBestModePrefersHighestResolution(t *testing.T) {
	mode, ok := bestMode([]drmModeModeInfo{
		{Hdisplay: 1280, Vdisplay: 720, Vrefresh: 144},
		{Hdisplay: 1920, Vdisplay: 1080, Vrefresh: 60},
		{Hdisplay: 1600, Vdisplay: 1200, Vrefresh: 75},
	})
	if !ok {
		t.Fatalf("bestMode() reported no mode")
	}
	if mode.Hdisplay != 1920 || mode.Vdisplay != 1080 {
		t.Fatalf("bestMode() = %+v, want 1920x1080", mode)
	}
}

func TestSelectModeUsesExplicitResolution(t *testing.T) {
	mode, err := selectMode([]drmModeModeInfo{
		{Hdisplay: 1920, Vdisplay: 1200, Vrefresh: 60},
		{Hdisplay: 2560, Vdisplay: 1440, Vrefresh: 60},
	}, "1920x1200")
	if err != nil {
		t.Fatalf("selectMode() error = %v", err)
	}
	if mode.Hdisplay != 1920 || mode.Vdisplay != 1200 {
		t.Fatalf("selectMode() = %+v, want 1920x1200", mode)
	}
}

func TestSelectModeUsesHighestRefreshForRequestedResolution(t *testing.T) {
	mode, err := selectMode([]drmModeModeInfo{
		{Hdisplay: 1920, Vdisplay: 1200, Vrefresh: 50},
		{Hdisplay: 1920, Vdisplay: 1200, Vrefresh: 60},
	}, "1920x1200")
	if err != nil {
		t.Fatalf("selectMode() error = %v", err)
	}
	if mode.Vrefresh != 60 {
		t.Fatalf("selectMode() = %+v, want refresh 60", mode)
	}
}

func TestSelectModeRejectsUnavailableResolution(t *testing.T) {
	_, err := selectMode([]drmModeModeInfo{
		{Hdisplay: 1920, Vdisplay: 1200, Vrefresh: 60},
	}, "1600x900")
	if err == nil {
		t.Fatalf("selectMode() unexpectedly succeeded")
	}
}

func BenchmarkBlitRegionToMmapFullHD(b *testing.B) {
	const width = 1536
	const height = 863

	fb := &framebuffer{
		pitch:  width * 4,
		mmap:   make([]byte, width*height*4),
		canvas: pixbuf.NewCanvas(width, height),
	}
	r := image.Rect(0, 0, width, height)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		fb.blitRegionToMmap(width, height, r)
	}
}

func BenchmarkBlitRegionToMmapPartial(b *testing.B) {
	const width = 1536
	const height = 863
	const partialHeight = 268

	fb := &framebuffer{
		pitch:  width * 4,
		mmap:   make([]byte, width*height*4),
		canvas: pixbuf.NewCanvas(width, height),
	}
	r := image.Rect(0, height-partialHeight, width, height)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		fb.blitRegionToMmap(width, height, r)
	}
}
