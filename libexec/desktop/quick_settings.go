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
	"fmt"

	"avyos.dev/lib/graphics/color"
	"avyos.dev/lib/graphics/layout"
	"avyos.dev/lib/graphics/widget"
)

const quickSettingsW = 360.0
const quickSettingsGap = 12.0
const quickSettingsScreenInset = 12.0

// ─── QuickSettingsPanel ───────────────────────────────────────────────────────

// QuickSettingsPanel is the GNOME-style quick settings popover that floats
// above the right side of the shelf.
type QuickSettingsPanel struct {
	OnClose func()
}

func (qs QuickSettingsPanel) CreateState() widget.State {
	return &qsState{panel: qs}
}

type qsState struct {
	widget.StateBase
	panel QuickSettingsPanel

	// Toggle tiles
	wifi       bool
	bluetooth  bool
	darkMode   bool
	dnd        bool
	nightLight bool
	airplane   bool

	// Sliders [0..100]
	volume     float64
	brightness float64
}

func (s *qsState) InitState() {
	s.wifi = true
	s.volume = 75
	s.brightness = 80
}

func (s *qsState) Build(ctx widget.BuildContext) widget.Widget {
	th := ctx.Theme

	// ── backdrop (transparent; tap outside panel to dismiss) ─────────────
	backdrop := widget.GestureDetector{
		OnTap: s.panel.OnClose,
		Child: widget.Container{Fill: color.Color{A: 0}},
	}

	// ── toggle tiles 2×3 grid ─────────────────────────────────────────────
	toggleTiles := widget.Grid{
		Columns: 2,
		Gap:     8,
		Children: []widget.Widget{
			s.tile(ctx, "network-wireless", "📶", "Wi-Fi",
				s.wifi, func() { s.SetState(func() { s.wifi = !s.wifi }) }),
			s.tile(ctx, "bluetooth", "🔵", "Bluetooth",
				s.bluetooth, func() { s.SetState(func() { s.bluetooth = !s.bluetooth }) }),
			s.tile(ctx, "weather-clear-night", "🌙", "Dark Mode",
				s.darkMode, func() { s.SetState(func() { s.darkMode = !s.darkMode }) }),
			s.tile(ctx, "notification-disabled", "🔕", "Do Not Disturb",
				s.dnd, func() { s.SetState(func() { s.dnd = !s.dnd }) }),
			s.tile(ctx, "night-light", "🌅", "Night Light",
				s.nightLight, func() { s.SetState(func() { s.nightLight = !s.nightLight }) }),
			s.tile(ctx, "airplane-mode", "✈", "Airplane Mode",
				s.airplane, func() { s.SetState(func() { s.airplane = !s.airplane }) }),
		},
	}

	// ── volume slider ─────────────────────────────────────────────────────
	volumeRow := s.sliderRow(ctx, "audio-volume-high", "🔊", s.volume, func(v float64) {
		s.SetState(func() { s.volume = v })
	})

	// ── brightness slider ─────────────────────────────────────────────────
	brightnessRow := s.sliderRow(ctx, "display-brightness", "☀", s.brightness, func(v float64) {
		s.SetState(func() { s.brightness = v })
	})

	// ── settings button ───────────────────────────────────────────────────
	settingsSt := th.TextTheme.LabelMedium
	settingsSt.Color = th.ColorScheme.OnSurface
	settingsBtn := widget.GestureDetector{
		Builder: func(state widget.InteractionState) widget.Widget {
			fill := th.ColorScheme.SurfaceVariant
			if state.Hovered {
				fill = th.ColorScheme.SurfaceContainerHighest
			}
			return widget.Container{
				Fill:    fill,
				Radius:  8,
				Padding: layout.Symmetric(12, 10),
				Child: widget.Row{
					CrossAxisAlignment: layout.CrossCenter,
					Children: []widget.Widget{
						widget.Icon{Name: "preferences-system", Size: 16,
							Fallback: widget.Text{Content: "⚙", Style: &th.TextTheme.LabelMedium},
						},
						widget.SizedBox{Width: 8},
						widget.Text{Content: "Settings", Style: &settingsSt},
					},
				},
			}
		},
	}

	// ── panel card ────────────────────────────────────────────────────────
	panel := widget.Container{
		Width:         quickSettingsW,
		Fill:          th.ColorScheme.Surface,
		Radius:        12,
		Shadow:        th.ColorScheme.Shadow,
		ShadowBlur:    th.Shadow.LG.Blur,
		ShadowOffsetY: -4,
		Padding:       layout.All(16),
		Child: widget.Column{
			Children: []widget.Widget{
				toggleTiles,
				widget.SizedBox{Height: 16},
				widget.Separator{},
				widget.SizedBox{Height: 12},
				volumeRow,
				widget.SizedBox{Height: 10},
				brightnessRow,
				widget.SizedBox{Height: 12},
				widget.Separator{},
				widget.SizedBox{Height: 12},
				settingsBtn,
			},
		},
	}

	return widget.Stack{
		Children: []widget.Widget{
			widget.Positioned{
				Top: widget.Ptr(0.0), Right: widget.Ptr(0.0),
				Bottom: widget.Ptr(0.0), Left: widget.Ptr(0.0),
				Child: backdrop,
			},
			widget.Positioned{
				Right:  widget.Ptr(quickSettingsScreenInset),
				Bottom: widget.Ptr(shelfHeight + quickSettingsGap),
				Child:  panel,
			},
		},
	}
}

