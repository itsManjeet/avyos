package svg_test

import (
	"image"
	"image/color"
	stddraw "image/draw"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"avyos.dev/pkg/graphics/svg"
)

func TestDecodeConfig(t *testing.T) {
	cfg, err := svg.DecodeConfig(strings.NewReader(`<svg width="24" height="16" xmlns="http://www.w3.org/2000/svg"/>`))
	if err != nil {
		t.Fatalf("DecodeConfig() error = %v", err)
	}
	if cfg.Width != 24 || cfg.Height != 16 {
		t.Fatalf("DecodeConfig() size = %dx%d, want 24x16", cfg.Width, cfg.Height)
	}
}

func TestDecodeSolidRect(t *testing.T) {
	img, err := svg.Decode(strings.NewReader(`<svg width="8" height="8" xmlns="http://www.w3.org/2000/svg"><rect width="8" height="8" fill="#ff0000"/></svg>`))
	if err != nil {
		t.Fatalf("Decode() error = %v", err)
	}
	r, g, b, a := img.At(4, 4).RGBA()
	if r < 0xff00 || g != 0 || b != 0 || a != 0xffff {
		t.Fatalf("center pixel = rgba(%#x,%#x,%#x,%#x), want solid red", r, g, b, a)
	}
}

func TestDecodeViewBoxScale(t *testing.T) {
	img, err := svg.Decode(strings.NewReader(`<svg width="20" height="10" viewBox="0 0 10 5" xmlns="http://www.w3.org/2000/svg"><rect x="0" y="0" width="10" height="5" fill="blue"/></svg>`))
	if err != nil {
		t.Fatalf("Decode() error = %v", err)
	}
	if got := img.Bounds(); got != image.Rect(0, 0, 20, 10) {
		t.Fatalf("bounds = %v, want 20x10", got)
	}
	_, _, b, a := img.At(19, 9).RGBA()
	if b == 0 || a == 0 {
		t.Fatalf("scaled pixel is transparent, want blue fill")
	}
}

func TestImageDecodeRegistration(t *testing.T) {
	img, format, err := image.Decode(strings.NewReader(`<svg width="6" height="4" xmlns="http://www.w3.org/2000/svg"><circle cx="3" cy="2" r="2" fill="lime"/></svg>`))
	if err != nil {
		t.Fatalf("image.Decode() error = %v", err)
	}
	if format != "svg" {
		t.Fatalf("image.Decode() format = %q, want svg", format)
	}
	if got := img.Bounds(); got != image.Rect(0, 0, 6, 4) {
		t.Fatalf("bounds = %v, want 6x4", got)
	}
}

func TestImageDecodeRegistrationXMLProlog(t *testing.T) {
	_, format, err := image.Decode(strings.NewReader(`<?xml version="1.0" encoding="UTF-8"?><svg width="4" height="4" xmlns="http://www.w3.org/2000/svg"><rect width="4" height="4" fill="red"/></svg>`))
	if err != nil {
		t.Fatalf("image.Decode() error = %v", err)
	}
	if format != "svg" {
		t.Fatalf("image.Decode() format = %q, want svg", format)
	}
}

func TestDecodeLinearGradient(t *testing.T) {
	img, err := svg.Decode(strings.NewReader(`<svg width="20" height="4" xmlns="http://www.w3.org/2000/svg"><defs><linearGradient id="g" x1="0%" y1="0%" x2="100%" y2="0%"><stop offset="0%" stop-color="#ff0000"/><stop offset="100%" stop-color="#0000ff"/></linearGradient></defs><rect width="20" height="4" fill="url(#g)"/></svg>`))
	if err != nil {
		t.Fatalf("Decode() error = %v", err)
	}
	leftR, _, leftB, _ := img.At(1, 2).RGBA()
	rightR, _, rightB, _ := img.At(18, 2).RGBA()
	if leftR <= leftB {
		t.Fatalf("left pixel = rgba(%#x,_,%#x,_), want red-dominant", leftR, leftB)
	}
	if rightB <= rightR {
		t.Fatalf("right pixel = rgba(%#x,_,%#x,_), want blue-dominant", rightR, rightB)
	}
}

