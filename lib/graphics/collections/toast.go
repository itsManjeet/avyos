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

package collections

import (
	"sync"
	"time"

	"avyos.dev/lib/graphics/color"
	"avyos.dev/lib/graphics/layout"
	"avyos.dev/lib/graphics/theme"
	"avyos.dev/lib/graphics/widget"
)

// ─── ToastVariant ─────────────────────────────────────────────────────────────

// ToastVariant controls the accent colour of a toast notification.
type ToastVariant int

const (
	ToastDefault ToastVariant = iota // neutral, no semantic accent
	ToastSuccess                     // green accent for successful operations
	ToastWarning                     // orange accent for warnings
	ToastError                       // red accent for errors
	ToastInfo                        // blue accent for informational messages
)

// ─── ToastController ──────────────────────────────────────────────────────────

// ToastController manages a queue of transient notification toasts.
// Create one with [NewToastController] and share it between [ToastHost] and
// the components that trigger notifications.
type ToastController struct {
	mu       sync.Mutex
	entries  []toastEntry
	nextID   int
	watchers []func()
}

type toastEntry struct {
	id      int
	title   string
	message string
	variant ToastVariant
}

const defaultToastDuration = 4 * time.Second
const (
	defaultToastWidth  = 320.0
	expandedToastWidth = 560.0
	maxToastEntries    = 5
)

// NewToastController creates a ToastController.
func NewToastController() *ToastController { return &ToastController{} }

// Show adds a toast. The returned function dismisses it immediately.
func (tc *ToastController) Show(message string, variant ToastVariant) (dismiss func()) {
	return tc.show("", message, variant, defaultToastDuration)
}

// ShowFor shows a toast and auto-dismisses it after d.
func (tc *ToastController) ShowFor(message string, variant ToastVariant, d time.Duration) {
	tc.show("", message, variant, d)
}

// ShowDetailed adds a toast with a title and message.
func (tc *ToastController) ShowDetailed(title, message string, variant ToastVariant) (dismiss func()) {
	return tc.show(title, message, variant, defaultToastDuration)
}

// ShowDetailedFor shows a titled toast and auto-dismisses it after d.
func (tc *ToastController) ShowDetailedFor(title, message string, variant ToastVariant, d time.Duration) {
	tc.show(title, message, variant, d)
}

func (tc *ToastController) show(title, message string, variant ToastVariant, d time.Duration) (dismiss func()) {
	tc.mu.Lock()
	id := tc.nextID
	tc.nextID++
	tc.entries = append(tc.entries, toastEntry{id: id, title: title, message: message, variant: variant})
	if extra := len(tc.entries) - maxToastEntries; extra > 0 {
		tc.entries = append([]toastEntry(nil), tc.entries[extra:]...)
	}
	watchers := append([]func(){}, tc.watchers...)
	tc.mu.Unlock()
	for _, watcher := range watchers {
		if watcher != nil {
			watcher()
		}
	}
	dismiss = func() { tc.dismiss(id) }
	if d > 0 {
		go func() {
			time.Sleep(d)
			dismiss()
		}()
	}
	return dismiss
}

func (tc *ToastController) dismiss(id int) {
	tc.mu.Lock()
	for i, e := range tc.entries {
		if e.id == id {
			tc.entries = append(tc.entries[:i], tc.entries[i+1:]...)
			break
		}
	}
	watchers := append([]func(){}, tc.watchers...)
	tc.mu.Unlock()
	for _, watcher := range watchers {
		if watcher != nil {
			watcher()
		}
	}
}

func (tc *ToastController) snapshot() []toastEntry {
	tc.mu.Lock()
	defer tc.mu.Unlock()
	if len(tc.entries) == 0 {
		return nil
	}
	out := make([]toastEntry, len(tc.entries))
	copy(out, tc.entries)
	return out
}

func (tc *ToastController) Watch(fn func()) {
	tc.mu.Lock()
	tc.watchers = append(tc.watchers, fn)
	tc.mu.Unlock()
}

// ─── ToastHost ────────────────────────────────────────────────────────────────

// ToastHost renders its Child and paints active toasts from Controller in the
// bottom-right corner (or wherever there is room). It is non-modal; pointer
// events pass through the toast area.
//
// Place ToastHost near the root, typically inside [OverlayHost].
type ToastHost struct {
	Controller  *ToastController
	Child       widget.Widget
	RightInset  float64
	BottomInset float64
}

func (t ToastHost) CreateState() widget.State { return &toastHostState{} }

type toastHostState struct {
	widget.StateBase
	w ToastHost
}

func (s *toastHostState) UpdateWidget(w widget.Widget) {
	if v, ok := w.(ToastHost); ok {
		if v.Controller != nil && v.Controller != s.w.Controller {
			v.Controller.Watch(func() { s.SetState(nil) })
		}
		s.w = v
	}
}

