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

// Package widget provides a minimal, composable UI widget system.
//
// # Architecture
//
// Every UI element is a Widget (any). There are three fundamental kinds:
//
//   - [RenderBox]: a leaf that knows how to measure itself and draw itself.
//   - [MultiChild]: a parent that controls the layout and rendering of
//     its children via [ChildRenderer].
//   - [Buildable]: a pure-function widget that decomposes into another Widget.
//
// Mutable widgets embed [StateBase] and implement [StatefulWidget].
// [GestureDetector] wraps any child and fires callbacks on pointer events.
// [Frame] drives the render loop for one window.
//
// # Typical usage
//
//	frame := widget.NewFrame(theme.Light(), geom.Sz(800, 600))
//	// each frame:
//	frame.Render(root, canvas)
//	// on events:
//	frame.HandlePointerDown(pos, button)
//	frame.HandlePointerUp(pos, button)
//	frame.HandlePointerMove(pos)
//	frame.HandleScroll(pos, dx, dy)
//	frame.HandleKey(event)
package widget

import (
	"image"
	"math"
	"reflect"
	"strconv"
	"sync"
	"time"
	"unicode/utf8"

	"avyos.dev/pkg/graphics/canvas"
	"avyos.dev/pkg/graphics/event"
	"avyos.dev/pkg/graphics/geom"
	"avyos.dev/pkg/graphics/layout"
	"avyos.dev/pkg/graphics/paint"
	"avyos.dev/pkg/graphics/theme"
)

// Widget is the base type for all UI elements.
type Widget any

// Buildable is a Widget whose Build method returns another Widget.
// Build must be a pure function of the widget's fields and ctx.
type Buildable interface {
	Widget
	Build(ctx BuildContext) Widget
}

// RenderBox is a leaf Widget that measures and paints itself directly.
type RenderBox interface {
	Widget
	// Layout returns the size this widget occupies under the given constraints.
	Layout(c layout.BoxConstraints) geom.Size
	// Paint draws the widget at offset within the previously-computed size.
	Paint(ctx *paint.Context, offset geom.Point, size geom.Size)
	// HitTest reports whether pos (in parent coordinates) hits this widget.
	HitTest(pos, offset geom.Point, size geom.Size) bool
}

// MultiChild is a Widget that lays out and renders its own children.
// It receives a [ChildRenderer] to measure and paint each child.
type MultiChild interface {
	Widget
	RenderChildren(c layout.BoxConstraints, pctx *paint.Context, offset geom.Point, cr ChildRenderer) geom.Size
}

// ChildRenderer is passed to [MultiChild] widgets to render each child.
type ChildRenderer interface {
	// Render lays out and paints w; returns its size.
	Render(w Widget, c layout.BoxConstraints, offset geom.Point, suffix string) geom.Size
	// Measure lays out w without painting; returns its size.
	Measure(w Widget, c layout.BoxConstraints, suffix string) geom.Size
}

// StatefulWidget creates a persistent [State] object on first render.
// The framework calls CreateState once and caches the result.
type StatefulWidget interface {
	Widget
	CreateState() State
}

// State holds mutable data for a [StatefulWidget].
// Implementations must embed [StateBase].
type State interface {
	Build(ctx BuildContext) Widget
}

// dirtyInjector lets the framework inject MarkDirty into StateBase.
type dirtyInjector interface {
	setMarkDirty(fn func())
}

// StateBase must be embedded in all [State] implementations.
// Call [StateBase.SetState] to mutate fields and trigger a rebuild.
type StateBase struct {
	markDirty func()
}

func (s *StateBase) setMarkDirty(fn func()) { s.markDirty = fn }

// SetState enqueues fn (and a subsequent dirty-mark) to run on the main
// goroutine during the next PumpBackgroundWork drain. Safe to call from any
// goroutine. fn may be nil if only a redraw is needed.
func (s *StateBase) SetState(fn func()) {
	markDirty := s.markDirty
	EnqueueWork(func() {
		if fn != nil {
			fn()
		}
		if markDirty != nil {
			markDirty()
		}
	})
}

// InteractionState describes the current pointer state over a widget.
type InteractionState struct {
	Hovered bool
	Pressed bool
}