func TestDecodeGradientHrefWithStopStyle(t *testing.T) {
	img, err := svg.Decode(strings.NewReader(`<svg width="20" height="4" xmlns="http://www.w3.org/2000/svg" xmlns:xlink="http://www.w3.org/1999/xlink"><defs><linearGradient id="base"><stop offset="0%" style="stop-color:#ff0000;stop-opacity:1"/><stop offset="100%" style="stop-color:#0000ff;stop-opacity:0.5"/></linearGradient><linearGradient id="derived" xlink:href="#base" x1="0%" y1="0%" x2="100%" y2="0%"/></defs><rect width="20" height="4" fill="url(#derived)"/></svg>`))
	if err != nil {
		t.Fatalf("Decode() error = %v", err)
	}
	leftR, _, leftB, leftA := img.At(1, 2).RGBA()
	rightR, _, rightB, rightA := img.At(18, 2).RGBA()
	if leftR <= leftB || leftA == 0 {
		t.Fatalf("left pixel = rgba(%#x,_,%#x,%#x), want visible red gradient stop", leftR, leftB, leftA)
	}
	if rightB <= rightR || rightA == 0 {
		t.Fatalf("right pixel = rgba(%#x,_,%#x,%#x), want visible blue gradient stop", rightR, rightB, rightA)
	}
}

func TestRenderScalesToDestinationBounds(t *testing.T) {
	dst := image.NewRGBA(image.Rect(0, 0, 40, 20))
	stddraw.Draw(dst, dst.Bounds(), &image.Uniform{C: color.Transparent}, image.Point{}, stddraw.Src)
	err := svg.Render(dst, strings.NewReader(`<svg width="10" height="5" xmlns="http://www.w3.org/2000/svg"><rect width="10" height="5" fill="#00ff00"/></svg>`))
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}
	_, g, _, a := dst.At(39, 19).RGBA()
	if g == 0 || a == 0 {
		t.Fatalf("scaled pixel is transparent, want green fill")
	}
}

func TestDecodeHandlesEmptyStyleAndBrokenPathTail(t *testing.T) {
	img, err := svg.Decode(strings.NewReader(`<svg xmlns="http://www.w3.org/2000/svg" width="16" height="16"><defs><style type="text/css"></style></defs><path d="M 13,9 3,15 h 11 v -3 l" fill="#00ff00"/></svg>`))
	if err != nil {
		t.Fatalf("Decode() error = %v", err)
	}
	_, g, _, a := img.At(10, 10).RGBA()
	if g == 0 || a == 0 {
		t.Fatalf("pixel is transparent, want green fill")
	}
}

func TestDecodeHandlesNaNInPathData(t *testing.T) {
	img, err := svg.Decode(strings.NewReader(`<svg xmlns="http://www.w3.org/2000/svg" width="10" height="10"><path d="M8 5a3 3 NaN 1 1-6 0 3 3 NaN 1 1 6 0z" fill="#ffffff"/></svg>`))
	if err != nil {
		t.Fatalf("Decode() error = %v", err)
	}
	_, _, _, a := img.At(5, 5).RGBA()
	if a == 0 {
		t.Fatalf("center pixel is transparent, want rasterized path")
	}
}

func TestDecodeAppliesInheritedGroupOpacity(t *testing.T) {
	img, err := svg.Decode(strings.NewReader(`<svg xmlns="http://www.w3.org/2000/svg" width="8" height="8"><g fill="#ffffff" opacity=".5"><rect width="8" height="8"/></g></svg>`))
	if err != nil {
		t.Fatalf("Decode() error = %v", err)
	}
	r, g, b, a := img.At(4, 4).RGBA()
	if a < 0x7000 || a > 0x9000 {
		t.Fatalf("alpha = %#x, want approximately 50%% opacity", a)
	}
	if r == 0 || g == 0 || b == 0 {
		t.Fatalf("pixel = rgba(%#x,%#x,%#x,%#x), want visible semi-transparent white", r, g, b, a)
	}
}

