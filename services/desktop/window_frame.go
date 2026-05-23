// Copyright (c) 2026 Manjeet Singh <itsmanjeet1998@gmail.com>.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, version 3.
//
// This program is distributed in the hope that it will be useful, but
// WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the GNU
// General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with this program. If not, see <http://www.gnu.org/licenses/>.

package main

import (
	"math"

	"avyos.dev/pkg/graphics/canvas"
	"avyos.dev/pkg/graphics/event"
	"avyos.dev/pkg/graphics/geom"
	"avyos.dev/pkg/graphics/layout"
	"avyos.dev/pkg/graphics/paint"
	"avyos.dev/pkg/graphics/widget"
)

const resizeHandleSize = 6.0

// ResizeEdge identifies which window edge or corner is being dragged.
type ResizeEdge int

const (
	ResizeN ResizeEdge = iota
	ResizeS
	ResizeE
	ResizeW
	ResizeNE
	ResizeNW
	ResizeSE
	ResizeSW
)

// WindowFrame is a Buildable that renders chrome (titlebar + border) around a
// remote application window surface.
type WindowFrame struct {
	MW         *ManagedWindow
	Focused    bool
	OnFocus    func()
	OnClose    func()
	OnMinimize func()
	// OnMaximize toggles between maximized and restored.
	OnMaximize func()
	OnMove     func(dx, dy float64)
	OnResize   func(edge ResizeEdge, dx, dy float64)
}

