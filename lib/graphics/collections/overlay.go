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

// Package collections provides higher-level reusable UI components built on
// top of lib/graphics/widget. It acts as the framework's standard library of
// composed components and application-level layout patterns.
//
// The package is deliberately small. Every component included must clearly
// justify its existence as a reusable abstraction. When in doubt, compose
// from widget primitives instead.
//
// Core components:
//   - [OverlayManager] / [OverlayView] — feature-complete overlay stack
//   - [Application] — responsive application shell with overlays and navigation
//   - [AppBar] — top bar with title and actions
//   - [NavBar] — persistent sidebar navigation
//   - [BottomNav] — mobile bottom navigation
//   - [DrawerConfig] / [DrawerController] — slide-in drawer
//   - [Card] / [Section] — grouped surface containers
//   - [DialogController] — modal dialog
//   - [PopupMenuController] / [MenuItem] — contextual popup menu
//   - [ToastController] / [ToastHost] — non-blocking notification toasts
//   - [FAB] — floating action button
//   - [PanelController] — exclusive panel (launcher, quick-settings, …)
package collections

import (
	"slices"
	"sync"

	"avyos.dev/lib/graphics/color"
	"avyos.dev/lib/graphics/widget"
)

// ─── OverlayEntry ─────────────────────────────────────────────────────────────

// OverlayEntry is a single widget layer managed by an [OverlayManager].
//
// Create an entry, configure it, then add it to the stack via
// [OverlayManager.Insert] or its siblings. Remove it by calling [Remove].
//
//	entry := &collections.OverlayEntry{
//	    Builder: func(ctx widget.BuildContext) widget.Widget { return myWidget },
//	    Modal:   true,
//	}
//	overlay.Insert(entry)
//	defer entry.Remove()
type OverlayEntry struct {
	// Builder is called every frame to produce the entry's widget.
	// The widget is rendered inside a full-screen Stack, so use
	// widget.Positioned to anchor it wherever you want.
	Builder func(ctx widget.BuildContext) widget.Widget

	// Opaque hints that this entry covers the entire screen.
	// When true, entries below this one in the stack are not built or painted,
	// saving CPU on deeply nested overlays. Only set this when the entry truly
	// fills the screen (e.g. a full-screen route transition).
	Opaque bool

	// Modal causes a semi-transparent scrim to be rendered immediately below
	// this entry. Clicking the scrim does NOT automatically close the overlay —
	// your Builder must handle that via a backdrop GestureDetector.
	Modal bool

	// Z is the explicit z-index. Entries with higher Z appear in front.
	// Entries with the same Z are rendered in insertion order (first=back).
	// The default value (0) is the normal layer; use positive values for
	// top-priority overlays (tooltips, toasts) and negative for backgrounds.
	Z int

	// unexported
	id      int
	manager *OverlayManager
}

// Remove removes this entry from its [OverlayManager]. Safe to call even if
// the entry has already been removed or was never inserted.
func (e *OverlayEntry) Remove() {
	if e.manager != nil {
		e.manager.Remove(e)
	}
}

// MarkNeedsBuild triggers a full rebuild of the [OverlayView] that hosts this
// entry's manager. Use this when the entry's content has changed outside of
// a normal widget SetState cycle (e.g. from a goroutine).
func (e *OverlayEntry) MarkNeedsBuild() {
	if e.manager != nil {
		e.manager.rebuild()
	}
}

// ─── OverlayManager ───────────────────────────────────────────────────────────

// OverlayManager maintains an ordered stack of [OverlayEntry] layers and
// notifies its [OverlayView] host whenever the stack changes.
//
// Entries are rendered bottom-to-top. The ordering is determined by the
// entry's [OverlayEntry.Z] field (ascending), with ties broken by insertion
// sequence (earlier insertions are further back).
//
// Use [NewOverlayManager] to create one, then pair it with an [OverlayView]
// placed near the root of the widget tree.
type OverlayManager struct {
	mu      sync.Mutex
	entries []*OverlayEntry
	nextID  int
	notify  func()
}

// NewOverlayManager creates an OverlayManager.
func NewOverlayManager() *OverlayManager { return &OverlayManager{} }

