package svg

import (
	"bytes"
	"image"
	"image/color"
	"testing"
)

func decodeTest(t *testing.T, source string) *image.RGBA {
	t.Helper()
	img, err := Decode(bytes.NewBufferString(source))
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	rgba, ok := img.(*image.RGBA)
	if !ok {
		t.Fatalf("got %T", img)
	}
	return rgba
}

func pixel(img image.Image, x, y int) color.NRGBA {
	return color.NRGBAModel.Convert(img.At(x, y)).(color.NRGBA)
}
func near(a, b uint8, d uint8) bool {
	if a > b {
		return a-b <= d
	}
	return b-a <= d
}

func TestDecodeConfigViewBoxAndUnits(t *testing.T) {
	cfg, err := DecodeConfig(bytes.NewBufferString(`<svg xmlns="http://www.w3.org/2000/svg" width="25.4mm" height="1in"/>`))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Width != 96 || cfg.Height != 96 {
		t.Fatalf("size=%dx%d", cfg.Width, cfg.Height)
	}
	cfg, err = DecodeConfig(bytes.NewBufferString(`<svg viewBox="-10 -20 30 40"/>`))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Width != 30 || cfg.Height != 40 {
		t.Fatalf("viewBox size=%dx%d", cfg.Width, cfg.Height)
	}
}

func TestShapesPathTransformsAndCSS(t *testing.T) {
	img := decodeTest(t, `<svg width="40" height="30" viewBox="0 0 40 30">
<style>g > .hot { fill: rgb(255 0 0); stroke: #00f; stroke-width: 2 } #moved { transform: translate(10px,0) }</style>
<g color="green"><rect class="hot" x="2" y="2" width="10" height="10"/><path id="moved" fill="currentColor" d="M2 18h8v8H2z"/></g></svg>`)
	a := pixel(img, 6, 6)
	if a.R < 230 || a.G > 20 {
		t.Fatalf("CSS fill=%v", a)
	}
	b := pixel(img, 15, 22)
	if b.G < 90 || b.R > 20 {
		t.Fatalf("path/currentColor/transform=%v", b)
	}
	s := pixel(img, 1, 6)
	if s.B < 150 {
		t.Fatalf("stroke=%v", s)
	}
}

func TestPathCommandsAndArc(t *testing.T) {
	img := decodeTest(t, `<svg width="50" height="30"><path fill="#f80" d="M2 25 Q10 2 18 25 T34 25 A8 8 0 0 1 48 18 L48 28H2Z M3 3h4v4h-4z"/></svg>`)
	if pixel(img, 10, 20).A == 0 {
		t.Fatal("quadratic path did not fill")
	}
	if pixel(img, 44, 23).A == 0 {
		t.Fatal("arc path did not fill")
	}
	if pixel(img, 5, 5).A == 0 {
		t.Fatal("subpath following close-path did not fill")
	}
}

func TestFillImplicitlyClosesOpenSubpaths(t *testing.T) {
	img := decodeTest(t, `<svg width="10" height="10"><path d="M1 9L5 1L9 9" fill="red"/></svg>`)
	if pixel(img, 5, 6).A == 0 {
		t.Fatal("open subpath was not implicitly closed for fill")
	}
}

func TestGradientsInheritanceAndSpread(t *testing.T) {
	img := decodeTest(t, `<svg width="60" height="10"><defs><linearGradient id="a"><stop stop-color="red"/><stop offset="1" stop-color="blue"/></linearGradient><linearGradient id="b" href="#a" x1="0" x2="50%" spreadMethod="reflect"/></defs><rect width="60" height="10" fill="url(#b)"/></svg>`)
	l, m, r := pixel(img, 1, 5), pixel(img, 29, 5), pixel(img, 58, 5)
	if l.R < 180 || m.B < 180 || r.R < 150 {
		t.Fatalf("gradient colors left=%v mid=%v right=%v", l, m, r)
	}
}

func TestGroupOpacityClipMaskAndUse(t *testing.T) {
	img := decodeTest(t, `<svg width="40" height="20"><defs><rect id="tile" width="10" height="10" fill="red"/><clipPath id="clip"><circle cx="10" cy="10" r="8"/></clipPath><mask id="fade"><rect width="20" height="20" fill="white" fill-opacity="0.5"/></mask></defs><g opacity="0.5" clip-path="url(#clip)" mask="url(#fade)"><use href="#tile" x="5" y="5"/><use href="#tile" x="10" y="5"/></g></svg>`)
	center := pixel(img, 11, 10)
	if center.R < 180 || center.A < 40 || center.A > 90 {
		t.Fatalf("group compositing=%v", center)
	}
	if pixel(img, 1, 1).A != 0 {
		t.Fatal("clip path leaked")
	}
}

func TestPatternAndFilter(t *testing.T) {
	img := decodeTest(t, `<svg width="30" height="12"><defs><pattern id="p" width="0.5" height="1"><rect width="5" height="10" fill="lime"/></pattern><filter id="blur"><feGaussianBlur stdDeviation="1"/></filter></defs><rect width="20" height="10" fill="url(#p)"/><rect x="24" y="3" width="3" height="3" fill="red" filter="url(#blur)"/></svg>`)
	if pixel(img, 2, 5).G < 150 || pixel(img, 7, 5).A != 0 {
		t.Fatalf("pattern samples=%v %v", pixel(img, 2, 5), pixel(img, 7, 5))
	}
	if pixel(img, 23, 4).A == 0 {
		t.Fatal("blur did not expand alpha")
	}
}

func TestRenderScalesToDestinationBounds(t *testing.T) {
	dst := image.NewRGBA(image.Rect(10, 20, 50, 40))
	err := Render(dst, bytes.NewBufferString(`<svg width="20" height="20"><rect width="20" height="20" fill="#123456"/></svg>`))
	if err != nil {
		t.Fatal(err)
	}
	c := pixel(dst, 30, 30)
	if c.R != 0x12 || c.G != 0x34 || c.B != 0x56 || c.A != 255 {
		t.Fatalf("scaled pixel=%v", c)
	}
}

func TestMalformedPathTerminates(t *testing.T) {
	_ = decodeTest(t, `<svg width="10" height="10"><path d="M 1 1 L nope"/></svg>`)
}