func (wf WindowFrame) Build(ctx widget.BuildContext) widget.Widget {
	th := ctx.Theme
	win := wf.MW.Win
	mw := wf.MW

	// ── titlebar label ────────────────────────────────────────────────────
	titleSt := th.TextTheme.LabelMedium
	if wf.Focused {
		titleSt.Color = th.ColorScheme.OnSurface
	} else {
		titleSt.Color = th.ColorScheme.OnSurfaceVariant
	}

	label := win.AppName
	if label == "" {
		label = win.Title
	}
	if label == "" {
		label = "Window"
	}

	// ── close button ──────────────────────────────────────────────────────
	closeSt := th.TextTheme.LabelMedium
	closeBtn := widget.GestureDetector{
		OnTap: wf.OnClose,
		Builder: func(state widget.InteractionState) widget.Widget {
			if state.Hovered {
				closeSt.Color = th.ColorScheme.Error
			} else {
				closeSt.Color = th.ColorScheme.OnSurfaceVariant
			}
			return widget.Container{
				Width: 20, Height: 20,
				Padding: layout.All(2),
				Child:   widget.Text{Content: "×", Style: &closeSt},
			}
		},
	}

	// ── maximize / restore button ─────────────────────────────────────────
	maxSt := th.TextTheme.LabelMedium
	maxIcon := "□"
	if mw.Maximized {
		maxIcon = "⧉"
	}
	maxBtn := widget.GestureDetector{
		OnTap: wf.OnMaximize,
		Builder: func(state widget.InteractionState) widget.Widget {
			if state.Hovered {
				maxSt.Color = th.ColorScheme.OnSurface
			} else {
				maxSt.Color = th.ColorScheme.OnSurfaceVariant
			}
			return widget.Container{
				Width: 20, Height: 20,
				Padding: layout.All(2),
				Child:   widget.Text{Content: maxIcon, Style: &maxSt},
			}
		},
	}

	// ── minimize button ───────────────────────────────────────────────────
	minSt := th.TextTheme.LabelMedium
	minBtn := widget.GestureDetector{
		OnTap: wf.OnMinimize,
		Builder: func(state widget.InteractionState) widget.Widget {
			if state.Hovered {
				minSt.Color = th.ColorScheme.OnSurface
			} else {
				minSt.Color = th.ColorScheme.OnSurfaceVariant
			}
			return widget.Container{
				Width: 20, Height: 20,
				Padding: layout.All(2),
				Child:   widget.Text{Content: "−", Style: &minSt},
			}
		},
	}

	// ── accent dot ────────────────────────────────────────────────────────
	var accentDot widget.Widget = widget.SizedBox{Width: 8}
	if wf.Focused && win.Accent.A > 0 {
		accentDot = widget.Container{
			Width: 8, Height: 8,
			Fill: win.Accent, Radius: 4,
		}
	}

	// ── titlebar drag ─────────────────────────────────────────────────────
	titlebar := widget.GestureDetector{
		Cursor: event.CursorMove,
		OnPointerDownLocal: func(local geom.Point) {
			mw.lastDragPos = geom.Pt(mw.X+local.X, mw.Y+local.Y)
			mw.dragging = true
			if wf.OnFocus != nil {
				wf.OnFocus()
			}
		},
		OnDragMove: func(global geom.Point) {
			if !mw.dragging || mw.Maximized {
				return
			}
			dx := global.X - mw.lastDragPos.X
			dy := global.Y - mw.lastDragPos.Y
			mw.lastDragPos = global
			if wf.OnMove != nil {
				wf.OnMove(dx, dy)
			}
		},
		OnDragEnd: func() { mw.dragging = false },
		Builder: func(state widget.InteractionState) widget.Widget {
			barFill := th.ColorScheme.SurfaceContainer
			if wf.Focused {
				barFill = th.ColorScheme.Background
			}
			return windowCornerClip{
				Radius: windowRadius - borderWidth,
				Top:    true,
				Child: widget.Container{
					Height:  titlebarHeight,
					Fill:    barFill,
					Padding: layout.Symmetric(8, 6),
					Child: widget.Row{
						CrossAxisAlignment: layout.CrossCenter,
						Children: []widget.Widget{
							accentDot,
							widget.SizedBox{Width: 6},
							widget.Expanded{Child: widget.Text{Content: label, Style: &titleSt}},
							widget.SizedBox{Width: 4},
							minBtn,
							widget.SizedBox{Width: 2},
							maxBtn,
							widget.SizedBox{Width: 2},
							closeBtn,
						},
					},
				},
			}
		},
	}

	// ── window content area with input forwarding ─────────────────────────
	contentWidget := widget.GestureDetector{
		CursorFunc: func() event.CursorShape {
			return mw.CursorShape
		},
		OnTapDown: func() {
			if wf.OnFocus != nil {
				wf.OnFocus()
			}
		},
		OnPointerDownLocal: func(local geom.Point) {
			_ = win.SendInput(event.Encode(event.ButtonEvent{
				Button: event.ButtonLeft, X: local.X, Y: local.Y, Down: true,
			}))
		},
		OnPointerUpLocal: func(local geom.Point) {
			_ = win.SendInput(event.Encode(event.ButtonEvent{
				Button: event.ButtonLeft, X: local.X, Y: local.Y, Down: false,
			}))
		},
		OnPointerMoveLocal: func(local geom.Point) {
			_ = win.SendInput(event.Encode(event.CursorEvent{X: local.X, Y: local.Y}))
		},
		OnDragMove: func(global geom.Point) {
			cx, cy, _, _ := mw.ContentRect()
			_ = win.SendInput(event.Encode(event.CursorEvent{
				X: global.X - cx, Y: global.Y - cy,
			}))
		},
		OnScroll: func(dx, dy float64) {
			_ = win.SendInput(event.Encode(event.ScrollEvent{DX: dx, DY: dy}))
		},
		Child: windowCornerClip{
			Radius: windowRadius - borderWidth,
			Bottom: true,
			Child:  win.Build(ctx),
		},
	}

	// ── chrome container ──────────────────────────────────────────────────
	chrome := widget.GestureDetector{
		OnTapDown: func() {
			if wf.OnFocus != nil {
				wf.OnFocus()
			}
		},
		Child: widget.Container{
			Fill:          th.ColorScheme.Surface,
			Border:        th.ColorScheme.Outline,
			BorderWidth:   borderWidth,
			Radius:        windowRadius,
			Shadow:        th.ColorScheme.Shadow,
			ShadowBlur:    th.Shadow.MD.Blur,
			ShadowOffsetY: th.Shadow.MD.OffsetY,
			Padding:       layout.All(borderWidth),
			Child: widget.Column{
				Children: []widget.Widget{
					titlebar,
					widget.Expanded{Child: contentWidget},
				},
			},
		},
	}

	// Maximized windows have no resize handles.
	if mw.Maximized {
		return chrome
	}

	// ── resize handles (overlaid at the chrome edges) ─────────────────────
	rs := func(edge ResizeEdge) widget.Widget {
		return makeResizeHandle(edge, mw, wf.OnFocus, wf.OnResize)
	}
	p := widget.Ptr

	return widget.Stack{
		Children: []widget.Widget{
			chrome,
			// edges
			widget.Positioned{Top: p(0), Left: p(resizeHandleSize), Right: p(resizeHandleSize), Height: p(resizeHandleSize), Child: rs(ResizeN)},
			widget.Positioned{Bottom: p(0), Left: p(resizeHandleSize), Right: p(resizeHandleSize), Height: p(resizeHandleSize), Child: rs(ResizeS)},
			widget.Positioned{Left: p(0), Top: p(resizeHandleSize), Bottom: p(resizeHandleSize), Width: p(resizeHandleSize), Child: rs(ResizeW)},
			widget.Positioned{Right: p(0), Top: p(resizeHandleSize), Bottom: p(resizeHandleSize), Width: p(resizeHandleSize), Child: rs(ResizeE)},
			// corners
			widget.Positioned{Left: p(0), Top: p(0), Width: p(resizeHandleSize), Height: p(resizeHandleSize), Child: rs(ResizeNW)},
			widget.Positioned{Right: p(0), Top: p(0), Width: p(resizeHandleSize), Height: p(resizeHandleSize), Child: rs(ResizeNE)},
			widget.Positioned{Left: p(0), Bottom: p(0), Width: p(resizeHandleSize), Height: p(resizeHandleSize), Child: rs(ResizeSW)},
			widget.Positioned{Right: p(0), Bottom: p(0), Width: p(resizeHandleSize), Height: p(resizeHandleSize), Child: rs(ResizeSE)},
		},
	}
}

