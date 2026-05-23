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

// Package app provides top-level application lifecycle orchestration for the
// widget framework.
//
// Set Options fields before calling Run. Run opens a window, drives the
// build→layout→paint cycle each frame, and forwards events to the widget.Frame.
//
// Usage:
//
//	app.Options.Title = "My App"
//	app.Options.Width = 480
//	app.Options.Height = 560
//	app.Run(MyRootWidget{})
//
// For custom event handling, set EventHandler before Run:
//
//	app.EventHandler = func(e event.Event) {
//	    // handle e, call app.DefaultHandler(e) for built-in behaviour
//	}
package app

import (
	"fmt"
	"image"
	"os"
	"strconv"
	"sync"
	"time"

	"avyos.dev/pkg/graphics/backend"
	"avyos.dev/pkg/graphics/canvas"
	"avyos.dev/pkg/graphics/event"
	"avyos.dev/pkg/graphics/geom"
	"avyos.dev/pkg/graphics/theme"
	"avyos.dev/pkg/graphics/widget"
	"avyos.dev/pkg/logger"
)

var log = logger.New("graphics/app")

// Options controls application window creation. Set fields before calling Run.
var Options = struct {
	Title      string
	Width      int
	Height     int
	Fullscreen bool
	Resizable  bool
	Layer      *backend.LayerSurfaceOptions
	Backend    backend.Backend
	// Scale is the logical-to-physical render scale.
	// Values <= 0 use the backend default.
	Scale float64
	// UseFrameDamageHints enables widget-framework-derived partial repaint
	// hints when no explicit DamageProvider is set. Disabled by default
	// because some apps paint visual effects beyond their layout bounds.
	UseFrameDamageHints bool
	// Theme overrides the default theme (theme.Light() if nil).
	Theme *theme.ThemeData
}{
	Title:     "App",
	Width:     800,
	Height:    600,
	Resizable: true,
	Scale:     0,
}

// BeforeEvents is called at the start of each event loop iteration, before
// polling for new events. Use this to drain background work queues.
var BeforeEvents func()

// EventHandler, if non-nil, is called for each incoming event instead of the
// default handling. Call DefaultHandler(e) within your handler to run the
// built-in behaviour for events you do not handle yourself.
var EventHandler func(e event.Event)

// AfterFrame is called after each frame is presented. The frame argument is
// the widget.Frame used for rendering and remains valid until Run returns.
var AfterFrame func(f *widget.Frame)

// DamageProvider, if non-nil, is called before each frame is rendered.
// It should return the minimal bounding rectangle (in physical pixels) of all
// pixels that semantically changed this frame. When the returned rectangle is
// non-empty, only that region is cleared and re-rendered; the canvas clip
// automatically restricts all draw operations (including DrawImage of client
// buffers) to the hint rect, so the display backend only blits the changed
// pixels. Return an empty rectangle to trigger a full repaint.
var DamageProvider func() image.Rectangle

func resolveDamageRect(f *widget.Frame) image.Rectangle {
	if DamageProvider != nil {
		return DamageProvider()
	}
	if Options.UseFrameDamageHints && f != nil {
		return f.DamageHint()
	}
	return image.Rectangle{}
}

func syncCursorShape(f *widget.Frame, surf backend.Surface) {
	if f == nil || surf == nil {
		return
	}
	setter, ok := surf.(backend.CursorSetter)
	if !ok {
		return
	}
	_ = setter.SetCursorShape(f.CursorShape())
}

// package-level run state, valid only while Run is executing.
var (
	stateMu      sync.Mutex
	currentFrame *widget.Frame
	currentBack  backend.Backend
	currentScale float64
	currentSurf  backend.Surface
	stopped      bool
	injectedMu   sync.Mutex
	injected     []event.Event
)

type frameTraceConfig struct {
	enabled bool
	every   int
}

type frameTrace struct {
	every         int
	frames        int
	beginTotal    time.Duration
	renderTotal   time.Duration
	presentTotal  time.Duration
	afterTotal    time.Duration
	frameTotal    time.Duration
	partialFrames int
	fullFrames    int
	renderStats   canvas.RenderStats
	presentStats  backend.PresentStats
}

func loadFrameTraceConfig() frameTraceConfig {
	return parseFrameTraceConfig(os.Getenv)
}

func parseFrameTraceConfig(getenv func(string) string) frameTraceConfig {
	cfg := frameTraceConfig{every: 60}
	if enabled, err := strconv.ParseBool(getenv("AVYOS_GRAPHICS_TRACE_FRAMES")); err == nil {
		cfg.enabled = enabled
	}
	if every := getenv("AVYOS_GRAPHICS_TRACE_EVERY"); every != "" {
		if n, err := strconv.Atoi(every); err == nil && n > 0 {
			cfg.every = n
		}
	}
	return cfg
}