// GestureDetector wraps a child widget and fires callbacks on pointer events.
// It may supply a Builder instead of (or in addition to) a static Child to
// rebuild its subtree based on hover/press state.
type GestureDetector struct {
	// Child is the wrapped widget. Either Child or Builder must be set.
	Child Widget
	// Builder rebuilds the child on each frame using current interaction state.
	Builder func(state InteractionState) Widget
	// Cursor is the preferred pointer shape while this detector is hovered.
	Cursor event.CursorShape
	// CursorFunc resolves the preferred pointer shape dynamically.
	CursorFunc func() event.CursorShape

	OnTap          func()
	OnTapDown      func()
	OnTapUp        func()
	OnPointerDown  func(pos geom.Point)
	OnHoverChanged func(hovered bool)
	OnPressChanged func(pressed bool)

	// Local callbacks receive coordinates relative to the widget's top-left.
	OnPointerDownLocal func(local geom.Point)
	OnPointerUpLocal   func(local geom.Point)
	OnPointerMoveLocal func(local geom.Point)

	// Drag callbacks fire while the widget is held, regardless of pointer position.
	OnDragMove func(global geom.Point)
	OnDragEnd  func()

	// OnScroll fires on wheel/trackpad scroll over this widget.
	OnScroll func(dx, dy float64)
}

// BuildContext is the environment passed to [Buildable.Build] and [State.Build].
type BuildContext struct {
	Theme      *theme.ThemeData
	ScreenSize geom.Size
	Scale      float64
	path       string
	frame      *Frame
}

// interactionEntry is a registered interactive region from the paint pass.
type interactionEntry struct {
	path               string
	rect               geom.Rect
	onTap              func()
	onTapDown          func()
	onTapUp            func()
	onPointerDown      func(geom.Point)
	onHoverChanged     func(bool)
	onPressChanged     func(bool)
	onPointerDownLocal func(geom.Point)
	onPointerUpLocal   func(geom.Point)
	onPointerMoveLocal func(geom.Point)
	onDragMove         func(geom.Point)
	onDragEnd          func()
	onScroll           func(dx, dy float64)
	cursor             event.CursorShape
	cursorFunc         func() event.CursorShape
}

// Frame manages the widget tree for one window.
// Create one at startup; call [Frame.Render] each frame and Handle* on events.
type Frame struct {
	Theme  *theme.ThemeData
	Screen geom.Size
	Scale  float64

	states        map[string]State
	stateTypes    map[string]reflect.Type
	buildCache    map[string]buildCacheEntry
	layoutCache   map[layoutCacheKey]layoutCacheEntry
	frameSeq      uint64
	interactions  []interactionEntry
	interactionBy map[string]int
	dirty         bool
	focusedInput  *string
	focusedPath   string
	animations    map[string]*animatedState
	animating     map[string]struct{}
	animatingNext map[string]struct{}
	pathCache     map[pathJoinKey]string
	pathRects     map[string]geom.Rect
	damageHint    image.Rectangle
	now           func() time.Time
	renderTime    time.Time
	hoveredPath   string
	pressedPath   string
	pressedInside bool
}

const damageEffectPad = 32.0

type pathJoinKey struct{ parent, suffix string }

type layoutCacheKey struct {
	path        string
	constraints layout.BoxConstraints
}

type layoutCacheEntry struct {
	frame uint64
	size  geom.Size
}

type buildCacheEntry struct {
	frame  uint64
	widget Widget
}

const precomputedChildPathSlots = 256

var childPathSlots = func() [precomputedChildPathSlots]string {
	var s [precomputedChildPathSlots]string
	for i := range precomputedChildPathSlots {
		s[i] = "c" + strconv.Itoa(i)
	}
	return s
}()

var childMeasurePathSlots = func() [precomputedChildPathSlots]string {
	var s [precomputedChildPathSlots]string
	for i := range precomputedChildPathSlots {
		s[i] = childPathSlots[i] + "/m"
	}
	return s
}()

func childPathSlot(i int) string {
	if uint(i) < uint(len(childPathSlots)) {
		return childPathSlots[i]
	}
	return "c" + strconv.Itoa(i)
}

func childMeasurePathSlot(i int) string {
	if uint(i) < uint(len(childMeasurePathSlots)) {
		return childMeasurePathSlots[i]
	}
	return "c" + strconv.Itoa(i) + "/m"
}

