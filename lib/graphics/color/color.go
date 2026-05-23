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

// Package color provides color types and blending operations.
package color

import "math"

// Color represents an RGBA color with components in [0,1].
type Color struct{ R, G, B, A float64 }

func (c Color) RGBA8() (r, g, b, a uint8) {
	clamp := func(v float64) uint8 {
		if v <= 0 {
			return 0
		}
		if v >= 1 {
			return 255
		}
		return uint8(v*255 + 0.5)
	}
	return clamp(c.R), clamp(c.G), clamp(c.B), clamp(c.A)
}

// ARGB32 returns the color packed as 0xAARRGGBB.
func (c Color) ARGB32() uint32 {
	r, g, b, a := c.RGBA8()
	return uint32(a)<<24 | uint32(r)<<16 | uint32(g)<<8 | uint32(b)
}

// BGRA32 returns the color packed as 0xAABBGGRR (DRM/Wayland XRGB8888 format).
func (c Color) BGRA32() uint32 {
	r, g, b, a := c.RGBA8()
	return uint32(a)<<24 | uint32(b)<<16 | uint32(g)<<8 | uint32(r)
}

// FromHex parses 0xRRGGBB or 0xAARRGGBB.
func FromHex(hex uint32) Color {
	if hex <= 0x00FFFFFF {
		return Color{
			R: float64((hex>>16)&0xFF) / 255,
			G: float64((hex>>8)&0xFF) / 255,
			B: float64(hex&0xFF) / 255,
			A: 1,
		}
	}
	return Color{
		R: float64((hex>>16)&0xFF) / 255,
		G: float64((hex>>8)&0xFF) / 255,
		B: float64(hex&0xFF) / 255,
		A: float64((hex>>24)&0xFF) / 255,
	}
}

// FromRGBA8 creates a Color from 0-255 components.
func FromRGBA8(r, g, b, a uint8) Color {
	return Color{float64(r) / 255, float64(g) / 255, float64(b) / 255, float64(a) / 255}
}

func (c Color) WithAlpha(a float64) Color { return Color{c.R, c.G, c.B, a} }

// Lerp linearly interpolates between c and other.
func (c Color) Lerp(other Color, t float64) Color {
	lerp := func(a, b float64) float64 { return a + (b-a)*t }
	return Color{lerp(c.R, other.R), lerp(c.G, other.G), lerp(c.B, other.B), lerp(c.A, other.A)}
}

// Over composites c over dst (Porter-Duff "over").
func (c Color) Over(dst Color) Color {
	a := c.A + dst.A*(1-c.A)
	if a == 0 {
		return Transparent
	}
	blend := func(src, d float64) float64 { return (src*c.A + d*dst.A*(1-c.A)) / a }
	return Color{blend(c.R, dst.R), blend(c.G, dst.G), blend(c.B, dst.B), a}
}

// Luminance returns the relative luminance of the color.
func (c Color) Luminance() float64 {
	lin := func(v float64) float64 {
		if v <= 0.04045 {
			return v / 12.92
		}
		return math.Pow((v+0.055)/1.055, 2.4)
	}
	return 0.2126*lin(c.R) + 0.7152*lin(c.G) + 0.0722*lin(c.B)
}

// Predefined colors.
var (
	Transparent = Color{0, 0, 0, 0}
	Black       = Color{0, 0, 0, 1}
	White       = Color{1, 1, 1, 1}
	Red         = Color{1, 0, 0, 1}
	Green       = Color{0, 0.502, 0, 1}
	Lime        = Color{0, 1, 0, 1}
	Blue        = Color{0, 0, 1, 1}
	Yellow      = Color{1, 1, 0, 1}
	Cyan        = Color{0, 1, 1, 1}
	Magenta     = Color{1, 0, 1, 1}
	Gray        = Color{0.502, 0.502, 0.502, 1}
	LightGray   = Color{0.827, 0.827, 0.827, 1}
	DarkGray    = Color{0.251, 0.251, 0.251, 1}
	Orange      = Color{1, 0.647, 0, 1}
	Pink        = Color{1, 0.753, 0.796, 1}
	Purple      = Color{0.502, 0, 0.502, 1}
)
