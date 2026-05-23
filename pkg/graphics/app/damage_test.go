package app

import (
	"image"
	"testing"

	"avyos.dev/pkg/graphics/canvas/pixbuf"
	"avyos.dev/pkg/graphics/geom"
	"avyos.dev/pkg/graphics/theme"
	"avyos.dev/pkg/graphics/widget"
)

func TestResolveDamageRectUsesFrameHintByDefault(t *testing.T) {
	old := DamageProvider
	oldUseHints := Options.UseFrameDamageHints
	DamageProvider = nil
	Options.UseFrameDamageHints = true
	t.Cleanup(func() {
		DamageProvider = old
		Options.UseFrameDamageHints = oldUseHints
	})

	frame := widget.NewFrame(theme.Light(), geom.Sz(100, 60))
	canvas := pixbuf.NewCanvas(100, 60)
	root := widget.Stack{
		Children: []widget.Widget{
			widget.Positioned{
				Left:  widget.Ptr(5),
				Top:   widget.Ptr(5),
				Child: widget.GestureDetector{Child: widget.SizedBox{Width: 40, Height: 20}},
			},
		},
	}

	frame.Render(root, canvas)
	frame.HandlePointerMove(geom.Pt(10, 10))

	want := image.Rect(0, 0, 77, 57)
	if got := resolveDamageRect(frame); got != want {
		t.Fatalf("expected frame hint %v, got %v", want, got)
	}
}

func TestResolveDamageRectPrefersAppDamageProvider(t *testing.T) {
	old := DamageProvider
	oldUseHints := Options.UseFrameDamageHints
	DamageProvider = func() image.Rectangle { return image.Rect(1, 2, 3, 4) }
	Options.UseFrameDamageHints = true
	t.Cleanup(func() {
		DamageProvider = old
		Options.UseFrameDamageHints = oldUseHints
	})

	frame := widget.NewFrame(theme.Light(), geom.Sz(100, 60))

	if got := resolveDamageRect(frame); got != image.Rect(1, 2, 3, 4) {
		t.Fatalf("expected app damage provider to win, got %v", got)
	}
}

func TestResolveDamageRectDisablesFrameHintByDefault(t *testing.T) {
	old := DamageProvider
	oldUseHints := Options.UseFrameDamageHints
	DamageProvider = nil
	Options.UseFrameDamageHints = false
	t.Cleanup(func() {
		DamageProvider = old
		Options.UseFrameDamageHints = oldUseHints
	})

	frame := widget.NewFrame(theme.Light(), geom.Sz(100, 60))
	canvas := pixbuf.NewCanvas(100, 60)
	root := widget.Stack{
		Children: []widget.Widget{
			widget.Positioned{
				Left:  widget.Ptr(5),
				Top:   widget.Ptr(5),
				Child: widget.GestureDetector{Child: widget.SizedBox{Width: 40, Height: 20}},
			},
		},
	}

	frame.Render(root, canvas)
	frame.HandlePointerMove(geom.Pt(10, 10))

	if got := resolveDamageRect(frame); got != (image.Rectangle{}) {
		t.Fatalf("expected no frame hint when disabled, got %v", got)
	}
}