// NewFrame creates a Frame with the given theme and initial screen size.
func NewFrame(th *theme.ThemeData, screen geom.Size) *Frame {
	return &Frame{
		Theme:         th,
		Screen:        screen,
		Scale:         1,
		states:        make(map[string]State),
		stateTypes:    make(map[string]reflect.Type),
		buildCache:    make(map[string]buildCacheEntry),
		layoutCache:   make(map[layoutCacheKey]layoutCacheEntry),
		interactionBy: make(map[string]int),
		dirty:         true,
		animations:    make(map[string]*animatedState),
		animating:     make(map[string]struct{}),
		pathCache:     make(map[pathJoinKey]string),
		pathRects:     make(map[string]geom.Rect),
		now:           time.Now,
	}
}

// Resize updates the screen dimensions and marks the frame dirty.
func (f *Frame) Resize(w, h int) {
	f.Screen = geom.Sz(float64(w), float64(h))
	f.dirty = true
}

// MarkDirty schedules a rebuild on the next iteration.
func (f *Frame) MarkDirty() { f.dirty = true }

// DamageHint returns a best-effort physical-pixel dirty region derived from
// prior-frame widget rects for transient UI-only updates.
func (f *Frame) DamageHint() image.Rectangle { return f.damageHint }

// IsDirty reports whether a rebuild is pending and clears the flag.
func (f *Frame) IsDirty() bool {
	b := f.dirty
	f.dirty = false
	for path := range f.animating {
		f.addDamagePath(path)
	}
	return b || len(f.animating) > 0
}

// Render builds, lays out, and paints the widget tree rooted at root.
// State is preserved across calls; the tree is otherwise rebuilt from scratch.
func (f *Frame) Render(root Widget, c canvas.Canvas) {
	f.frameSeq++
	f.interactions = f.interactions[:0]
	clear(f.interactionBy)
	clear(f.pathRects)
	f.damageHint = image.Rectangle{}
	f.renderTime = f.currentTime()
	clear(f.animating)
	f.animatingNext = f.animating
	pctx := paint.Context{Canvas: c}
	bc := BuildContext{Theme: f.Theme, ScreenSize: f.Screen, Scale: f.Scale}
	f.renderWidget(root, layout.Tight(f.Screen.Width, f.Screen.Height), &pctx, geom.Pt(0, 0), "root", bc)
	f.animating = f.animatingNext
	f.animatingNext = nil
	if f.hoveredPath != "" && f.findInteractionByPath(f.hoveredPath) == nil {
		f.hoveredPath = ""
	}
	if f.pressedPath != "" && f.findInteractionByPath(f.pressedPath) == nil {
		f.pressedPath = ""
		f.pressedInside = false
	}
	f.renderTime = time.Time{}
}

// HandleTap dispatches a tap to the topmost interactive region at pos.
func (f *Frame) HandleTap(pos geom.Point) bool {
	if hit := f.findInteractionAt(pos); hit != nil && hit.onTap != nil {
		hit.onTap()
		return true
	}
	return false
}

// HandlePointerMove updates hover state.
func (f *Frame) HandlePointerMove(pos geom.Point) {
	hit := f.findInteractionAt(pos)
	nextPath := ""
	if hit != nil {
		nextPath = hit.path
	}

	if nextPath != f.hoveredPath {
		f.addDamagePath(f.hoveredPath)
		f.addDamagePath(nextPath)
		if prev := f.findInteractionByPath(f.hoveredPath); prev != nil && prev.onHoverChanged != nil {
			prev.onHoverChanged(false)
		}
		if hit != nil && hit.onHoverChanged != nil {
			hit.onHoverChanged(true)
		}
		f.hoveredPath = nextPath
		f.MarkDirty()
	}

	if f.pressedPath != "" {
		inside := nextPath != "" && nextPath == f.pressedPath
		if inside != f.pressedInside {
			f.addDamagePath(f.pressedPath)
			if pressed := f.findInteractionByPath(f.pressedPath); pressed != nil && pressed.onPressChanged != nil {
				pressed.onPressChanged(inside)
			}
			f.pressedInside = inside
			f.MarkDirty()
		}
		if pressed := f.findInteractionByPath(f.pressedPath); pressed != nil && pressed.onDragMove != nil {
			pressed.onDragMove(pos)
		}
	}
	if hit != nil && hit.onPointerMoveLocal != nil {
		hit.onPointerMoveLocal(pos.Sub(hit.rect.Min))
	}
}

