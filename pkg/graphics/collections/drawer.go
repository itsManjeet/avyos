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
	"avyos.dev/pkg/graphics/color"
	"avyos.dev/pkg/graphics/geom"
	"avyos.dev/pkg/graphics/layout"
	"avyos.dev/pkg/graphics/widget"
)

// ─── DrawerController ─────────────────────────────────────────────────────────

// DrawerController manages open/close state for a [DrawerConfig].
// It is a reference type; pass it to both the Application and any widget that
// needs to open or close the drawer (e.g. a hamburger button in the AppBar).
type DrawerController struct {
	open   bool
	notify func()
}

// NewDrawerController creates a DrawerController (initially closed).
func NewDrawerController() *DrawerController { return &DrawerController{} }

// Open slides the drawer into view.
func (c *DrawerController) Open() {
	if !c.open {
		c.open = true
		if c.notify != nil {
			c.notify()
		}
	}
}

// Close slides the drawer out of view.
func (c *DrawerController) Close() {
	if c.open {
		c.open = false
		if c.notify != nil {
			c.notify()
		}
	}
}

// Toggle flips the open state.
func (c *DrawerController) Toggle() {
	if c.open {
		c.Close()
	} else {
		c.Open()
	}
}

// IsOpen reports whether the drawer is currently open.
func (c *DrawerController) IsOpen() bool { return c.open }

// ─── DrawerConfig ─────────────────────────────────────────────────────────────

// DrawerConfig binds a [DrawerController] with the drawer's content and
// dimensions. Pass one to [Application.Drawer].
type DrawerConfig struct {
	Controller *DrawerController
	Child      widget.Widget
	// Width is the drawer panel width. 0 defaults to 280.
	Width float64
}

// ─── Drawer widget ────────────────────────────────────────────────────────────

// Drawer is the animated slide-in panel widget. It wraps Body with a
// slide-in panel controlled by DrawerConfig.Controller.
//
// For full-app drawer usage, prefer setting [Application.Drawer] which
// composes Drawer automatically. Use Drawer directly when you need a
// bounded drawer within a sub-section of the layout.
type Drawer struct {
	Config DrawerConfig
	Body   widget.Widget // content behind the panel
}

func (d Drawer) CreateState() widget.State { return &drawerState{} }

type drawerState struct {
	widget.StateBase
	w Drawer
}

func (s *drawerState) UpdateWidget(w widget.Widget) {
	if v, ok := w.(Drawer); ok {
		s.w = v
		if v.Config.Controller != nil {
			v.Config.Controller.notify = func() { s.SetState(nil) }
		}
	}
}

func (s *drawerState) InitState() {
	if s.w.Config.Controller != nil {
		s.w.Config.Controller.notify = func() { s.SetState(nil) }
	}
}

func (s *drawerState) Build(ctx widget.BuildContext) widget.Widget {
	th := ctx.Theme
	cfg := s.w.Config
	open := cfg.Controller != nil && cfg.Controller.IsOpen()

	panelWidth := cfg.Width
	if panelWidth <= 0 {
		panelWidth = 280
	}

	return widget.Animated{
		Value:    boolToFloat(open),
		Duration: th.Motion.Moderate,
		Curve:    widget.EaseInOut,
		Builder: func(t float64) widget.Widget {
			if t <= 0 {
				// Fully closed: just the body.
				return s.w.Body
			}

			// Panel slides in from offset -panelWidth to 0.
			panelX := (t - 1.0) * panelWidth

			scrimAlpha := t * 0.48
			scrim := color.Black.WithAlpha(scrimAlpha)

			var closeDrawer func()
			if cfg.Controller != nil {
				closeDrawer = cfg.Controller.Close
			}

			panel := widget.Positioned{
				Top:    widget.Ptr(0),
				Bottom: widget.Ptr(0),
				Left:   widget.Ptr(panelX),
				Width:  widget.Ptr(panelWidth),
				Child: widget.Container{
					Fill:        th.ColorScheme.Surface,
					Border:      th.ColorScheme.Outline,
					BorderWidth: 1,
					Shadow:      th.ColorScheme.Shadow,
					ShadowBlur:  th.Shadow.LG.Blur,
					Padding:     layout.All(th.Space.Unit(4)),
					Child:       cfg.Child,
				},
			}

			var scrimWidget widget.Widget = widget.Positioned{
				Top:    widget.Ptr(0),
				Right:  widget.Ptr(0),
				Bottom: widget.Ptr(0),
				Left:   widget.Ptr(0),
				Child:  widget.Container{Fill: scrim},
			}
			if closeDrawer != nil {
				scrimWidget = widget.GestureDetector{
					OnTap: closeDrawer,
					Child: widget.Positioned{
						Top:    widget.Ptr(0),
						Right:  widget.Ptr(0),
						Bottom: widget.Ptr(0),
						Left:   widget.Ptr(0),
						Child:  widget.Container{Fill: scrim},
					},
				}
			}

			return widget.Stack{
				Children: []widget.Widget{
					s.w.Body,
					scrimWidget,
					panel,
				},
			}
		},
	}
}

// boolToFloat converts a bool to 0.0 or 1.0 for animation targets.
func boolToFloat(v bool) float64 {
	if v {
		return 1
	}
	return 0
}

// ─── Expand helpers ───────────────────────────────────────────────────────────

// expandedRow wraps children in a Row where one child fills remaining space.
func expandedRow(children ...widget.Widget) widget.Widget {
	return widget.Row{
		CrossAxisAlignment: layout.CrossStretch,
		Children:           children,
	}
}

// expandedCol wraps children in a Column where one child fills remaining space.
func expandedCol(children ...widget.Widget) widget.Widget {
	return widget.Column{
		CrossAxisAlignment: layout.CrossStretch,
		Children:           children,
	}
}

// pt is a shorthand for geom.Pt to reduce verbosity inside package.
func pt(x, y float64) geom.Point { return geom.Pt(x, y) }