type windowCornerClip struct {
	Radius float64
	Top    bool
	Bottom bool
	Child  widget.Widget
}

func (w windowCornerClip) RenderChildren(c layout.BoxConstraints, pctx *paint.Context, offset geom.Point, cr widget.ChildRenderer) geom.Size {
	if w.Child == nil {
		return c.Smallest()
	}

	sz := cr.Measure(w.Child, c, "child")
	if pctx == nil || w.Radius <= 0 {
		cr.Render(w.Child, c, offset, "child")
		return sz
	}

	pixelSaver, ok := pctx.Canvas.(canvas.PixelSaver)
	if !ok {
		cr.Render(w.Child, c, offset, "child")
		return sz
	}

	radius := math.Min(w.Radius, math.Min(sz.Width/2, sz.Height))
	if radius <= 0 {
		cr.Render(w.Child, c, offset, "child")
		return sz
	}

	cornerSize := math.Ceil(radius)
	type savedCorner struct {
		rect   geom.Rect
		saved  []byte
		center geom.Point
	}
	corners := make([]savedCorner, 0, 4)
	if w.Top {
		topLeft := geom.NewRect(offset.X, offset.Y, cornerSize, cornerSize)
		topRight := geom.NewRect(offset.X+sz.Width-cornerSize, offset.Y, cornerSize, cornerSize)
		corners = append(corners,
			savedCorner{
				rect:   topLeft,
				saved:  pixelSaver.SavePixels(topLeft),
				center: geom.Pt(offset.X+radius, offset.Y+radius),
			},
			savedCorner{
				rect:   topRight,
				saved:  pixelSaver.SavePixels(topRight),
				center: geom.Pt(offset.X+sz.Width-radius, offset.Y+radius),
			},
		)
	}
	if w.Bottom {
		bottomLeft := geom.NewRect(offset.X, offset.Y+sz.Height-cornerSize, cornerSize, cornerSize)
		bottomRight := geom.NewRect(offset.X+sz.Width-cornerSize, offset.Y+sz.Height-cornerSize, cornerSize, cornerSize)
		corners = append(corners,
			savedCorner{
				rect:   bottomLeft,
				saved:  pixelSaver.SavePixels(bottomLeft),
				center: geom.Pt(offset.X+radius, offset.Y+sz.Height-radius),
			},
			savedCorner{
				rect:   bottomRight,
				saved:  pixelSaver.SavePixels(bottomRight),
				center: geom.Pt(offset.X+sz.Width-radius, offset.Y+sz.Height-radius),
			},
		)
	}

	cr.Render(w.Child, c, offset, "child")

	for _, corner := range corners {
		restoreRoundedCornerPixels(pctx.Canvas, corner.rect, corner.saved, corner.center, radius)
	}
	return sz
}