// CursorShape returns the preferred pointer shape for the currently hovered widget.
func (f *Frame) CursorShape() event.CursorShape {
	if hit := f.findInteractionByPath(f.hoveredPath); hit != nil {
		return hit.cursorShape()
	}
	return event.CursorDefault
}

// HandlePointerDown marks the topmost interactive region as pressed.
func (f *Frame) HandlePointerDown(pos geom.Point, button event.Button) bool {
	if button != 0 && button != event.ButtonLeft {
		return false
	}
	f.HandlePointerMove(pos)
	hit := f.findInteractionAt(pos)
	if hit == nil {
		return false
	}
	f.pressedPath = hit.path
	f.pressedInside = true
	f.addDamagePath(hit.path)
	if hit.onPointerDown != nil {
		hit.onPointerDown(pos)
	}
	if hit.onPointerDownLocal != nil {
		hit.onPointerDownLocal(pos.Sub(hit.rect.Min))
	}
	if hit.onTapDown != nil {
		hit.onTapDown()
	}
	if hit.onPressChanged != nil {
		hit.onPressChanged(true)
	}
	f.MarkDirty()
	return true
}

// HandlePointerUp clears press state and fires a tap if released over the
// same region that received the press.
func (f *Frame) HandlePointerUp(pos geom.Point, button event.Button) bool {
	if button != 0 && button != event.ButtonLeft {
		return false
	}
	f.HandlePointerMove(pos)
	pressedPath := f.pressedPath
	if pressedPath == "" {
		return false
	}
	pressed := f.findInteractionByPath(pressedPath)
	if pressed != nil {
		if pressed.onPointerUpLocal != nil {
			pressed.onPointerUpLocal(pos.Sub(pressed.rect.Min))
		}
		if pressed.onDragEnd != nil {
			pressed.onDragEnd()
		}
		if pressed.onTapUp != nil {
			pressed.onTapUp()
		}
		if pressed.onPressChanged != nil {
			pressed.onPressChanged(false)
		}
	}
	releasedOverPressed := f.hoveredPath != "" && f.hoveredPath == pressedPath
	f.pressedPath = ""
	f.pressedInside = false
	f.addDamagePath(pressedPath)
	f.MarkDirty()
	if releasedOverPressed && pressed != nil && pressed.onTap != nil {
		pressed.onTap()
	}
	return true
}

// ClearPointerState removes transient hover/press state (e.g. on window blur).
func (f *Frame) ClearPointerState() {
	f.addDamagePath(f.hoveredPath)
	f.addDamagePath(f.pressedPath)
	if hovered := f.findInteractionByPath(f.hoveredPath); hovered != nil && hovered.onHoverChanged != nil {
		hovered.onHoverChanged(false)
	}
	if pressed := f.findInteractionByPath(f.pressedPath); pressed != nil && pressed.onPressChanged != nil {
		pressed.onPressChanged(false)
	}
	f.hoveredPath = ""
	f.pressedPath = ""
	f.pressedInside = false
	f.MarkDirty()
}

// FocusInput directs keyboard input to the pointed-at string variable.
// Pass nil to clear focus.
func (f *Frame) FocusInput(v *string) {
	path := ""
	if v != nil && v == f.focusedInput {
		path = f.focusedPath
	}
	f.FocusInputPath(v, path)
}

// FocusInputPath directs keyboard input to v and records the widget path whose
// visuals change when focus or text content changes. Pass nil to clear focus.
func (f *Frame) FocusInputPath(v *string, path string) {
	if v == nil {
		path = ""
	}
	if path == "" && v != nil && v == f.focusedInput {
		path = f.focusedPath
	}
	if f.focusedInput == v && f.focusedPath == path {
		return
	}
	f.addDamagePath(f.focusedPath)
	f.addDamagePath(path)
	f.focusedInput = v
	f.focusedPath = path
	f.MarkDirty()
}