func TestDecodeFileResolvesAlias(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "real.svg")
	alias := filepath.Join(dir, "alias.svg")
	if err := os.WriteFile(target, []byte(`<svg xmlns="http://www.w3.org/2000/svg" width="6" height="6"><rect width="6" height="6" fill="#ff00ff"/></svg>`), 0o644); err != nil {
		t.Fatalf("WriteFile(real.svg) error = %v", err)
	}
	if err := os.WriteFile(alias, []byte("real.svg\n"), 0o644); err != nil {
		t.Fatalf("WriteFile(alias.svg) error = %v", err)
	}
	img, err := svg.DecodeFile(alias)
	if err != nil {
		t.Fatalf("DecodeFile() error = %v", err)
	}
	r, _, b, a := img.At(3, 3).RGBA()
	if r == 0 || b == 0 || a == 0 {
		t.Fatalf("alias pixel = rgba(%#x,_,%#x,%#x), want magenta", r, b, a)
	}
}

func TestDecodeFileResolvesAliasAcrossSiblingThemes(t *testing.T) {
	root := t.TempDir()
	darkDir := filepath.Join(root, "data", "icons", "default-dark", "16", "actions")
	defaultDir := filepath.Join(root, "data", "icons", "default", "symbolic", "actions")
	if err := os.MkdirAll(darkDir, 0o755); err != nil {
		t.Fatalf("MkdirAll(darkDir) error = %v", err)
	}
	if err := os.MkdirAll(defaultDir, 0o755); err != nil {
		t.Fatalf("MkdirAll(defaultDir) error = %v", err)
	}
	target := filepath.Join(darkDir, "bookmark-add-folder.svg")
	alias := filepath.Join(defaultDir, "bookmark-add-symbolic.svg")
	if err := os.WriteFile(target, []byte(`<svg xmlns="http://www.w3.org/2000/svg" width="6" height="6"><rect width="6" height="6" fill="#00ffff"/></svg>`), 0o644); err != nil {
		t.Fatalf("WriteFile(target) error = %v", err)
	}
	if err := os.WriteFile(alias, []byte("../../16/actions/bookmark-add-folder.svg\n"), 0o644); err != nil {
		t.Fatalf("WriteFile(alias) error = %v", err)
	}
	img, err := svg.DecodeFile(alias)
	if err != nil {
		t.Fatalf("DecodeFile() error = %v", err)
	}
	_, g, b, a := img.At(3, 3).RGBA()
	if g == 0 || b == 0 || a == 0 {
		t.Fatalf("sibling alias pixel = rgba(_, %#x, %#x, %#x), want cyan", g, b, a)
	}
}

func TestDecodeFileNormalizesSiblingThemePalette(t *testing.T) {
	root := t.TempDir()
	darkDir := filepath.Join(root, "data", "icons", "default-dark", "16", "actions")
	defaultDir := filepath.Join(root, "data", "icons", "default", "symbolic", "actions")
	if err := os.MkdirAll(darkDir, 0o755); err != nil {
		t.Fatalf("MkdirAll(darkDir) error = %v", err)
	}
	if err := os.MkdirAll(defaultDir, 0o755); err != nil {
		t.Fatalf("MkdirAll(defaultDir) error = %v", err)
	}
	target := filepath.Join(darkDir, "bookmark-new.svg")
	alias := filepath.Join(defaultDir, "bookmark-add-symbolic.svg")
	if err := os.WriteFile(target, []byte(`<svg xmlns="http://www.w3.org/2000/svg" width="6" height="6"><defs><style type="text/css">.ColorScheme-Text { color:#aaaaaa; }</style></defs><rect width="6" height="6" class="ColorScheme-Text" fill="currentColor"/></svg>`), 0o644); err != nil {
		t.Fatalf("WriteFile(target) error = %v", err)
	}
	if err := os.WriteFile(alias, []byte("../../16/actions/bookmark-new.svg\n"), 0o644); err != nil {
		t.Fatalf("WriteFile(alias) error = %v", err)
	}
	img, err := svg.DecodeFile(alias)
	if err != nil {
		t.Fatalf("DecodeFile() error = %v", err)
	}
	r, g, b, a := img.At(3, 3).RGBA()
	if r>>8 != 0x50 || g>>8 != 0x50 || b>>8 != 0x50 || a != 0xffff {
		t.Fatalf("pixel = rgba(%#x,%#x,%#x,%#x), want normalized #505050", r, g, b, a)
	}
}