// SetNotify registers fn to be called whenever the stack changes.
// OverlayView sets this automatically; external callers rarely need it.
func (m *OverlayManager) SetNotify(fn func()) {
	m.mu.Lock()
	m.notify = fn
	m.mu.Unlock()
}

// Insert adds entry to the stack. Its position is determined by entry.Z:
// it is placed after the last existing entry whose Z ≤ entry.Z, so entries
// with the same Z appear in insertion order (newest at the front of that group).
//
// If entry has already been inserted into this or another manager it will be
// moved to this manager at the computed position (no duplicates).
func (m *OverlayManager) Insert(entry *OverlayEntry) {
	m.mu.Lock()
	m.adopt(entry)
	idx := m.zInsertionIndex(entry.Z)
	m.entries = slices.Insert(m.entries, idx, entry)
	notify := m.notify
	m.mu.Unlock()
	if notify != nil {
		notify()
	}
}

// InsertAbove inserts entry immediately above reference in the render stack,
// regardless of Z values. If reference is not present, entry is appended at
// the very top (same as Insert with a very high Z).
func (m *OverlayManager) InsertAbove(entry, reference *OverlayEntry) {
	m.mu.Lock()
	m.adopt(entry)
	idx := len(m.entries) // default: top
	for i, e := range m.entries {
		if e == reference {
			idx = i + 1
			break
		}
	}
	m.entries = slices.Insert(m.entries, idx, entry)
	notify := m.notify
	m.mu.Unlock()
	if notify != nil {
		notify()
	}
}

// InsertBelow inserts entry immediately below reference in the render stack,
// regardless of Z values. If reference is not present, entry is prepended at
// the very bottom.
func (m *OverlayManager) InsertBelow(entry, reference *OverlayEntry) {
	m.mu.Lock()
	m.adopt(entry)
	idx := 0 // default: bottom
	for i, e := range m.entries {
		if e == reference {
			idx = i
			break
		}
	}
	m.entries = slices.Insert(m.entries, idx, entry)
	notify := m.notify
	m.mu.Unlock()
	if notify != nil {
		notify()
	}
}

// InsertAll inserts all entries using their Z values for ordering.
// Entries are inserted sequentially so they end up in the given order within
// each Z group.
func (m *OverlayManager) InsertAll(entries []*OverlayEntry) {
	for _, e := range entries {
		m.Insert(e)
	}
}

// Remove removes entry from the stack. Safe to call if not present.
func (m *OverlayManager) Remove(entry *OverlayEntry) {
	m.mu.Lock()
	for i, e := range m.entries {
		if e == entry {
			m.entries = slices.Delete(m.entries, i, i+1)
			break
		}
	}
	entry.manager = nil
	notify := m.notify
	m.mu.Unlock()
	if notify != nil {
		notify()
	}
}

// RemoveAll removes every entry from the stack.
func (m *OverlayManager) RemoveAll() {
	m.mu.Lock()
	for _, e := range m.entries {
		e.manager = nil
	}
	m.entries = nil
	notify := m.notify
	m.mu.Unlock()
	if notify != nil {
		notify()
	}
}

// Rearrange moves the listed entries to just above the reference entry.
// If above is nil, they are placed at the very top.
// Entries not currently in the stack are inserted at the target position.
// The relative order of the given entries is preserved.
func (m *OverlayManager) Rearrange(entries []*OverlayEntry, above *OverlayEntry) {
	m.mu.Lock()

	// Build a fast-lookup set of IDs being rearranged.
	ids := make(map[int]bool, len(entries))
	for _, e := range entries {
		ids[e.id] = true
	}

	// Split current stack into "keep" (not being rearranged) and discard the rest.
	keep := make([]*OverlayEntry, 0, len(m.entries))
	for _, e := range m.entries {
		if !ids[e.id] {
			keep = append(keep, e)
		}
	}

	// Find the splice point in the "keep" slice.
	insertAt := len(keep) // default: top
	if above != nil {
		for i, e := range keep {
			if e == above {
				insertAt = i + 1
				break
			}
		}
	}

	// Build the final slice.
	result := make([]*OverlayEntry, 0, len(keep)+len(entries))
	result = append(result, keep[:insertAt]...)
	for _, e := range entries {
		m.adopt(e)
		result = append(result, e)
	}
	result = append(result, keep[insertAt:]...)
	m.entries = result

	notify := m.notify
	m.mu.Unlock()
	if notify != nil {
		notify()
	}
}

