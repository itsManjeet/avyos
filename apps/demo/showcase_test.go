package main

import (
	"testing"

	"avyos.dev/pkg/graphics/canvas/pixbuf"
	"avyos.dev/pkg/graphics/geom"
	"avyos.dev/pkg/graphics/theme"
	"avyos.dev/pkg/graphics/widget"
)

type testShowcaseRoot struct {
	state *ShowcaseState
}

func (r testShowcaseRoot) Build(ctx widget.BuildContext) widget.Widget {
	return r.state.Build(ctx)
}

func TestShowcaseScrollSectionRendersAcrossFrames(t *testing.T) {
	state := &ShowcaseState{}
	state.InitState()
	state.section = 5

	frame := widget.NewFrame(theme.Light(), geom.Sz(1280, 800))
	canvas := pixbuf.NewCanvas(1280, 800)
	root := testShowcaseRoot{state: state}

	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("render panic: %v", r)
		}
	}()

	frame.Render(root, canvas)
	widget.PumpBackgroundWork()
	frame.Render(root, canvas)
	widget.PumpBackgroundWork()
	frame.Render(root, canvas)
}