// tile builds a single toggle tile (icon + label, highlighted when active).
func (s *qsState) tile(ctx widget.BuildContext, iconName, fallback, label string, active bool, onTap func()) widget.Widget {
	th := ctx.Theme
	labelSt := th.TextTheme.LabelSmall
	return widget.GestureDetector{
		OnTap: onTap,
		Builder: func(state widget.InteractionState) widget.Widget {
			var fill color.Color
			switch {
			case active:
				fill = th.ColorScheme.Primary
				labelSt.Color = th.ColorScheme.Surface
			case state.Hovered:
				fill = th.ColorScheme.SurfaceContainerHighest
				labelSt.Color = th.ColorScheme.OnSurface
			default:
				fill = th.ColorScheme.SurfaceVariant
				labelSt.Color = th.ColorScheme.OnSurfaceVariant
			}
			return widget.Container{
				Fill:    fill,
				Radius:  10,
				Padding: layout.Symmetric(12, 10),
				Child: widget.Row{
					CrossAxisAlignment: layout.CrossCenter,
					Children: []widget.Widget{
						widget.Icon{Name: iconName, Size: 18,
							Fallback: widget.Text{Content: fallback, Style: &th.TextTheme.LabelMedium},
						},
						widget.SizedBox{Width: 8},
						widget.Text{Content: label, Style: &labelSt},
					},
				},
			}
		},
	}
}

// sliderRow builds an icon + slider + percentage label row.
func (s *qsState) sliderRow(ctx widget.BuildContext, iconName, fallback string, value float64, onChange func(float64)) widget.Widget {
	th := ctx.Theme
	pctSt := th.TextTheme.LabelSmall
	pctSt.Color = th.ColorScheme.OnSurfaceVariant
	return widget.Row{
		CrossAxisAlignment: layout.CrossCenter,
		Children: []widget.Widget{
			widget.Icon{Name: iconName, Size: 18,
				Fallback: widget.Text{Content: fallback, Style: &th.TextTheme.LabelMedium},
			},
			widget.SizedBox{Width: 10},
			widget.Expanded{
				Child: widget.Slider{
					Value:     value,
					Min:       0,
					Max:       100,
					OnChanged: onChange,
				},
			},
			widget.SizedBox{Width: 8},
			widget.Text{Content: fmt.Sprintf("%d%%", int(value)), Style: &pctSt},
		},
	}
}
