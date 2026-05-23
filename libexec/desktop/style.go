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
	"avyos.dev/lib/graphics/color"
	"avyos.dev/lib/graphics/theme"
)

const (
	desktopPanelRadius   = 24.0
	desktopPopupRadius   = 30.0
	desktopShelfInset    = 14.0
	desktopShelfRadius   = 24.0
	desktopSurfaceRadius = 20.0
	desktopControlRadius = 16.0
	desktopPillRadius    = 999.0
)

func desktopShelfReserve() float64 {
	return shelfHeight + desktopShelfInset
}

func desktopPanelBottomInset() float64 {
	return desktopShelfReserve() + quickSettingsGap
}

func desktopShadow() color.Color {
	return color.FromHex(0x3E2D1C).WithAlpha(0.18)
}

func desktopPopupShadow() color.Color {
	return color.FromHex(0x3E2D1C).WithAlpha(0.42)
}

func desktopLightShadow() color.Color {
	return color.FromHex(0x3E2D1C).WithAlpha(0.08)
}

func desktopAccentShadow(th *theme.ThemeData) color.Color {
	return th.ColorScheme.Primary.WithAlpha(0.58)
}

func desktopPillShadow() color.Color {
	return color.FromHex(0x3E2D1C).WithAlpha(0.26)
}

func hoverFill(th *theme.ThemeData, hovered bool) color.Color {
	if hovered {
		return th.ColorScheme.SurfaceContainer
	}
	return color.Transparent
}
