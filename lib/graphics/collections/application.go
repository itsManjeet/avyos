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

	"avyos.dev/lib/graphics/geom"
	"avyos.dev/lib/graphics/layout"
	"avyos.dev/lib/graphics/widget"
)

// ─── Breakpoints ──────────────────────────────────────────────────────────────

// Breakpoints defines the screen-width thresholds used by [Application] to
// select between compact (mobile), medium (tablet), and expanded (desktop)
// layouts.
type Breakpoints struct {
	Compact float64
	Medium  float64
}

// DefaultBreakpoints is the breakpoint set used when Application.Breaks is zero.
var DefaultBreakpoints = Breakpoints{Compact: 600, Medium: 1000}

// LayoutMode describes the active responsive layout tier.
type LayoutMode int

const (
	LayoutCompact LayoutMode = iota
	LayoutMedium
	LayoutExpanded
)

// ─── ApplicationPage ──────────────────────────────────────────────────────────

// ApplicationPage is one entry in the application's page stack.
//
// Set either Child or Builder. Builder is preferred when the page needs the
// current BuildContext.
type ApplicationPage struct {
	Name    string
	Child   widget.Widget
	Builder func(ctx widget.BuildContext) widget.Widget
}

func (p ApplicationPage) build(ctx widget.BuildContext) widget.Widget {
	if p.Builder != nil {
		return p.Builder(ctx)
	}
	if p.Child != nil {
		return p.Child
	}
	return widget.SizedBox{}
}

// ─── ApplicationController ───────────────────────────────────────────────────

// ApplicationController owns application-scoped services: overlays, toasts,
// dialogs, popup menus, exclusive panels, and a page stack for navigation.
//
// It is the runtime companion to [Application], similar in spirit to a simple
// navigator plus overlay host in other UI frameworks.
type ApplicationController struct {
	mu     sync.Mutex
	pages  []ApplicationPage
	notify func()

	overlay *OverlayManager
	toasts  *ToastController
	dialogs *DialogController
	popups  *PopupMenuController
	panels  *PanelController
}

// NewApplicationController creates an ApplicationController with internal
// overlay, toast, dialog, popup, and panel managers wired together.
func NewApplicationController() *ApplicationController {
	overlay := NewOverlayManager()
	c := &ApplicationController{
		overlay: overlay,
		toasts:  NewToastController(),
	}
	c.dialogs = NewDialogController(overlay)
	c.popups = NewPopupMenuController(overlay)
	c.panels = NewPanelController(overlay)
	c.panels.SetNotify(c.rebuild)
	return c
}

// SetNotify registers fn to be called when application-level state changes,
// primarily navigation or page-stack updates.
func (c *ApplicationController) SetNotify(fn func()) {
	c.mu.Lock()
	c.notify = fn
	c.mu.Unlock()
}

func (c *ApplicationController) rebuild() {
	c.mu.Lock()
	notify := c.notify
	c.mu.Unlock()
	if notify != nil {
		notify()
	}
}

// Overlay returns the application's shared overlay manager.
func (c *ApplicationController) Overlay() *OverlayManager { return c.overlay }

// Toasts returns the application's shared toast controller.
func (c *ApplicationController) Toasts() *ToastController { return c.toasts }

// Dialogs returns the application's shared dialog controller.
func (c *ApplicationController) Dialogs() *DialogController { return c.dialogs }

// Popups returns the application's shared popup menu controller.
func (c *ApplicationController) Popups() *PopupMenuController { return c.popups }

// Panels returns the application's shared exclusive panel controller.
func (c *ApplicationController) Panels() *PanelController { return c.panels }

// Push adds page to the top of the navigation stack.
func (c *ApplicationController) Push(page ApplicationPage) {
	c.mu.Lock()
	c.pages = append(c.pages, page)
	notify := c.notify
	c.mu.Unlock()
	if notify != nil {
		notify()
	}
}

// NavigateTo is an alias for [Push], provided for a more app-like API.
func (c *ApplicationController) NavigateTo(page ApplicationPage) { c.Push(page) }

// Replace swaps the current top page for page, or pushes it if the stack is empty.
func (c *ApplicationController) Replace(page ApplicationPage) {
	c.mu.Lock()
	if len(c.pages) == 0 {
		c.pages = append(c.pages, page)
	} else {
		c.pages[len(c.pages)-1] = page
	}
	notify := c.notify
	c.mu.Unlock()
	if notify != nil {
		notify()
	}
}