func newFrameTrace(cfg frameTraceConfig) *frameTrace {
	if !cfg.enabled {
		return nil
	}
	return &frameTrace{every: cfg.every}
}

func (t *frameTrace) record(beginDur, renderDur, presentDur, afterDur, totalDur time.Duration, partial bool, stats canvas.RenderStats, presentStats backend.PresentStats) {
	if t == nil {
		return
	}
	t.frames++
	t.beginTotal += beginDur
	t.renderTotal += renderDur
	t.presentTotal += presentDur
	t.afterTotal += afterDur
	t.frameTotal += totalDur
	t.renderStats.FillRectCalls += stats.FillRectCalls
	t.renderStats.FillRectTime += stats.FillRectTime
	t.renderStats.FillRoundedRectCalls += stats.FillRoundedRectCalls
	t.renderStats.FillRoundedRectTime += stats.FillRoundedRectTime
	t.renderStats.StrokeRoundedRectCalls += stats.StrokeRoundedRectCalls
	t.renderStats.StrokeRoundedRectTime += stats.StrokeRoundedRectTime
	t.renderStats.DrawTextCalls += stats.DrawTextCalls
	t.renderStats.DrawTextTime += stats.DrawTextTime
	t.renderStats.DrawRuneCalls += stats.DrawRuneCalls
	t.renderStats.DrawRuneTime += stats.DrawRuneTime
	t.renderStats.DrawImageCalls += stats.DrawImageCalls
	t.renderStats.DrawImageTime += stats.DrawImageTime
	t.renderStats.ClearCalls += stats.ClearCalls
	t.renderStats.ClearTime += stats.ClearTime
	t.presentStats.Blit += presentStats.Blit
	t.presentStats.Submit += presentStats.Submit
	t.presentStats.Wait += presentStats.Wait
	if partial {
		t.partialFrames++
	} else {
		t.fullFrames++
	}
	if t.frames < t.every {
		return
	}
	avgBegin := t.beginTotal / time.Duration(t.frames)
	avgRender := t.renderTotal / time.Duration(t.frames)
	avgPresent := t.presentTotal / time.Duration(t.frames)
	avgAfter := t.afterTotal / time.Duration(t.frames)
	avgTotal := t.frameTotal / time.Duration(t.frames)
	fps := 0.0
	if avgTotal > 0 {
		fps = float64(time.Second) / float64(avgTotal)
	}
	log.Info(
		"frame trace avg frames=%d total=%s render=%s present=%s begin=%s after=%s fps=%.1f partial=%d full=%d env=%s/%s",
		t.frames,
		formatFrameDuration(avgTotal),
		formatFrameDuration(avgRender),
		formatFrameDuration(avgPresent),
		formatFrameDuration(avgBegin),
		formatFrameDuration(avgAfter),
		fps,
		t.partialFrames,
		t.fullFrames,
		"AVYOS_GRAPHICS_TRACE_FRAMES",
		"AVYOS_GRAPHICS_TRACE_EVERY",
	)
	if t.presentStats.Blit > 0 || t.presentStats.Submit > 0 || t.presentStats.Wait > 0 {
		log.Info(
			"frame trace present avg blit=%s submit=%s wait=%s",
			formatFrameDuration(t.presentStats.Blit/time.Duration(t.frames)),
			formatFrameDuration(t.presentStats.Submit/time.Duration(t.frames)),
			formatFrameDuration(t.presentStats.Wait/time.Duration(t.frames)),
		)
	}
	if t.renderStats.FillRoundedRectCalls > 0 || t.renderStats.StrokeRoundedRectCalls > 0 || t.renderStats.DrawTextCalls > 0 || t.renderStats.ClearCalls > 0 {
		log.Info(
			"frame trace draw avg fill_rect=%s/%d fill_rr=%s/%d stroke_rr=%s/%d text=%s/%d rune=%s/%d image=%s/%d clear=%s/%d",
			formatFrameDuration(t.renderStats.FillRectTime/time.Duration(t.frames)),
			t.renderStats.FillRectCalls/t.frames,
			formatFrameDuration(t.renderStats.FillRoundedRectTime/time.Duration(t.frames)),
			t.renderStats.FillRoundedRectCalls/t.frames,
			formatFrameDuration(t.renderStats.StrokeRoundedRectTime/time.Duration(t.frames)),
			t.renderStats.StrokeRoundedRectCalls/t.frames,
			formatFrameDuration(t.renderStats.DrawTextTime/time.Duration(t.frames)),
			t.renderStats.DrawTextCalls/t.frames,
			formatFrameDuration(t.renderStats.DrawRuneTime/time.Duration(t.frames)),
			t.renderStats.DrawRuneCalls/t.frames,
			formatFrameDuration(t.renderStats.DrawImageTime/time.Duration(t.frames)),
			t.renderStats.DrawImageCalls/t.frames,
			formatFrameDuration(t.renderStats.ClearTime/time.Duration(t.frames)),
			t.renderStats.ClearCalls/t.frames,
		)
	}
	*t = frameTrace{every: t.every}
}