// HandleScroll dispatches a scroll event to the topmost scrollable region at pos.
func (f *Frame) HandleScroll(pos geom.Point, dx, dy float64) {
	for i := len(f.interactions) - 1; i >= 0; i-- {
		e := &f.interactions[i]
		if e.rect.Contains(pos) && e.onScroll != nil {
			e.onScroll(dx, dy)
			return
		}
	}
}

// HandleKey dispatches a key event to the currently focused text input.
func (f *Frame) HandleKey(e event.Event) {
	if f.focusedInput == nil {
		return
	}
	switch ev := e.(type) {
	case event.TextInputEvent:
		f.addDamagePath(f.focusedPath)
		*f.focusedInput += string(ev.Rune)
		f.MarkDirty()
	case event.KeyEvent:
		if !ev.Down {
			return
		}
		switch ev.Key {
		case event.KeyBackspace:
			if len(*f.focusedInput) > 0 {
				f.addDamagePath(f.focusedPath)
				_, size := utf8.DecodeLastRuneInString(*f.focusedInput)
				if size <= 0 {
					size = 1
				}
				*f.focusedInput = (*f.focusedInput)[:len(*f.focusedInput)-size]
				f.MarkDirty()
			}
		default:
			if r := keyCodeToRune(ev.Key, ev.Mods); r != 0 {
				f.addDamagePath(f.focusedPath)
				*f.focusedInput += string(r)
				f.MarkDirty()
			}
		}
	}
}

// keyCodeToRune converts a KeyCode and modifier state to a printable rune.
// Returns 0 for non-printable keys.
func keyCodeToRune(key event.KeyCode, mods event.Modifiers) rune {
	shift := mods&event.ModShift != 0
	switch key {
	case event.KeySpace:
		return ' '
	case event.KeyA:
		if shift {
			return 'A'
		}
		return 'a'
	case event.KeyB:
		if shift {
			return 'B'
		}
		return 'b'
	case event.KeyC:
		if shift {
			return 'C'
		}
		return 'c'
	case event.KeyD:
		if shift {
			return 'D'
		}
		return 'd'
	case event.KeyE:
		if shift {
			return 'E'
		}
		return 'e'
	case event.KeyF:
		if shift {
			return 'F'
		}
		return 'f'
	case event.KeyG:
		if shift {
			return 'G'
		}
		return 'g'
	case event.KeyH:
		if shift {
			return 'H'
		}
		return 'h'
	case event.KeyI:
		if shift {
			return 'I'
		}
		return 'i'
	case event.KeyJ:
		if shift {
			return 'J'
		}
		return 'j'
	case event.KeyK:
		if shift {
			return 'K'
		}
		return 'k'
	case event.KeyL:
		if shift {
			return 'L'
		}
		return 'l'
	case event.KeyM:
		if shift {
			return 'M'
		}
		return 'm'
	case event.KeyN:
		if shift {
			return 'N'
		}
		return 'n'
	case event.KeyO:
		if shift {
			return 'O'
		}
		return 'o'
	case event.KeyP:
		if shift {
			return 'P'
		}
		return 'p'
	case event.KeyQ:
		if shift {
			return 'Q'
		}
		return 'q'
	case event.KeyR:
		if shift {
			return 'R'
		}
		return 'r'
	case event.KeyS:
		if shift {
			return 'S'
		}
		return 's'
	case event.KeyT:
		if shift {
			return 'T'
		}
		return 't'
	case event.KeyU:
		if shift {
			return 'U'
		}
		return 'u'
	case event.KeyV:
		if shift {
			return 'V'
		}
		return 'v'
	case event.KeyW:
		if shift {
			return 'W'
		}
		return 'w'
	case event.KeyX:
		if shift {
			return 'X'
		}
		return 'x'
	case event.KeyY:
		if shift {
			return 'Y'
		}
		return 'y'
	case event.KeyZ:
		if shift {
			return 'Z'
		}
		return 'z'
	case event.Key0:
		if shift {
			return ')'
		}
		return '0'
	case event.Key1:
		if shift {
			return '!'
		}
		return '1'
	case event.Key2:
		if shift {
			return '@'
		}
		return '2'
	case event.Key3:
		if shift {
			return '#'
		}
		return '3'
	case event.Key4:
		if shift {
			return '$'
		}
		return '4'
	case event.Key5:
		if shift {
			return '%'
		}
		return '5'
	case event.Key6:
		if shift {
			return '^'
		}
		return '6'
	case event.Key7:
		if shift {
			return '&'
		}
		return '7'
	case event.Key8:
		if shift {
			return '*'
		}
		return '8'
	case event.Key9:
		if shift {
			return '('
		}
		return '9'
	case event.KeyMinus:
		if shift {
			return '_'
		}
		return '-'
	case event.KeyEqual:
		if shift {
			return '+'
		}
		return '='
	case event.KeyLeftBracket:
		if shift {
			return '{'
		}
		return '['
	case event.KeyRightBracket:
		if shift {
			return '}'
		}
		return ']'
	case event.KeySemicolon:
		if shift {
			return ':'
		}
		return ';'
	case event.KeyApostrophe:
		if shift {
			return '"'
		}
		return '\''
	case event.KeyGrave:
		if shift {
			return '~'
		}
		return '`'
	case event.KeyBackslash:
		if shift {
			return '|'
		}
		return '\\'
	case event.KeyComma:
		if shift {
			return '<'
		}
		return ','
	case event.KeyPeriod:
		if shift {
			return '>'
		}
		return '.'
	case event.KeySlash:
		if shift {
			return '?'
		}
		return '/'
	}
	return 0
}

