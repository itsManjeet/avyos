package widget

import (
	"testing"

	"avyos.dev/pkg/graphics/canvas/pixbuf"
	"avyos.dev/pkg/graphics/event"
	"avyos.dev/pkg/graphics/geom"
	"avyos.dev/pkg/graphics/theme"
)

func TestSliderTrackClickDoesNotChangeValue(t *testing.T) {
	got := 50.0
	frame := NewFrame(theme.Light(), geom.Sz(240, 40))
	canvas := pixbuf.NewCanvas(240, 40)

	root := SizedBox{
		Width: 200,
		Child: Slider{
			Min:       0,
			Max:       100,
			Value:     50,
			OnChanged: func(v float64) { got = v },
		},
	}

	frame.Render(root, canvas)
	frame.HandlePointerDown(geom.Pt(10, 11), event.ButtonLeft)
	frame.HandlePointerMove(geom.Pt(40, 11))
	frame.HandlePointerUp(geom.Pt(40, 11), event.ButtonLeft)

	if got != 50 {
		t.Fatalf("expected track click to leave value unchanged, got %.2f", got)
	}
}

func TestSliderThumbDragChangesValue(t *testing.T) {
	got := 50.0
	frame := NewFrame(theme.Light(), geom.Sz(240, 40))
	canvas := pixbuf.NewCanvas(240, 40)

	root := SizedBox{
		Width: 200,
		Child: Slider{
			Min:       0,
			Max:       100,
			Value:     50,
			OnChanged: func(v float64) { got = v },
		},
	}

	frame.Render(root, canvas)
	frame.HandlePointerDown(geom.Pt(100, 11), event.ButtonLeft)
	frame.HandlePointerMove(geom.Pt(150, 11))
	frame.HandlePointerUp(geom.Pt(150, 11), event.ButtonLeft)

	if got <= 50 {
		t.Fatalf("expected thumb drag to increase value, got %.2f", got)
	}
}