// ResetTo clears the page stack and sets page as the only active page.
func (c *ApplicationController) ResetTo(page ApplicationPage) {
	c.mu.Lock()
	c.pages = []ApplicationPage{page}
	notify := c.notify
	c.mu.Unlock()
	if notify != nil {
		notify()
	}
}

// Pop removes the current page. It returns false when the stack is already empty.
func (c *ApplicationController) Pop() bool {
	c.mu.Lock()
	if len(c.pages) == 0 {
		c.mu.Unlock()
		return false
	}
	c.pages = c.pages[:len(c.pages)-1]
	notify := c.notify
	c.mu.Unlock()
	if notify != nil {
		notify()
	}
	return true
}

// GoHome clears the page stack so the application's base body becomes visible again.
func (c *ApplicationController) GoHome() {
	c.mu.Lock()
	if len(c.pages) == 0 {
		c.mu.Unlock()
		return
	}
	c.pages = nil
	notify := c.notify
	c.mu.Unlock()
	if notify != nil {
		notify()
	}
}

// CanPop reports whether there is at least one page above the application's base body.
func (c *ApplicationController) CanPop() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.pages) > 0
}

// PageDepth reports the number of stacked application pages.
func (c *ApplicationController) PageDepth() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.pages)
}

// CurrentPage returns the current top page, if any.
func (c *ApplicationController) CurrentPage() (ApplicationPage, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.pages) == 0 {
		return ApplicationPage{}, false
	}
	return c.pages[len(c.pages)-1], true
}

// ShowDialog forwards to the application's shared dialog controller.
func (c *ApplicationController) ShowDialog(d Dialog) func() {
	return c.dialogs.Show(d)
}

// ShowPopup forwards to the application's shared popup controller.
func (c *ApplicationController) ShowPopup(items []MenuItem, anchorRect geom.Rect) func() {
	return c.popups.Show(items, anchorRect)
}

// ShowToast forwards to the application's shared toast controller.
func (c *ApplicationController) ShowToast(message string, variant ToastVariant) func() {
	return c.toasts.Show(message, variant)
}

// ShowToastFor forwards to the application's shared toast controller.
func (c *ApplicationController) ShowToastFor(message string, variant ToastVariant, d time.Duration) {
	c.toasts.ShowFor(message, variant, d)
}

// TogglePanel forwards to the application's shared exclusive panel controller.
func (c *ApplicationController) TogglePanel(key string, build func() widget.Widget) {
	c.panels.Toggle(key, build)
}

// ClosePanel closes the currently open exclusive panel, if any.
func (c *ApplicationController) ClosePanel() { c.panels.Close() }

// InsertOverlay inserts entry into the application's shared overlay manager.
func (c *ApplicationController) InsertOverlay(entry *OverlayEntry) {
	c.overlay.Insert(entry)
}

// RemoveOverlay removes entry from the application's shared overlay manager.
func (c *ApplicationController) RemoveOverlay(entry *OverlayEntry) {
	c.overlay.Remove(entry)
}

// ─── Application ─────────────────────────────────────────────────────────────

// Application is the responsive application shell.
//
// It composes the major structural regions of an app, owns the overlay hosts
// through [ApplicationController], and resolves the visible page from the
// controller's navigation stack.
type Application struct {
	Controller *ApplicationController
	AppBar     *AppBar
	NavBar     *NavBar
	BottomNav  *BottomNav
	Drawer     *DrawerConfig
	FAB        *FAB
	StatusBar  widget.Widget
	Body       widget.Widget
	Breaks     Breakpoints
}

func (Application) CreateState() widget.State { return &applicationState{} }

type applicationState struct {
	widget.StateBase
	w          Application
	ctrl       *ApplicationController
	autoDrawer *DrawerController
	sidebarOn  bool
}

func (s *applicationState) InitState() {
	if s.autoDrawer == nil {
		s.autoDrawer = NewDrawerController()
	}
	s.sidebarOn = true
	s.bindController(s.w.Controller)
}

func (s *applicationState) UpdateWidget(w widget.Widget) {
	if v, ok := w.(Application); ok {
		prev := s.ctrl
		s.w = v
		next := v.Controller
		if next == nil {
			next = prev
		}
		if next == nil {
			next = NewApplicationController()
		}
		if next != prev {
			s.bindController(next)
		}
	}
}

