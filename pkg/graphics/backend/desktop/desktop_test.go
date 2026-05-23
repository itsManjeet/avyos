package desktop

import (
	"testing"

	"avyos.dev/pkg/graphics/event"
)

func TestScalePixelsRoundTrip(t *testing.T) {
	if got := scalePixels(320, 2); got != 640 {
		t.Fatalf("expected 640 scaled pixels, got %d", got)
	}
	if got := logicalPixels(640, 2); got != 320 {
		t.Fatalf("expected 320 logical pixels, got %d", got)
	}
}

func TestNormalizeScaleDefaultsToOne(t *testing.T) {
	if got := normalizeScale(0); got != 1 {
		t.Fatalf("expected default scale 1, got %.1f", got)
	}
}

func TestHandleRemoteResizeUsesScaledPixelSize(t *testing.T) {
	s := &surface{scale: 2}

	if err := s.handleRemoteResize(320, 180); err != nil {
		t.Fatalf("handleRemoteResize returned error: %v", err)
	}

	evs := s.PollEvents()
	if len(evs) != 1 {
		t.Fatalf("expected 1 resize event, got %d", len(evs))
	}

	resize, ok := evs[0].(event.ResizeEvent)
	if !ok {
		t.Fatalf("expected ResizeEvent, got %T", evs[0])
	}
	if resize.Width != 640 || resize.Height != 360 {
		t.Fatalf("expected 640x360 pixel resize, got %dx%d", resize.Width, resize.Height)
	}
}

func TestScaleInputEventExpandsPointerCoordinates(t *testing.T) {
	button, ok := scaleInputEvent(event.ButtonEvent{X: 12, Y: 8}, 2).(event.ButtonEvent)
	if !ok {
		t.Fatalf("expected ButtonEvent after scaling, got %T", button)
	}
	if button.X != 24 || button.Y != 16 {
		t.Fatalf("expected scaled button coords 24x16, got %.0fx%.0f", button.X, button.Y)
	}

	cursor, ok := scaleInputEvent(event.CursorEvent{X: 5, Y: 7}, 1.5).(event.CursorEvent)
	if !ok {
		t.Fatalf("expected CursorEvent after scaling, got %T", cursor)
	}
	if cursor.X != 7.5 || cursor.Y != 10.5 {
		t.Fatalf("expected scaled cursor coords 7.5x10.5, got %.1fx%.1f", cursor.X, cursor.Y)
	}
}