func (s *toastHostState) InitState() {
	if s.w.Controller != nil {
		s.w.Controller.Watch(func() { s.SetState(nil) })
	}
}

func (s *toastHostState) Build(ctx widget.BuildContext) widget.Widget {
	entries := s.w.Controller.snapshot()

	child := s.w.Child
	if child == nil {
		child = widget.SizedBox{}
	}

	if len(entries) == 0 {
		return child
	}

	tc := s.w.Controller
	toastWidgets := make([]widget.Widget, 0, len(entries)*2)
	for i, e := range entries {
		if i > 0 {
			toastWidgets = append(toastWidgets, widget.SizedBox{Height: 10})
		}
		e := e
		dismiss := func() { tc.dismiss(e.id) }
		toastWidgets = append(toastWidgets, buildToastWidget(e, dismiss, ctx))
	}

	// Stack toasts vertically from bottom.
	stack := widget.Column{
		MainAxisAlignment:  layout.MainEnd,
		CrossAxisAlignment: layout.CrossEnd,
		Children:           toastWidgets,
	}

	return widget.Stack{
		Children: []widget.Widget{
			child,
			widget.Positioned{
				Right:  widget.Ptr(resolveToastInset(s.w.RightInset)),
				Bottom: widget.Ptr(resolveToastInset(s.w.BottomInset)),
				Width:  widget.Ptr(expandedToastWidth),
				Child:  stack,
			},
		},
	}
}

func buildToastWidget(e toastEntry, dismiss func(), ctx widget.BuildContext) widget.Widget {
	th := ctx.Theme

	accent, fg := toastColors(e.variant, ctx)

	titleSt := th.TextTheme.LabelMedium
	titleSt.Color = th.ColorScheme.OnSurface
	msgSt := th.TextTheme.BodyMedium
	msgSt.Color = fg

	return widget.GestureDetector{
		Builder: func(state widget.InteractionState) widget.Widget {
			width := defaultToastWidth
			if state.Hovered {
				width = expandedToastWidth
			}
			return widget.SizedBox{
				Width: width,
				Child: widget.Container{
					Fill:          th.ColorScheme.Surface,
					Border:        accent.WithAlpha(0.4),
					BorderWidth:   1,
					Radius:        th.Shape.XXLargeRadius,
					Shadow:        th.ColorScheme.Shadow,
					ShadowBlur:    th.Shadow.MD.Blur,
					ShadowOffsetY: th.Shadow.MD.OffsetY,
					Padding:       layout.Symmetric(th.Space.Unit(4), th.Space.Unit(3)),
					Child: widget.Row{
						CrossAxisAlignment: layout.CrossStart,
						Children: []widget.Widget{
							widget.Container{Fill: accent, Width: 4, Radius: 999},
							widget.SizedBox{Width: th.Space.Unit(3)},
							widget.Expanded{
								Child: buildToastContent(e, titleSt, msgSt),
							},
							widget.SizedBox{Width: th.Space.Unit(2)},
							widget.GestureDetector{
								OnTap: dismiss,
								Builder: func(state widget.InteractionState) widget.Widget {
									closeSt := th.TextTheme.LabelSmall
									if state.Hovered {
										closeSt.Color = th.ColorScheme.OnSurface
									} else {
										closeSt.Color = th.ColorScheme.OnSurfaceVariant
									}
									return widget.Text{Content: "×", Style: &closeSt}
								},
							},
						},
					},
				},
			}
		},
	}
}

func buildToastContent(e toastEntry, titleSt, msgSt theme.TextStyle) widget.Widget {
	if e.title == "" {
		return widget.Text{Content: e.message, Style: &msgSt}
	}

	children := []widget.Widget{
		widget.Text{Content: e.title, Style: &titleSt},
	}
	if e.message != "" {
		children = append(children,
			widget.SizedBox{Height: 4},
			widget.Text{Content: e.message, Style: &msgSt},
		)
	}
	return widget.Column{
		CrossAxisAlignment: layout.CrossStretch,
		MainAxisSize:       layout.MainMin,
		Children:           children,
	}
}

func resolveToastInset(v float64) float64 {
	if v <= 0 {
		return 16
	}
	return v
}

func toastColors(v ToastVariant, ctx widget.BuildContext) (accent, fg color.Color) {
	th := ctx.Theme
	switch v {
	case ToastSuccess:
		return th.ColorScheme.Success, th.ColorScheme.Success
	case ToastWarning:
		return th.ColorScheme.Warning, th.ColorScheme.Warning
	case ToastError:
		return th.ColorScheme.Error, th.ColorScheme.Error
	case ToastInfo:
		return th.ColorScheme.Info, th.ColorScheme.Info
	default:
		return th.ColorScheme.Outline, th.ColorScheme.OnSurface
	}
}
