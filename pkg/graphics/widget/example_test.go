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

// Package widget — example usage.
//
// This file demonstrates how to compose UIs from the minimal widget set.
// Every UI element in the example is built from the 17 base widget types;
// no special-purpose widget is invented.
package widget_test

import (
	"avyos.dev/pkg/graphics/geom"
	"avyos.dev/pkg/graphics/layout"
	"avyos.dev/pkg/graphics/theme"
	"avyos.dev/pkg/graphics/widget"
)

// ─── SettingsPanel ────────────────────────────────────────────────────────────

// SettingsPanel is a settings screen composed entirely from base widgets.
// State is held in the struct; Build returns a fresh widget tree each frame.
type SettingsPanel struct {
	username      string
	darkMode      bool
	notifications bool
	volume        float64
}

func (p *SettingsPanel) Build(ctx widget.BuildContext) widget.Widget {
	th := ctx.Theme

	// section renders a labeled card around its children.
	section := func(title string, children ...widget.Widget) widget.Widget {
		heading := th.TextTheme.LabelLarge
		heading.Color = th.ColorScheme.OnSurfaceVariant

		rows := make([]widget.Widget, 0, 2+len(children))
		rows = append(rows,
			widget.Text{Content: title, Style: &heading},
			widget.SizedBox{Height: th.Space.Unit(3)},
		)
		rows = append(rows, children...)

		return widget.Container{
			Fill:        th.ColorScheme.Surface,
			Border:      th.ColorScheme.Outline,
			BorderWidth: 1,
			Radius:      th.Shape.LargeRadius,
			Padding:     layout.All(th.Space.Unit(4)),
			Child: widget.Column{
				CrossAxisAlignment: layout.CrossStretch,
				Children:           rows,
			},
		}
	}

	// labelRow renders a label on the left and a trailing control on the right.
	labelRow := func(label string, trailing widget.Widget) widget.Widget {
		labelStyle := th.TextTheme.BodyMedium
		labelStyle.Color = th.ColorScheme.OnSurface
		return widget.Row{
			CrossAxisAlignment: layout.CrossCenter,
			Children: []widget.Widget{
				widget.Text{Content: label, Style: &labelStyle},
				widget.Spacer{},
				trailing,
			},
		}
	}

	// ── Sections ─────────────────────────────────────────────────────────────
	profile := section("Profile",
		widget.TextInput{
			Value: &p.username,
			Label: "Username",
			Hint:  "Enter your username",
		},
	)

	appearance := section("Appearance",
		labelRow("Dark mode", widget.Switch{
			Value:     p.darkMode,
			OnChanged: func(v bool) { p.darkMode = v },
		}),
		widget.SizedBox{Height: th.Space.Unit(3)},
		labelRow("Notifications", widget.Checkbox{
			Value:     p.notifications,
			OnChanged: func(v bool) { p.notifications = v },
		}),
	)

	audio := section("Audio",
		widget.Slider{
			Value:     p.volume,
			Min:       0,
			Max:       100,
			OnChanged: func(v float64) { p.volume = v },
		},
	)

	danger := section("Danger Zone",
		widget.Button{
			Child:     widget.Text{Content: "Delete account"},
			Variant:   widget.ButtonOutline,
			Tone:      widget.ButtonDanger,
			OnPressed: func() {},
		},
	)

	// ── Page layout (scrollable column) ──────────────────────────────────────
	titleStyle := th.TextTheme.TitleLarge
	titleStyle.Color = th.ColorScheme.OnSurface

	return widget.Scroll{
		Child: widget.Padding{
			Insets: layout.Symmetric(th.Space.Unit(8), th.Space.Unit(4)),
			Child: widget.Column{
				CrossAxisAlignment: layout.CrossStretch,
				Children: []widget.Widget{
					widget.Text{Content: "Settings", Style: &titleStyle},
					widget.SizedBox{Height: th.Space.Unit(6)},
					profile,
					widget.SizedBox{Height: th.Space.Unit(4)},
					appearance,
					widget.SizedBox{Height: th.Space.Unit(4)},
					audio,
					widget.SizedBox{Height: th.Space.Unit(4)},
					danger,
				},
			},
		},
	}
}

// Compile-time assertion: SettingsPanel is a Buildable.
var _ widget.Buildable = (*SettingsPanel)(nil)

// ─── Frame usage ──────────────────────────────────────────────────────────────

// ExampleNewFrame shows the typical app loop.
// Replace the noop canvas with a real backend surface.
func ExampleNewFrame() {
	th := theme.Light()
	frame := widget.NewFrame(th, geom.Sz(800, 600))
	panel := &SettingsPanel{username: "alice", volume: 75}

	// In the real loop, acquire a canvas from the backend surface:
	//
	//   for frame.IsDirty() {
	//       canvas := surface.Begin()
	//       frame.Render(panel, canvas)
	//       surface.Present(canvas)
	//   }
	_ = frame
	_ = panel
}

// ─── Composition patterns ─────────────────────────────────────────────────────

// card shows that "Card" is not a widget — it is a Container composition.
// No special Card type needed; the composition is self-documenting.
func card(title, body string, th *theme.ThemeData) widget.Widget {
	titleStyle := th.TextTheme.TitleSmall
	titleStyle.Color = th.ColorScheme.OnSurface
	bodyStyle := th.TextTheme.BodySmall
	bodyStyle.Color = th.ColorScheme.OnSurfaceVariant

	return widget.Container{
		Fill:          th.ColorScheme.Surface,
		Radius:        th.Shape.LargeRadius,
		Shadow:        th.ColorScheme.Shadow,
		ShadowBlur:    th.Shadow.MD.Blur,
		ShadowOffsetY: th.Shadow.MD.OffsetY,
		Padding:       layout.All(th.Space.Unit(4)),
		Child: widget.Column{
			CrossAxisAlignment: layout.CrossStart,
			Children: []widget.Widget{
				widget.Text{Content: title, Style: &titleStyle},
				widget.SizedBox{Height: th.Space.Unit(2)},
				widget.Text{Content: body, Style: &bodyStyle},
			},
		},
	}
}

// badge shows that overlaid indicators are just Stack + Positioned.
// No special Badge type needed.
func badge(child widget.Widget, count int, th *theme.ThemeData) widget.Widget {
	if count <= 0 {
		return child
	}
	dot := widget.Container{
		Fill:   th.ColorScheme.Error,
		Radius: 999,
		Width:  8,
		Height: 8,
	}
	return widget.Stack{
		Children: []widget.Widget{
			child,
			widget.Positioned{
				Top:   widget.Ptr(0),
				Right: widget.Ptr(0),
				Child: dot,
			},
		},
	}
}

var _ = card
var _ = badge