// registerInteraction records an interactive region during the paint pass.
func (f *Frame) registerInteraction(e interactionEntry) {
	index := len(f.interactions)
	f.interactions = append(f.interactions, e)
	f.interactionBy[e.path] = index
}

func (f *Frame) findInteractionAt(pos geom.Point) *interactionEntry {
	for i := len(f.interactions) - 1; i >= 0; i-- {
		if f.interactions[i].rect.Contains(pos) {
			return &f.interactions[i]
		}
	}
	return nil
}

func (f *Frame) findInteractionByPath(path string) *interactionEntry {
	if path == "" {
		return nil
	}
	idx, ok := f.interactionBy[path]
	if !ok || idx < 0 || idx >= len(f.interactions) {
		return nil
	}
	return &f.interactions[idx]
}

func (e *interactionEntry) cursorShape() event.CursorShape {
	if e == nil {
		return event.CursorDefault
	}
	if e.cursorFunc != nil {
		return e.cursorFunc()
	}
	return e.cursor
}

func (f *Frame) addDamagePath(path string) {
	if path == "" {
		return
	}
	rect, ok := f.pathRects[path]
	if !ok {
		return
	}
	f.addDamageRect(rect)
}

func (f *Frame) addDamageRect(r geom.Rect) {
	if damageEffectPad > 0 {
		r = r.Inset(-damageEffectPad, -damageEffectPad)
	}
	scale := f.Scale
	if scale <= 0 {
		scale = 1
	}
	x0 := int(math.Floor(r.Min.X * scale))
	y0 := int(math.Floor(r.Min.Y * scale))
	x1 := int(math.Ceil(r.Max.X * scale))
	y1 := int(math.Ceil(r.Max.Y * scale))
	if x0 >= x1 || y0 >= y1 {
		return
	}
	clip := image.Rect(0, 0, int(math.Ceil(f.Screen.Width*scale)), int(math.Ceil(f.Screen.Height*scale)))
	ir := image.Rect(x0, y0, x1, y1).Intersect(clip)
	if ir.Empty() {
		return
	}
	if f.damageHint.Empty() {
		f.damageHint = ir
	} else {
		f.damageHint = f.damageHint.Union(ir)
	}
}

func (f *Frame) storePathRect(path string, offset geom.Point, sz geom.Size, pctx *paint.Context) {
	if pctx == nil || path == "" || sz.Width <= 0 || sz.Height <= 0 {
		return
	}
	f.pathRects[path] = geom.NewRect(offset.X, offset.Y, sz.Width, sz.Height)
}