func formatFrameDuration(d time.Duration) string {
	return fmt.Sprintf("%.3fms", float64(d)/float64(time.Millisecond))
}

// CurrentFrame returns the widget.Frame for the currently running app.
// Returns nil if Run is not executing.
func CurrentFrame() *widget.Frame {
	stateMu.Lock()
	defer stateMu.Unlock()
	return currentFrame
}

// CurrentBackend returns the backend for the currently running app.
func CurrentBackend() backend.Backend {
	stateMu.Lock()
	defer stateMu.Unlock()
	return currentBack
}

// Stop signals the event loop to exit cleanly. Safe to call from any goroutine.
func Stop() {
	stateMu.Lock()
	stopped = true
	stateMu.Unlock()
}

// AddEvent injects an event into the loop queue. Safe to call from any goroutine.
func AddEvent(e event.Event) {
	injectedMu.Lock()
	injected = append(injected, e)
	injectedMu.Unlock()
}

// DefaultHandler runs the built-in event dispatch for e.
// It forwards pointer, key, and resize events to the current widget.Frame.
func DefaultHandler(e event.Event) {
	stateMu.Lock()
	f := currentFrame
	scale := currentScale
	surf := currentSurf
	stateMu.Unlock()

	if f == nil {
		return
	}

	switch ev := e.(type) {
	case event.CloseEvent:
		Stop()
	case event.BlurEvent:
		f.ClearPointerState()
	case event.TextInputEvent:
		f.HandleKey(ev)
	case event.KeyEvent:
		if ev.Down && ev.Key == event.KeyEscape {
			Stop()
			return
		}
		f.HandleKey(ev)
	case event.ResizeEvent:
		if ev.Width > 0 && ev.Height > 0 {
			if surf != nil {
				_ = surf.Resize(ev.Width, ev.Height)
			}
			stateMu.Lock()
			currentFrame.Resize(int(float64(ev.Width)/scale), int(float64(ev.Height)/scale))
			stateMu.Unlock()
		}
	case event.ButtonEvent:
		pos := geom.Pt(ev.X/scale, ev.Y/scale)
		if ev.Down {
			f.HandlePointerDown(pos, ev.Button)
		} else {
			f.HandlePointerUp(pos, ev.Button)
		}
	case event.CursorEvent:
		f.HandlePointerMove(geom.Pt(ev.X/scale, ev.Y/scale))
	case event.ScrollEvent:
		f.HandleScroll(geom.Pt(ev.X/scale, ev.Y/scale), ev.DX/scale, ev.DY/scale)
	}
}