func TestDecodeResolvesCurrentColorFromCSSClass(t *testing.T) {
	img, err := svg.Decode(strings.NewReader(`<svg xmlns="http://www.w3.org/2000/svg" width="8" height="8"><defs><style type="text/css">.ColorScheme-Text { color:#565656; }</style></defs><rect width="8" height="8" class="ColorScheme-Text" fill="currentColor"/></svg>`))
	if err != nil {
		t.Fatalf("Decode() error = %v", err)
	}
	r, g, b, a := img.At(4, 4).RGBA()
	if r>>8 != 0x56 || g>>8 != 0x56 || b>>8 != 0x56 || a != 0xffff {
		t.Fatalf("pixel = rgba(%#x,%#x,%#x,%#x), want #565656", r, g, b, a)
	}
}

func TestDecodeResolvesCurrentColorFromColorAttribute(t *testing.T) {
	img, err := svg.Decode(strings.NewReader(`<svg xmlns="http://www.w3.org/2000/svg" width="8" height="8"><rect width="8" height="8" color="#5294e2" fill="currentColor"/></svg>`))
	if err != nil {
		t.Fatalf("Decode() error = %v", err)
	}
	r, g, b, a := img.At(4, 4).RGBA()
	if r>>8 != 0x52 || g>>8 != 0x94 || b>>8 != 0xe2 || a != 0xffff {
		t.Fatalf("pixel = rgba(%#x,%#x,%#x,%#x), want #5294e2", r, g, b, a)
	}
}

func TestDecodeAppIconGradientOpacity(t *testing.T) {
	path := filepath.Join("..", "..", "..", "apps", "files", "icon.svg")
	img, err := svg.DecodeFile(path)
	if err != nil {
		t.Fatalf("DecodeFile() error = %v", err)
	}

	tests := []struct {
		x, y     int
		minR     uint8
		minB     uint8
		minAlpha uint8
	}{
		{x: 57, y: 44, minR: 0x60, minB: 0xc8, minAlpha: 0xf0},
		{x: 50, y: 52, minR: 0x58, minB: 0xb0, minAlpha: 0xf0},
		{x: 57, y: 52, minR: 0x58, minB: 0xb0, minAlpha: 0xf0},
	}
	for _, tt := range tests {
		got := color.RGBAModel.Convert(img.At(tt.x, tt.y)).(color.RGBA)
		if got.R < tt.minR || got.B < tt.minB || got.A < tt.minAlpha || got.B <= got.R {
			t.Fatalf("pixel (%d,%d) = %#v, want a visible purple corner instead of an over-darkened/black patch", tt.x, tt.y, got)
		}
	}
}

func TestDecodeSizedFileFitsAspectRatio(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "wide.svg")
	if err := os.WriteFile(path, []byte(`<svg xmlns="http://www.w3.org/2000/svg" width="20" height="10"><rect width="20" height="10" fill="#0000ff"/></svg>`), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	img, err := svg.DecodeSizedFile(path, 10, 10)
	if err != nil {
		t.Fatalf("DecodeSizedFile() error = %v", err)
	}
	_, _, _, topA := img.At(5, 0).RGBA()
	_, _, midB, midA := img.At(5, 5).RGBA()
	if topA != 0 {
		t.Fatalf("top pixel alpha = %#x, want transparent letterbox", topA)
	}
	if midA == 0 || midB == 0 {
		t.Fatalf("middle pixel = blue %#x alpha %#x, want visible blue fill", midB, midA)
	}
}
