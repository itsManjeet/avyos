package ttf

import (
	"math"
	"testing"

	"golang.org/x/image/font"
)

func TestRasterSizeQuantizesHintedFacesToWholePixels(t *testing.T) {
	face := Default().WithDPI(96).WithHinting(font.HintingFull)

	for _, size := range []float64{8, 10, 14, 16} {
		raster := face.rasterSize(size)
		pixels := raster * face.dpi / 72.0
		if math.Abs(pixels-math.Round(pixels)) > 1e-9 {
			t.Fatalf("size %.2f rasterized to %.4f px, want whole-pixel ppem", size, pixels)
		}
	}
}

func TestRasterSizeLeavesUnhintedFacesUnchanged(t *testing.T) {
	face := Default().WithDPI(96).WithHinting(font.HintingNone)

	if got := face.rasterSize(10); got != 10 {
		t.Fatalf("unhinted raster size = %.2f, want 10", got)
	}
}

func TestSnapDotRoundsHintedGlyphPositions(t *testing.T) {
	face := Default().WithHinting(font.HintingFull)

	x, y := face.snapDot(10.25, 21.75)
	if x != 10 || y != 22 {
		t.Fatalf("hinted snap = (%.2f, %.2f), want (10, 22)", x, y)
	}

	unhinted := face.WithHinting(font.HintingNone)
	x, y = unhinted.snapDot(10.25, 21.75)
	if x != 10.25 || y != 21.75 {
		t.Fatalf("unhinted snap = (%.2f, %.2f), want unchanged values", x, y)
	}
}

func TestLinearToSRGB8LUTMatchesFormula(t *testing.T) {
	for i := 0; i <= 10000; i++ {
		v := float64(i) / 10000.0
		got := linearToSRGB8(v)
		want := linearToSRGB8Formula(v)
		diff := int(got) - int(want)
		if diff < 0 {
			diff = -diff
		}
		if diff > 1 {
			t.Fatalf("linearToSRGB8(%0.4f) = %d, want %d (diff=%d)", v, got, want, diff)
		}
	}
}

func linearToSRGB8Formula(v float64) uint8 {
	if v <= 0 {
		return 0
	}
	if v >= 1 {
		return 255
	}
	var s float64
	if v <= 0.0031308 {
		s = 12.92 * v
	} else {
		s = 1.055*math.Pow(v, 1.0/2.4) - 0.055
	}
	return uint8(s*255 + 0.5)
}