func (s *applicationState) bindController(ctrl *ApplicationController) {
	if ctrl == nil {
		ctrl = NewApplicationController()
	}
	if s.ctrl != nil && s.ctrl != ctrl {
		s.ctrl.SetNotify(nil)
	}
	s.ctrl = ctrl
	s.ctrl.SetNotify(func() { s.SetState(nil) })
}

func (s *applicationState) Build(ctx widget.BuildContext) widget.Widget {
	bp := s.w.Breaks
	if bp.Compact == 0 && bp.Medium == 0 {
		bp = DefaultBreakpoints
	}

	mode := resolveLayoutMode(ctx.ScreenSize.Width, bp)
	body := s.resolveBody(ctx)
	root := buildResponsiveApplicationShell(s.w, s.ctrl, s.autoDrawer, ctx, mode, body, s.sidebarOn, s.toggleSidebar)

	if s.ctrl.toasts != nil {
		root = ToastHost{Controller: s.ctrl.toasts, Child: root}
	}
	if s.ctrl.overlay != nil {
		root = OverlayView{Manager: s.ctrl.overlay, Child: root}
	}
	return root
}

func (s *applicationState) toggleSidebar() {
	s.SetState(func() {
		s.sidebarOn = !s.sidebarOn
	})
}

func (s *applicationState) resolveBody(ctx widget.BuildContext) widget.Widget {
	if s.ctrl != nil {
		if page, ok := s.ctrl.CurrentPage(); ok {
			return page.build(ctx)
		}
	}
	if s.w.Body != nil {
		return s.w.Body
	}
	return widget.SizedBox{}
}

func resolveLayoutMode(width float64, bp Breakpoints) LayoutMode {
	mode := LayoutExpanded
	if width > 0 && width < bp.Compact {
		mode = LayoutCompact
	} else if width > 0 && width < bp.Medium {
		mode = LayoutMedium
	}
	return mode
}

// ─── BreakpointLayout ─────────────────────────────────────────────────────────

// BreakpointLayout selects one of three widgets based on the current screen width.
// This is a lightweight alternative to [Application] for cases where you need
// responsive switching without the full application chrome.
type BreakpointLayout struct {
	Compact  widget.Widget
	Medium   widget.Widget
	Expanded widget.Widget
	Breaks   Breakpoints
}

func (b BreakpointLayout) Build(ctx widget.BuildContext) widget.Widget {
	bp := b.Breaks
	if bp.Compact == 0 && bp.Medium == 0 {
		bp = DefaultBreakpoints
	}

	switch resolveLayoutMode(ctx.ScreenSize.Width, bp) {
	case LayoutCompact:
		if b.Compact != nil {
			return b.Compact
		}
	case LayoutMedium:
		if b.Medium != nil {
			return b.Medium
		}
		if b.Compact != nil {
			return b.Compact
		}
	default:
		if b.Expanded != nil {
			return b.Expanded
		}
		if b.Medium != nil {
			return b.Medium
		}
	}
	if b.Compact != nil {
		return b.Compact
	}
	return widget.SizedBox{}
}

// ─── SplitLayout ──────────────────────────────────────────────────────────────

// SplitLayout renders a two-pane layout: a fixed-width primary panel on the
// left and an expanded secondary panel on the right. Below the breakpoint
// width the secondary fills the screen (primary is hidden).
type SplitLayout struct {
	Primary      widget.Widget
	Secondary    widget.Widget
	PrimaryWidth float64
	BreakWidth   float64
}

func (sl SplitLayout) Build(ctx widget.BuildContext) widget.Widget {
	breakW := sl.BreakWidth
	if breakW <= 0 {
		breakW = DefaultBreakpoints.Compact
	}
	primW := sl.PrimaryWidth
	if primW <= 0 {
		primW = 280
	}

	secondary := sl.Secondary
	if secondary == nil {
		secondary = widget.SizedBox{}
	}

	if ctx.ScreenSize.Width > 0 && ctx.ScreenSize.Width < breakW {
		return secondary
	}

	primary := sl.Primary
	if primary == nil {
		primary = widget.SizedBox{}
	}

	return widget.Row{
		CrossAxisAlignment: layout.CrossStretch,
		Children: []widget.Widget{
			widget.SizedBox{Width: primW, Child: primary},
			widget.Expanded{Child: secondary},
		},
	}
}