// Run initialises the backend, opens a window, and enters the event loop.
// Returns nil on clean exit or a wrapped error.
func Run(root widget.Widget) error {
	b := Options.Backend
	if b == nil {
		b = DefaultBackend()
	}

	th := Options.Theme
	if th == nil {
		th = theme.Light()
	}
	w := Options.Width
	if Options.Layer == nil && w == 0 {
		w = 800
	}
	h := Options.Height
	if Options.Layer == nil && h == 0 {
		h = 600
	}
	scale := Options.Scale
	if scale <= 0 {
		if provider, ok := b.(backend.ScaleProvider); ok {
			scale = provider.Scale()
		}
	}
	if scale <= 0 {
		scale = 1.0
	}

	if err := b.Init(); err != nil {
		return fmt.Errorf("app: backend init: %w", err)
	}
	defer b.Shutdown()

	win, err := b.NewWindow(backend.WindowOptions{
		Title:      Options.Title,
		Width:      w,
		Height:     h,
		Scale:      scale,
		Fullscreen: Options.Fullscreen,
		Resizable:  Options.Resizable,
		Layer:      Options.Layer,
	})
	if err != nil {
		return fmt.Errorf("app: new window: %w", err)
	}
	defer win.Close()

	physW, physH := win.Size()
	logW := float64(physW) / scale
	logH := float64(physH) / scale

	surf := win.Surface()
	f := widget.NewFrame(th, geom.Sz(logW, logH))
	f.Scale = scale
	trace := newFrameTrace(loadFrameTraceConfig())
	if trace != nil {
		if err := logger.SetupLog(); err != nil {
			log.Warn("failed to set up file logging for frame tracing: %v", err)
		}
		log.Info(
			"frame tracing enabled every=%d env=%s env_every=%s",
			trace.every,
			"AVYOS_GRAPHICS_TRACE_FRAMES",
			"AVYOS_GRAPHICS_TRACE_EVERY",
		)
	}

	stateMu.Lock()
	currentFrame = f
	currentBack = b
	currentScale = scale
	currentSurf = surf
	stopped = false
	stateMu.Unlock()
	defer func() {
		stateMu.Lock()
		currentFrame = nil
		currentBack = nil
		currentSurf = nil
		stateMu.Unlock()
	}()

	const idleSleep = 16 * time.Millisecond

	dispatch := func(e event.Event) {
		if EventHandler != nil {
			EventHandler(e)
		} else {
			DefaultHandler(e)
		}
	}

	for {
		stateMu.Lock()
		done := stopped
		stateMu.Unlock()
		if done {
			return nil
		}

		if BeforeEvents != nil {
			BeforeEvents()
		}
		widget.PumpBackgroundWork()

		// Poll events from the surface or backend.
		var evs []event.Event
		if ep, ok := surf.(backend.EventPoller); ok {
			evs = ep.PollEvents()
		} else {
			evs, _ = b.PollEvents()
		}
		for _, e := range evs {
			dispatch(e)
		}

		// Drain any externally injected events.
		injectedMu.Lock()
		pending := injected
		injected = nil
		injectedMu.Unlock()
		for _, e := range pending {
			dispatch(e)
		}

		// Pump work enqueued by gesture callbacks and sutra goroutines during
		// event dispatch so mutations land before the dirty check this frame.
		widget.PumpBackgroundWork()
		syncCursorShape(f, surf)

		// Re-check stop flag after event processing.
		stateMu.Lock()
		done = stopped
		stateMu.Unlock()
		if done {
			return nil
		}

		if f.IsDirty() {
			frameStart := time.Now()
			beginStart := time.Now()
			c, err := surf.Begin()
			if err != nil {
				return fmt.Errorf("app: surface begin: %w", err)
			}
			if rs, ok := c.(canvas.RenderStatsProvider); ok {
				rs.SetRenderStatsEnabled(true)
				rs.ResetRenderStats()
			}
			beginDur := time.Since(beginStart)
			// Reset damage tracking so dirty reflects only this frame's writes.
			if dt, ok := c.(canvas.DirtyTracker); ok {
				dt.ResetDirty()
			}
			// Query the damage hint before rendering. A non-empty hint means
			// only that physical-pixel region changed; clip the canvas to it so
			// draw operations (including DrawImage of client buffers) only touch
			// the dirty pixels and the canvas dirty rect stays tight. An empty
			// hint means a full repaint is needed.
			partial := resolveDamageRect(f)
			renderStart := time.Now()
			if !partial.Empty() {
				pr := geom.NewRect(
					float64(partial.Min.X), float64(partial.Min.Y),
					float64(partial.Dx()), float64(partial.Dy()),
				)
				c.Save()
				c.ClipRect(pr)
				c.SetFillColor(th.ColorScheme.Background)
				c.FillRect(pr)
				if scale != 1.0 {
					c.Scale(scale, scale)
				}
				f.Render(root, c)
				c.Restore()
			} else {
				c.Clear(th.ColorScheme.Background)
				if scale != 1.0 {
					c.Save()
					c.Scale(scale, scale)
				}
				f.Render(root, c)
				if scale != 1.0 {
					c.Restore()
				}
			}
			renderDur := time.Since(renderStart)
			presentStart := time.Now()
			if err := surf.Present(c); err != nil {
				return fmt.Errorf("app: surface present: %w", err)
			}
			presentDur := time.Since(presentStart)
			afterStart := time.Now()
			if AfterFrame != nil {
				AfterFrame(f)
			}
			syncCursorShape(f, surf)
			afterDur := time.Since(afterStart)
			stats := canvas.RenderStats{}
			if rs, ok := c.(canvas.RenderStatsProvider); ok {
				stats = rs.RenderStats()
			}
			presentStats := backend.PresentStats{}
			if ps, ok := surf.(backend.PresentStatsProvider); ok {
				presentStats = ps.PresentStats()
			}
			trace.record(beginDur, renderDur, presentDur, afterDur, time.Since(frameStart), !partial.Empty(), stats, presentStats)
		} else {
			time.Sleep(idleSleep)
		}
	}
}
