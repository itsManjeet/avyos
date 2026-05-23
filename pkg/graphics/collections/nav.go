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
	"avyos.dev/pkg/graphics/layout"
	"avyos.dev/pkg/graphics/widget"
)

// ─── NavDestination ───────────────────────────────────────────────────────────

// NavDestination is a single navigation target used by both [NavBar] and [BottomNav].
type NavDestination struct {
	// Label is the display name.
	Label string
	// Icon is the icon name passed to widget.Icon.
	// If empty, renders a bullet placeholder.
	Icon string
}

// ─── NavBar ───────────────────────────────────────────────────────────────────

// NavBar is a persistent vertical navigation sidebar for desktop/tablet layouts.
//
// It renders a scrollable list of [NavDestination] items with an optional Header
// (e.g. logo or app name) and Footer (e.g. settings link). Set Compact to
// render icon-only items.
//
// NavBar is a [widget.Buildable].
type NavBar struct {
	Destinations []NavDestination
	Selected     int
	OnSelected   func(int)
	Header       widget.Widget
	Footer       widget.Widget
	// Compact collapses the bar to icon-only width.
	Compact bool
	// Width overrides the default bar width. 0 uses theme default.
	Width float64
}

func (n NavBar) Build(ctx widget.BuildContext) widget.Widget {
	th := ctx.Theme

	w := n.Width
	if w == 0 {
		if n.Compact {
			w = th.Space.Unit(14)
		} else {
			w = th.Space.Unit(60)
		}
	}

	items := make([]widget.Widget, 0, len(n.Destinations))
	for i, dest := range n.Destinations {
		i, dest := i, dest
		active := i == n.Selected
		items = append(items, navItem(dest, active, n.Compact, func() {
			if n.OnSelected != nil {
				n.OnSelected(i)
			}
		}, ctx))
	}

	colChildren := make([]widget.Widget, 0, 4)
	if n.Header != nil {
		colChildren = append(colChildren, n.Header, widget.SizedBox{Height: th.Space.Unit(2)})
	}
	colChildren = append(colChildren, widget.Expanded{Child: widget.Scroll{
		Axis: layout.Vertical,
		Child: widget.Column{
			CrossAxisAlignment: layout.CrossStretch,
			Children:           items,
		},
	}})
	if n.Footer != nil {
		colChildren = append(colChildren, widget.SizedBox{Height: th.Space.Unit(2)}, n.Footer)
	}

	return widget.Container{
		Fill:        th.ColorScheme.SurfaceVariant,
		Border:      th.ColorScheme.Outline,
		BorderWidth: 1,
		Width:       w,
		Padding:     layout.Symmetric(th.Space.Unit(2), th.Space.Unit(3)),
		Child: widget.Column{
			CrossAxisAlignment: layout.CrossStretch,
			Children:           colChildren,
		},
	}
}

// navItem builds one navigation row.
func navItem(dest NavDestination, active, compact bool, onTap func(), ctx widget.BuildContext) widget.Widget {
	th := ctx.Theme

	labelSt := th.TextTheme.LabelMedium
	if active {
		labelSt.Color = th.ColorScheme.Primary
	} else {
		labelSt.Color = th.ColorScheme.OnSurfaceVariant
	}

	var iconW widget.Widget
	if dest.Icon != "" {
		iconW = widget.Icon{Name: dest.Icon, Size: 20}
	} else {
		dot := widget.Container{
			Fill:   labelSt.Color,
			Width:  6,
			Height: 6,
			Radius: 999,
		}
		iconW = widget.SizedBox{
			Width:  20,
			Height: 20,
			Child:  widget.Center(dot),
		}
	}

	var inner widget.Widget
	if compact {
		inner = widget.Center(iconW)
	} else {
		inner = widget.Row{
			CrossAxisAlignment: layout.CrossCenter,
			Children: []widget.Widget{
				iconW,
				widget.SizedBox{Width: th.Space.Unit(3)},
				widget.Text{Content: dest.Label, Style: &labelSt},
			},
		}
	}

	activeBg := color.Transparent
	if active {
		activeBg = th.ColorScheme.PrimaryContainer
	}

	return widget.GestureDetector{
		OnTap: onTap,
		Builder: func(state widget.InteractionState) widget.Widget {
			bg := activeBg
			if !active && state.Hovered {
				bg = th.ColorScheme.SurfaceContainer
			}
			return widget.Container{
				Fill:    bg,
				Radius:  th.Shape.MediumRadius,
				Padding: layout.Symmetric(th.Space.Unit(3), th.Space.Unit(2)),
				Child:   inner,
			}
		},
	}
}

// ─── BottomNav ────────────────────────────────────────────────────────────────

// BottomNav is a horizontal bottom navigation bar for mobile layouts.
// It renders up to ~5 destinations as icon+label tappable items.
//
// BottomNav is a [widget.Buildable].
type BottomNav struct {
	Destinations []NavDestination
	Selected     int
	OnSelected   func(int)
}

func (b BottomNav) Build(ctx widget.BuildContext) widget.Widget {
	th := ctx.Theme

	items := make([]widget.Widget, 0, len(b.Destinations))
	for i, dest := range b.Destinations {
		i, dest := i, dest
		active := i == b.Selected
		items = append(items, bottomNavItem(dest, active, func() {
			if b.OnSelected != nil {
				b.OnSelected(i)
			}
		}, ctx))
	}

	return widget.Container{
		Fill:        th.ColorScheme.Surface,
		Border:      th.ColorScheme.Outline,
		BorderWidth: 1,
		Padding:     layout.Symmetric(0, th.Space.Unit(1)),
		Child: widget.Row{
			MainAxisAlignment:  layout.MainSpaceEvenly,
			CrossAxisAlignment: layout.CrossCenter,
			Children:           items,
		},
	}
}

func bottomNavItem(dest NavDestination, active bool, onTap func(), ctx widget.BuildContext) widget.Widget {
	th := ctx.Theme

	iconColor := th.ColorScheme.OnSurfaceVariant
	if active {
		iconColor = th.ColorScheme.Primary
	}

	labelSt := th.TextTheme.LabelSmall
	labelSt.Color = iconColor

	var iconW widget.Widget
	if dest.Icon != "" {
		iconW = widget.Icon{Name: dest.Icon, Size: 24}
	} else {
		iconW = widget.Container{Fill: iconColor, Width: 8, Height: 8, Radius: 999}
	}

	return widget.GestureDetector{
		OnTap: onTap,
		Builder: func(state widget.InteractionState) widget.Widget {
			bg := color.Transparent
			if state.Hovered || state.Pressed {
				bg = th.ColorScheme.SurfaceContainer
			}
			return widget.Container{
				Fill:    bg,
				Radius:  th.Shape.LargeRadius,
				Padding: layout.Symmetric(th.Space.Unit(4), th.Space.Unit(2)),
				Child: widget.Column{
					MainAxisAlignment:  layout.MainCenter,
					CrossAxisAlignment: layout.CrossCenter,
					Children: []widget.Widget{
						iconW,
						widget.SizedBox{Height: th.Space.Unit(1)},
						widget.Text{Content: dest.Label, Style: &labelSt},
					},
				},
			}
		},
	}
}