// renderWidget recursively builds, lays out, and (if pctx≠nil) paints w.
// pctx == nil means layout-only pass.
func (f *Frame) renderWidget(w Widget, c layout.BoxConstraints, pctx *paint.Context, offset geom.Point, path string, bc BuildContext) geom.Size {
	if w == nil {
		return c.Smallest()
	}
	bc.path = path
	bc.frame = f

	if sz, ok := f.cachedLayout(path, c); ok && pctx == nil {
		return sz
	}

	// Fast path for the common Text case.
	if t, ok := w.(Text); ok && t.Style != nil {
		leaf := textLeaf{content: t.Content, style: *t.Style}
		sz, ok := f.cachedLayout(path, c)
		if !ok {
			sz = leaf.Layout(c)
			f.storeLayout(path, c, sz)
		}
		if pctx != nil {
			leaf.Paint(pctx, offset, sz)
		}
		f.storePathRect(path, offset, sz, pctx)
		return sz
	}

	// GestureDetector: must be handled before MultiChild so it can register
	// interaction regions during the paint pass.
	//
	// IMPORTANT: register the GD's own entry BEFORE rendering its children.
	// findInteractionAt iterates last-to-first (highest index wins), so by
	// reserving a slot early, children rendered inside will occupy later
	// indices and naturally win the hit-test over the enclosing GD.
	// This allows e.g. buttons inside a Scroll to be tapped normally.
	if gd, ok := w.(GestureDetector); ok {
		state := InteractionState{
			Hovered: f.hoveredPath == path,
			Pressed: f.pressedPath == path && f.pressedInside,
		}
		child, ok := f.cachedBuild(path)
		if !ok {
			child = gd.Child
			if gd.Builder != nil {
				child = gd.Builder(state)
			}
			f.storeBuild(path, child)
		}

		// Pre-reserve the slot with an empty rect; children registered during
		// the child render will occupy slots after this one.
		gdIdx := -1
		if pctx != nil {
			gdIdx = len(f.interactions)
			f.registerInteraction(interactionEntry{path: path})
		}

		sz := f.renderWidget(child, c, pctx, offset, f.joinPath(path, "gd"), bc)
		f.storeLayout(path, c, sz)

		// Backfill the pre-reserved slot with the actual rect and handlers.
		if pctx != nil && gdIdx >= 0 {
			f.interactions[gdIdx] = interactionEntry{
				path:               path,
				rect:               geom.NewRect(offset.X, offset.Y, sz.Width, sz.Height),
				onTap:              gd.OnTap,
				onTapDown:          gd.OnTapDown,
				onTapUp:            gd.OnTapUp,
				onPointerDown:      gd.OnPointerDown,
				onHoverChanged:     gd.OnHoverChanged,
				onPressChanged:     gd.OnPressChanged,
				onPointerDownLocal: gd.OnPointerDownLocal,
				onPointerUpLocal:   gd.OnPointerUpLocal,
				onPointerMoveLocal: gd.OnPointerMoveLocal,
				onDragMove:         gd.OnDragMove,
				onDragEnd:          gd.OnDragEnd,
				onScroll:           gd.OnScroll,
				cursor:             gd.Cursor,
				cursorFunc:         gd.CursorFunc,
			}
			f.interactionBy[path] = gdIdx
		}
		f.storePathRect(path, offset, sz, pctx)
		return sz
	}

	switch v := w.(type) {
	case MultiChild:
		cr := frameChildRenderer{f: f, pctx: pctx, path: path, bc: bc}
		sz := v.RenderChildren(c, pctx, offset, cr)
		f.storeLayout(path, c, sz)
		f.storePathRect(path, offset, sz, pctx)
		return sz

	case StatefulWidget:
		wType := reflect.TypeOf(v)
		st, ok := f.states[path]
		if ok && f.stateTypes[path] != wType {
			// Widget type changed at this path — discard the stale state.
			delete(f.states, path)
			delete(f.stateTypes, path)
			ok = false
		}
		if !ok {
			st = v.CreateState()
			if inj, ok2 := st.(dirtyInjector); ok2 {
				inj.setMarkDirty(func() { f.MarkDirty() })
			}
			if upd, ok2 := st.(interface{ UpdateWidget(Widget) }); ok2 {
				upd.UpdateWidget(v)
			}
			if init, ok2 := st.(interface{ InitState() }); ok2 {
				init.InitState()
			}
			f.states[path] = st
			f.stateTypes[path] = wType
		} else {
			if upd, ok2 := st.(interface{ UpdateWidget(Widget) }); ok2 {
				upd.UpdateWidget(v)
			}
		}
		child, ok := f.cachedBuild(path)
		if !ok {
			child = st.Build(bc)
			f.storeBuild(path, child)
		}
		sz := f.renderWidget(child, c, pctx, offset, f.joinPath(path, "s"), bc)
		f.storeLayout(path, c, sz)
		f.storePathRect(path, offset, sz, pctx)
		return sz

	case Buildable:
		child, ok := f.cachedBuild(path)
		if !ok {
			child = v.Build(bc)
			f.storeBuild(path, child)
		}
		sz := f.renderWidget(child, c, pctx, offset, f.joinPath(path, "b"), bc)
		f.storeLayout(path, c, sz)
		f.storePathRect(path, offset, sz, pctx)
		return sz

	case RenderBox:
		sz, ok := f.cachedLayout(path, c)
		if !ok {
			sz = v.Layout(c)
			f.storeLayout(path, c, sz)
		}
		if pctx != nil {
			v.Paint(pctx, offset, sz)
		}
		f.storePathRect(path, offset, sz, pctx)
		return sz

	default:
		sz := c.Smallest()
		f.storeLayout(path, c, sz)
		f.storePathRect(path, offset, sz, pctx)
		return sz
	}
}