func restoreRoundedCornerPixels(cv canvas.Canvas, rect geom.Rect, saved []byte, center geom.Point, radius float64) {
	if len(saved) == 0 || rect.Empty() || radius <= 0 {
		return
	}

	pixels := cv.Pixels()
	canvasSize := cv.Size()
	canvasWidth := int(math.Round(canvasSize.Width))
	canvasHeight := int(math.Round(canvasSize.Height))
	if canvasWidth <= 0 || canvasHeight <= 0 {
		return
	}

	stride := canvasWidth * 4
	if stride <= 0 || len(pixels) < stride*canvasHeight {
		return
	}

	minX := int(math.Max(0, math.Floor(rect.Min.X)))
	minY := int(math.Max(0, math.Floor(rect.Min.Y)))
	maxX := int(math.Min(canvasSize.Width, math.Ceil(rect.Max.X)))
	maxY := int(math.Min(canvasSize.Height, math.Ceil(rect.Max.Y)))
	width := maxX - minX
	height := maxY - minY
	if width <= 0 || height <= 0 || len(saved) < width*height*4 {
		return
	}

	r2 := radius * radius
	for y := 0; y < height; y++ {
		py := float64(minY+y) + 0.5
		for x := 0; x < width; x++ {
			px := float64(minX+x) + 0.5
			dx := px - center.X
			dy := py - center.Y
			if dx*dx+dy*dy <= r2 {
				continue
			}

			dstOff := ((minY+y)*canvasWidth + (minX + x)) * 4
			srcOff := (y*width + x) * 4
			copy(pixels[dstOff:dstOff+4], saved[srcOff:srcOff+4])
		}
	}
}

// makeResizeHandle builds a transparent gesture widget for one resize edge/corner.
// It blocks the first drag-move tick (to establish the reference position) then
// fires onResize with pixel deltas on every subsequent move.
func makeResizeHandle(edge ResizeEdge, mw *ManagedWindow, onFocus func(), onResize func(ResizeEdge, float64, float64)) widget.Widget {
	return widget.GestureDetector{
		Cursor: resizeEdgeCursor(edge),
		OnPointerDownLocal: func(_ geom.Point) {
			mw.resizing = true
			mw.resizeDragStarted = false
			if onFocus != nil {
				onFocus()
			}
		},
		OnDragMove: func(global geom.Point) {
			if !mw.resizing {
				return
			}
			if !mw.resizeDragStarted {
				mw.resizeDragStarted = true
				mw.lastResizePos = global
				return
			}
			dx := global.X - mw.lastResizePos.X
			dy := global.Y - mw.lastResizePos.Y
			mw.lastResizePos = global
			if onResize != nil {
				onResize(edge, dx, dy)
			}
		},
		OnDragEnd: func() { mw.resizing = false },
		Child:     widget.Container{},
	}
}

func resizeEdgeCursor(edge ResizeEdge) event.CursorShape {
	switch edge {
	case ResizeN, ResizeS:
		return event.CursorResizeNS
	case ResizeE, ResizeW:
		return event.CursorResizeEW
	case ResizeNW, ResizeSE:
		return event.CursorResizeNWSE
	case ResizeNE, ResizeSW:
		return event.CursorResizeNESW
	default:
		return event.CursorDefault
	}
}
