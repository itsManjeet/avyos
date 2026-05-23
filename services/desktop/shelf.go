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

	"avyos.dev/pkg/graphics/layout"
	"avyos.dev/pkg/graphics/theme"
	"avyos.dev/pkg/graphics/widget"
)

// Shelf is the desktop taskbar pinned to the bottom of the screen.
// It shows a launcher toggle button on the left, one chip per open window in
// the center, and a system tray + clock on the right.
type Shelf struct {
	Wins                  []*ManagedWindow
	FocusedWinID          uint32
	OnFocusWin            func(uint32)
	OnMinimizeWin         func(uint32)
	OnCloseWin            func(uint32)
	OnToggleLauncher      func()
	OnToggleNotifications func()
	OnToggleQuickSettings func()
	LauncherOpen          bool
	NotificationsOpen     bool
	QuickSettingsOpen     bool
	UnreadCount           int
	Clock                 string
}

func (s Shelf) Build(ctx widget.BuildContext) widget.Widget {
	th := ctx.Theme
	iconSlot := func(fill widget.Widget) widget.Widget {
		return widget.Container{
			Width:  20,
			Height: 20,
			Child:  widget.Center(fill),
		}
	}

	// ── launcher toggle button ──────────────────────────────────────────
	launcherBtn := widget.GestureDetector{
		OnTap: s.OnToggleLauncher,
		Builder: func(state widget.InteractionState) widget.Widget {
			fill := th.ColorScheme.Surface
			if s.LauncherOpen || state.Hovered {
				fill = th.ColorScheme.SurfaceContainerHighest
			}
			return widget.Container{
				Width:   shelfHeight,
				Height:  shelfHeight,
				Fill:    fill,
				Padding: layout.All(12),
				Child: widget.Icon{Name: "start-here", Size: 24,
					Fallback: widget.Text{
						Content: "⊞",
						Style:   &th.TextTheme.LabelLarge,
					},
				},
			}
		},
	}

	// ── window chips ────────────────────────────────────────────────────
	chipSt := th.TextTheme.LabelSmall
	chips := make([]widget.Widget, 0, len(s.Wins))
	for _, mw := range s.Wins {
		mw := mw
		focused := mw.Win.ID == s.FocusedWinID
		label := mw.Win.AppName
		if label == "" {
			label = mw.Win.Title
		}
		if label == "" {
			label = "Window"
		}

		chips = append(chips, widget.GestureDetector{
			OnTap: func() {
				if mw.Minimized {
					// Restore: unminimize and focus.
					if s.OnFocusWin != nil {
						s.OnFocusWin(mw.Win.ID)
					}
				} else if mw.Win.ID == s.FocusedWinID {
					// Minimize the currently focused window.
					if s.OnMinimizeWin != nil {
						s.OnMinimizeWin(mw.Win.ID)
					}
				} else {
					if s.OnFocusWin != nil {
						s.OnFocusWin(mw.Win.ID)
					}
				}
			},
			Builder: func(state widget.InteractionState) widget.Widget {
				fill := th.ColorScheme.Surface
				if focused && !mw.Minimized {
					fill = th.ColorScheme.SurfaceContainerHighest
				} else if state.Hovered {
					fill = th.ColorScheme.SurfaceVariant
				}
				st := chipSt
				if focused && !mw.Minimized {
					st.Color = th.ColorScheme.OnSurface
				} else {
					st.Color = th.ColorScheme.OnSurfaceVariant
				}
				return widget.Container{
					Height:  shelfHeight,
					Width:   120,
					Fill:    fill,
					Padding: layout.Symmetric(12, 0),
					Child: widget.Row{
						CrossAxisAlignment: layout.CrossCenter,
						Children: []widget.Widget{
							shelfWindowIcon(mw, th),
							widget.SizedBox{Width: 6},
							widget.Text{Content: label, Style: &st},
						},
					},
				}
			},
		})
	}

	// ── center: window chips row ─────────────────────────────────────────
	var windowArea widget.Widget
	if len(chips) > 0 {
		windowArea = widget.Scroll{
			Axis: layout.Horizontal,
			Child: widget.Row{
				CrossAxisAlignment: layout.CrossCenter,
				Children:           chips,
			},
		}
	} else {
		windowArea = widget.SizedBox{}
	}

	// ── system tray + clock (right side, taps open quick settings) ───────
	trayIconSt := th.TextTheme.LabelSmall
	trayIconSt.Color = th.ColorScheme.OnSurfaceVariant
	clockSt := th.TextTheme.LabelMedium
	clockSt.Color = th.ColorScheme.OnSurface

	notificationBtn := widget.GestureDetector{
		OnTap: s.OnToggleNotifications,
		Builder: func(state widget.InteractionState) widget.Widget {
			fill := th.ColorScheme.Surface
			if s.NotificationsOpen || state.Hovered {
				fill = th.ColorScheme.SurfaceContainerHighest
			}

			bell := widget.Container{
				Width:  shelfHeight,
				Height: shelfHeight,
				Fill:   fill,
				Child: widget.Center(
					iconSlot(widget.Icon{Name: "notifications", Size: 18,
						Fallback: widget.Text{Content: "🔔", Style: &trayIconSt},
					}),
				),
			}
			if s.UnreadCount <= 0 {
				return bell
			}

			badgeSt := th.TextTheme.LabelSmall
			badgeSt.Color = th.ColorScheme.Surface
			badgeText := "9+"
			if s.UnreadCount < 10 {
				badgeText = fmt.Sprintf("%d", s.UnreadCount)
			}
			return widget.SizedBox{
				Width:  shelfHeight,
				Height: shelfHeight,
				Child: widget.Stack{
					Children: []widget.Widget{
						bell,
						widget.Positioned{
							Top:   widget.Ptr(8),
							Right: widget.Ptr(4),
							Child: widget.Container{
								Fill:    th.ColorScheme.Error,
								Radius:  9,
								Padding: layout.Symmetric(5, 1),
								Child:   widget.Text{Content: badgeText, Style: &badgeSt},
							},
						},
					},
				},
			}
		},
	}

	tray := widget.GestureDetector{
		OnTap: s.OnToggleQuickSettings,
		Builder: func(state widget.InteractionState) widget.Widget {
			fill := th.ColorScheme.Surface
			if s.QuickSettingsOpen || state.Hovered {
				fill = th.ColorScheme.SurfaceContainerHighest
			}
			return widget.Container{
				Height:  shelfHeight,
				Fill:    fill,
				Padding: layout.Symmetric(12, 0),
				Child: widget.Row{
					CrossAxisAlignment: layout.CrossCenter,
					Children: []widget.Widget{
						iconSlot(widget.Icon{Name: "network-wireless", Size: 18,
							Fallback: widget.Text{Content: "📶", Style: &trayIconSt},
						}),
						widget.SizedBox{Width: 6},
						iconSlot(widget.Icon{Name: "audio-volume-high", Size: 18,
							Fallback: widget.Text{Content: "🔊", Style: &trayIconSt},
						}),
						widget.SizedBox{Width: 6},
						iconSlot(widget.Icon{Name: "battery-full", Size: 18,
							Fallback: widget.Text{Content: "🔋", Style: &trayIconSt},
						}),
						widget.SizedBox{Width: 10},
						widget.Text{Content: s.Clock, Style: &clockSt},
						widget.SizedBox{Width: 2},
					},
				},
			}
		},
	}

	// ── assemble ─────────────────────────────────────────────────────────
	return widget.Container{
		Fill:   th.ColorScheme.Surface,
		Border: th.ColorScheme.Outline,
		Height: shelfHeight,
		Child: widget.Row{
			CrossAxisAlignment: layout.CrossCenter,
			Children: []widget.Widget{
				launcherBtn,
				widget.Expanded{Child: windowArea},
				widget.Row{
					CrossAxisAlignment: layout.CrossCenter,
					Children: []widget.Widget{
						notificationBtn,
						widget.SizedBox{Width: 4},
						tray,
					},
				},
			},
		},
	}
}

func shelfWindowIcon(mw *ManagedWindow, th *theme.ThemeData) widget.Widget {
	if mw != nil && mw.Icon != nil {
		return widget.SizedBox{
			Width:  16,
			Height: 16,
			Child:  widget.Image{Source: mw.Icon, Fit: widget.ImageFitContain},
		}
	}
	return widget.Icon{Name: mw.Win.Icon, Size: 16,
		Fallback: widget.Container{Width: 16, Height: 16, Fill: th.ColorScheme.SurfaceVariant, Radius: 3},
	}
}