// frameChildRenderer implements [ChildRenderer] for a frame.
type frameChildRenderer struct {
	f    *Frame
	pctx *paint.Context
	path string
	bc   BuildContext
}

func (cr frameChildRenderer) Render(w Widget, c layout.BoxConstraints, offset geom.Point, suffix string) geom.Size {
	return cr.f.renderWidget(w, c, cr.pctx, offset, cr.f.joinPath(cr.path, suffix), cr.bc)
}

func (cr frameChildRenderer) Measure(w Widget, c layout.BoxConstraints, suffix string) geom.Size {
	return cr.f.renderWidget(w, c, nil, geom.Pt(0, 0), cr.f.joinPath(cr.path, suffix), cr.bc)
}

func (f *Frame) joinPath(parent, suffix string) string {
	if parent == "" {
		return suffix
	}
	key := pathJoinKey{parent: parent, suffix: suffix}
	if p, ok := f.pathCache[key]; ok {
		return p
	}
	p := parent + "/" + suffix
	f.pathCache[key] = p
	return p
}

func (f *Frame) cachedLayout(path string, c layout.BoxConstraints) (geom.Size, bool) {
	entry, ok := f.layoutCache[layoutCacheKey{path: path, constraints: c}]
	if !ok || entry.frame != f.frameSeq {
		return geom.Size{}, false
	}
	return entry.size, true
}

func (f *Frame) storeLayout(path string, c layout.BoxConstraints, sz geom.Size) {
	f.layoutCache[layoutCacheKey{path: path, constraints: c}] = layoutCacheEntry{
		frame: f.frameSeq,
		size:  sz,
	}
}

func (f *Frame) cachedBuild(path string) (Widget, bool) {
	entry, ok := f.buildCache[path]
	if !ok || entry.frame != f.frameSeq {
		return nil, false
	}
	return entry.widget, true
}

func (f *Frame) storeBuild(path string, w Widget) {
	f.buildCache[path] = buildCacheEntry{
		frame:  f.frameSeq,
		widget: w,
	}
}

// workMu guards workQueue, which may be written from any goroutine.
var (
	workMu    sync.Mutex
	workQueue []func()
)

// EnqueueWork schedules fn to run on the main goroutine during the next
// PumpBackgroundWork call. Safe to call from any goroutine.
func EnqueueWork(fn func()) {
	workMu.Lock()
	workQueue = append(workQueue, fn)
	workMu.Unlock()
}

// PumpBackgroundWork drains all pending work items enqueued via EnqueueWork.
// Called by the app event loop both before and after event dispatch so that
// mutations from sutra goroutines and gesture callbacks both land in the same
// frame that triggered them.
func PumpBackgroundWork() {
	for {
		workMu.Lock()
		if len(workQueue) == 0 {
			workMu.Unlock()
			return
		}
		q := workQueue
		workQueue = nil
		workMu.Unlock()
		for _, fn := range q {
			fn()
		}
	}
}

func (f *Frame) currentTime() time.Time {
	if !f.renderTime.IsZero() {
		return f.renderTime
	}
	if f.now != nil {
		return f.now()
	}
	return time.Now()
}