// IsPresent reports whether entry is currently in the stack.
func (m *OverlayManager) IsPresent(entry *OverlayEntry) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, e := range m.entries {
		if e == entry {
			return true
		}
	}
	return false
}

// rebuild calls the registered notify function without mutating the stack.
// Used by OverlayEntry.MarkNeedsBuild.
func (m *OverlayManager) rebuild() {
	m.mu.Lock()
	notify := m.notify
	m.mu.Unlock()
	if notify != nil {
		notify()
	}
}

// adopt assigns a fresh ID and back-pointer to entry if not already owned.
// Must be called with mu held.
func (m *OverlayManager) adopt(entry *OverlayEntry) {
	if entry.id == 0 {
		m.nextID++
		entry.id = m.nextID
	}
	entry.manager = m
}

// zInsertionIndex returns the index at which to insert an entry with the
// given Z value: after the last existing entry whose Z ≤ z.
// Must be called with mu held.
func (m *OverlayManager) zInsertionIndex(z int) int {
	for i := len(m.entries) - 1; i >= 0; i-- {
		if m.entries[i].Z <= z {
			return i + 1
		}
	}
	return 0
}

// snapshot returns a safe copy of the current entry slice.
func (m *OverlayManager) snapshot() []*OverlayEntry {
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.entries) == 0 {
		return nil
	}
	out := make([]*OverlayEntry, len(m.entries))
	copy(out, m.entries)
	return out
}

// ─── OverlayView ──────────────────────────────────────────────────────────────

// OverlayView renders its base Child and then paints every entry from Manager
// on top, in Z-order (lowest first). Place exactly one OverlayView near the
// root of the widget tree.
//
//	collections.OverlayView{
//	    Manager: myOverlayManager,
//	    Child:   myApp,
//	}
type OverlayView struct {
	Manager *OverlayManager
	Child   widget.Widget
}

func (o OverlayView) CreateState() widget.State { return &overlayViewState{} }

type overlayViewState struct {
	widget.StateBase
	w OverlayView
}

func (s *overlayViewState) InitState() {
	if s.w.Manager != nil {
		s.w.Manager.SetNotify(func() { s.SetState(nil) })
	}
}

func (s *overlayViewState) UpdateWidget(w widget.Widget) {
	if v, ok := w.(OverlayView); ok {
		prev := s.w.Manager
		s.w = v
		if v.Manager != prev {
			if prev != nil {
				prev.SetNotify(nil)
			}
			if v.Manager != nil {
				v.Manager.SetNotify(func() { s.SetState(nil) })
			}
		}
	}
}

func (s *overlayViewState) Build(ctx widget.BuildContext) widget.Widget {
	entries := s.w.Manager.snapshot()

	if len(entries) == 0 {
		if s.w.Child == nil {
			return widget.SizedBox{}
		}
		return s.w.Child
	}

	// Find the topmost Opaque entry; entries below it need not be built.
	renderFrom := 0
	includeBase := true
	for i, e := range entries {
		if e.Opaque {
			renderFrom = i
			includeBase = false
		}
	}

	cap := 1 + (len(entries)-renderFrom)*2
	children := make([]widget.Widget, 0, cap)

	if includeBase {
		base := s.w.Child
		if base == nil {
			base = widget.SizedBox{}
		}
		children = append(children, base)
	} else {
		// Stack needs a sized base even when the opaque entry covers it.
		children = append(children, widget.SizedBox{})
	}

	scrim := color.Black.WithAlpha(0.48)
	for i := renderFrom; i < len(entries); i++ {
		e := entries[i]
		if e.Modal {
			children = append(children, widget.Positioned{
				Top:    widget.Ptr(0),
				Right:  widget.Ptr(0),
				Bottom: widget.Ptr(0),
				Left:   widget.Ptr(0),
				Child:  widget.Container{Fill: scrim},
			})
		}
		if e.Builder != nil {
			children = append(children, e.Builder(ctx))
		}
	}

	return widget.Stack{Children: children}
}
