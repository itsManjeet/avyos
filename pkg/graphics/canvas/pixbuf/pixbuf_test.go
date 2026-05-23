package pixbuf

import (
	"image"
	stdcolor "image/color"
	"testing"

	"avyos.dev/pkg/graphics/color"
	"avyos.dev/pkg/graphics/geom"
)

type blockFace struct{}

func (blockFace) DrawRune(_ rune, _ float64, dst *image.RGBA, x, y float64, col color.Color) {
	for py := int(y); py < int(y)+4; py++ {
		for px := int(x); px < int(x)+6; px++ {
			if !image.Pt(px, py).In(dst.Bounds()) {
				continue
			}
			off := dst.PixOffset(px, py)
			r, g, b, a := col.RGBA8()
			dst.Pix[off+0] = r
			dst.Pix[off+1] = g
			dst.Pix[off+2] = b
			dst.Pix[off+3] = a
		}
	}
}

func (blockFace) RuneAdvance(_ rune, _ float64) float64 { return 6 }

func (blockFace) LineHeight(_ float64) float64 { return 4 }

func TestDrawTextHonorsClipRect(t *testing.T) {
	c := NewCanvas(12, 8)
	c.SetFillColor(color.White)
	c.ClipRect(geom.NewRect(0, 0, 4, 8))

	c.DrawText("A", geom.Pt(0, 0), blockFace{}, 1)

	if alphaAt(c.img, 3, 1) == 0 {
		t.Fatal("expected text to draw inside clip")
	}
	if alphaAt(c.img, 4, 1) != 0 {
		t.Fatal("expected text outside clip to remain untouched")
	}
}

func TestFillRoundedRectClampsOversizedRadius(t *testing.T) {
	c := NewCanvas(12, 8)
	c.SetFillColor(color.White)

	// A large radius should clamp to half the rect size instead of collapsing
	// the shape into mostly-transparent corners.
	c.FillRoundedRect(geom.NewRect(2, 2, 6, 2), 10)

	if alphaAt(c.img, 5, 2) == 0 {
		t.Fatal("expected clamped rounded rect to preserve the center fill")
	}
	if alphaAt(c.img, 0, 0) != 0 {
		t.Fatal("expected pixels outside the rounded rect to remain untouched")
	}
}

func TestDrawImageDownscaleUsesBilinearSampling(t *testing.T) {
	src := image.NewRGBA(image.Rect(0, 0, 2, 2))
	src.SetRGBA(0, 0, stdcolor.RGBA{R: 255, G: 0, B: 0, A: 255})
	src.SetRGBA(1, 0, stdcolor.RGBA{R: 0, G: 255, B: 0, A: 255})
	src.SetRGBA(0, 1, stdcolor.RGBA{R: 0, G: 0, B: 255, A: 255})
	src.SetRGBA(1, 1, stdcolor.RGBA{R: 255, G: 255, B: 255, A: 255})

	c := NewCanvas(1, 1)
	c.DrawImage(src, geom.NewRect(0, 0, 1, 1))

	got := c.img.RGBAAt(0, 0)
	if got.A != 255 {
		t.Fatalf("expected opaque downscaled pixel, got alpha %d", got.A)
	}
	if got.R < 120 || got.R > 136 || got.G < 120 || got.G > 136 || got.B < 120 || got.B > 136 {
		t.Fatalf("expected bilinear average near mid-gray, got %#v", got)
	}
}

func TestDrawOpaqueImageExactNRGBACopiesPixels(t *testing.T) {
	src := image.NewNRGBA(image.Rect(0, 0, 3, 2))
	src.SetNRGBA(0, 0, stdcolor.NRGBA{R: 10, G: 20, B: 30, A: 255})
	src.SetNRGBA(1, 0, stdcolor.NRGBA{R: 40, G: 50, B: 60, A: 255})
	src.SetNRGBA(2, 0, stdcolor.NRGBA{R: 70, G: 80, B: 90, A: 255})
	src.SetNRGBA(0, 1, stdcolor.NRGBA{R: 110, G: 120, B: 130, A: 255})
	src.SetNRGBA(1, 1, stdcolor.NRGBA{R: 140, G: 150, B: 160, A: 255})
	src.SetNRGBA(2, 1, stdcolor.NRGBA{R: 170, G: 180, B: 190, A: 255})

	dst := NewCanvas(5, 4)
	dst.SetFillColor(color.Black)
	dst.Clear(color.Black)

	dst.DrawOpaqueImage(src, geom.NewRect(1, 1, 3, 2))

	for y := 0; y < 2; y++ {
		for x := 0; x < 3; x++ {
			got := dst.img.RGBAAt(x+1, y+1)
			want := src.NRGBAAt(x, y)
			if got.R != want.R || got.G != want.G || got.B != want.B || got.A != 255 {
				t.Fatalf("pixel (%d,%d): got %#v want %#v", x, y, got, want)
			}
		}
	}
}

func alphaAt(img *image.RGBA, x, y int) uint8 {
	return img.Pix[img.PixOffset(x, y)+3]
}

func BenchmarkFillRoundedRectOpaque(b *testing.B) {
	c := NewCanvas(240, 120)
	r := geom.NewRect(12, 12, 216, 96)
	c.SetFillColor(color.White)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		c.FillRoundedRect(r, 18)
	}
}

func BenchmarkFillRoundedRectLargeAlpha(b *testing.B) {
	c := NewCanvas(1536, 863)
	r := geom.NewRect(120, 80, 1100, 640)
	c.SetFillColor(color.Black.WithAlpha(0.12))

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		c.FillRoundedRect(r, 18)
	}
}

func BenchmarkStrokeRoundedRectOpaque(b *testing.B) {
	c := NewCanvas(240, 120)
	r := geom.NewRect(12, 12, 216, 96)
	c.SetStrokeColor(color.White)
	c.SetLineWidth(1)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		c.StrokeRoundedRect(r, 18)
	}
}

func BenchmarkDrawImageExactNRGBA(b *testing.B) {
	src := image.NewNRGBA(image.Rect(0, 0, 320, 180))
	dst := NewCanvas(320, 180)
	r := geom.NewRect(0, 0, 320, 180)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		dst.DrawImage(src, r)
	}
}

func BenchmarkDrawOpaqueImageExactNRGBA(b *testing.B) {
	src := image.NewNRGBA(image.Rect(0, 0, 320, 180))
	dst := NewCanvas(320, 180)
	r := geom.NewRect(0, 0, 320, 180)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		dst.DrawOpaqueImage(src, r)
	}
}
