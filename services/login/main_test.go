package main

import (
	"image"
	"testing"

	"avyos.dev/pkg/graphics/geom"
)

func TestLoginEnsureBackgroundCachesScaledImage(t *testing.T) {
	src := image.NewRGBA(image.Rect(0, 0, 4, 4))
	state := &LoginState{backgroundSrc: src}

	state.ensureBackground(geom.Sz(2, 2))
	if state.background.Source == nil {
		t.Fatal("expected cached background image")
	}
	if got := state.background.Source.Bounds().Size(); got.X != 2 || got.Y != 2 {
		t.Fatalf("expected 2x2 cached background, got %dx%d", got.X, got.Y)
	}

	first := state.background.Source
	state.ensureBackground(geom.Sz(2, 2))
	if state.background.Source != first {
		t.Fatal("expected background cache to be reused for the same size")
	}
}

func TestLoginEnsureBackgroundReusesMatchingSource(t *testing.T) {
	src := image.NewRGBA(image.Rect(0, 0, 3, 5))
	state := &LoginState{backgroundSrc: src}

	state.ensureBackground(geom.Sz(3, 5))
	if state.background.Source != src {
		t.Fatal("expected matching-size source image to be reused directly")
	}
}
