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
	"strings"

	"avyos.dev/pkg/graphics/color"
	"avyos.dev/pkg/graphics/layout"
	"avyos.dev/pkg/graphics/widget"
)

const (
	launcherW = 760.0
	launcherH = 540.0
)

type LauncherPanel struct {
	Apps       []launcherApp
	OnLaunch   func(app launcherApp)
	OnLogout   func()
	OnReboot   func()
	OnPoweroff func()
	OnClose    func()
}

func (lp LauncherPanel) CreateState() widget.State {
	return &launcherState{panel: lp}
}

type launcherState struct {
	widget.StateBase
	panel LauncherPanel
	query string
}

func (s *launcherState) Build(ctx widget.BuildContext) widget.Widget {
	th := ctx.Theme
	apps := s.filteredApps()

	backdrop := widget.GestureDetector{
		OnTap: s.panel.OnClose,
		Child: widget.Container{
			Fill: color.Color{A: 0},
		},
	}

	panel := widget.Container{
		Width:         launcherW,
		Height:        launcherH,
		Fill:          th.ColorScheme.Surface,
		Shadow:        th.ColorScheme.Shadow,
		ShadowBlur:    th.Shadow.LG.Blur,
		ShadowOffsetY: -4,
		Radius:        16,
		Padding:       layout.All(20),
		Child: widget.Column{
			CrossAxisAlignment: layout.CrossStretch,
			Children: []widget.Widget{
				widget.Expanded{
					Child: widget.Scroll{
						Axis:  layout.Vertical,
						Child: s.buildIconFlow(ctx, apps),
					},
				},
				widget.Container{Height: 1, Fill: th.ColorScheme.OutlineVariant},
				widget.Container{
					Height: 28,
					Child: widget.Row{
						CrossAxisAlignment: layout.CrossCenter,
						Children: []widget.Widget{
							widget.Spacer{},
							s.buildPowerIcon(ctx, "system-log-out", s.panel.OnLogout),
							widget.SizedBox{Width: 10},
							s.buildPowerIcon(ctx, "system-reboot", s.panel.OnReboot),
							widget.SizedBox{Width: 10},
							s.buildPowerIcon(ctx, "system-shutdown", s.panel.OnPoweroff),
						},
					},
				},
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
				Left:   widget.Ptr(quickSettingsScreenInset),
				Bottom: widget.Ptr(shelfHeight + quickSettingsGap),
				Child:  panel,
			},
		},
	}
}

func (s *launcherState) buildIconFlow(ctx widget.BuildContext, apps []launcherApp) widget.Widget {
	th := ctx.Theme
	nameSt := th.TextTheme.LabelMedium
	nameSt.Color = th.ColorScheme.OnSurface

	items := make([]widget.Widget, 0, len(apps))
	for _, app := range apps {
		app := app
		items = append(items, widget.GestureDetector{
			OnTap: func() { s.launch(app) },
			Builder: func(state widget.InteractionState) widget.Widget {
				fill := color.Color{A: 0}
				if state.Hovered {
					fill = th.ColorScheme.SurfaceContainer
				}
				return widget.Container{
					Width:       108,
					Height:      104,
					Fill:        fill,
					Radius:      14,
					Border:      color.Color{A: 0},
					BorderWidth: 0,
					Padding:     layout.All(12),
					Child: widget.Column{
						CrossAxisAlignment: layout.CrossCenter,
						MainAxisAlignment:  layout.MainCenter,
						Children: []widget.Widget{
							widget.SizedBox{Width: 40, Height: 40, Child: s.appIconWidget(app, 40, ctx)},
							widget.SizedBox{Height: 10},
							widget.Text{Content: app.Name, Style: &nameSt},
						},
					},
				}
			},
		})
	}

	if len(items) == 0 {
		empty := th.TextTheme.BodyMedium
		empty.Color = th.ColorScheme.OnSurfaceVariant
		return widget.Container{
			Height: 376,
			Child:  widget.Center(widget.Text{Content: "No apps found", Style: &empty}),
		}
	}

	return widget.Wrap{
		Spacing:    12,
		RunSpacing: 12,
		Children:   items,
	}
}

func (s *launcherState) buildPowerIcon(ctx widget.BuildContext, glyph string, onTap func()) widget.Widget {
	th := ctx.Theme
	text := th.TextTheme.LabelLarge
	text.Color = th.ColorScheme.OnSurface

	return widget.GestureDetector{
		OnTap: onTap,
		Builder: func(state widget.InteractionState) widget.Widget {
			fill := th.ColorScheme.SurfaceContainer
			if state.Hovered {
				fill = th.ColorScheme.SurfaceContainerHighest
			}
			return widget.Container{
				Width:       28,
				Height:      28,
				Fill:        fill,
				Radius:      14,
				Border:      th.ColorScheme.OutlineVariant,
				BorderWidth: 1,
				Child: widget.Icon{
					Name:     glyph,
					Size:     22,
					Fallback: widget.Text{Content: glyph, Style: &text},
				},
			}
		},
	}
}

func (s *launcherState) filteredApps() []launcherApp {
	query := strings.TrimSpace(strings.ToLower(s.query))
	if query == "" {
		return s.panel.Apps
	}

	filtered := make([]launcherApp, 0, len(s.panel.Apps))
	for _, app := range s.panel.Apps {
		if strings.Contains(strings.ToLower(app.Name), query) || strings.Contains(strings.ToLower(app.ID), query) {
			filtered = append(filtered, app)
		}
	}
	return filtered
}

func (s *launcherState) launch(app launcherApp) {
	if s.panel.OnLaunch != nil {
		s.panel.OnLaunch(app)
	}
}

func (s *launcherState) appIconWidget(app launcherApp, size float64, ctx widget.BuildContext) widget.Widget {
	th := ctx.Theme
	if app.Icon != nil {
		return widget.SizedBox{
			Width:  size,
			Height: size,
			Child:  widget.Image{Source: app.Icon, Fit: widget.ImageFitContain},
		}
	}
	initial := "?"
	if app.Name != "" {
		initial = strings.ToUpper(string([]rune(app.Name)[0]))
	}
	return widget.Container{
		Width:  size,
		Height: size,
		Fill:   th.ColorScheme.PrimaryContainer,
		Radius: 6,
		Child:  widget.Center(widget.Text{Content: initial, Style: &th.TextTheme.LabelLarge}),
	}
}
