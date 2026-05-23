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

// Package font defines font abstractions for the graphics framework.
package font

import (
	"image"

	"avyos.dev/lib/graphics/canvas"
	"avyos.dev/lib/graphics/color"
	"avyos.dev/lib/graphics/geom"
)

// Font extends canvas.Typeface with metadata.
type Font interface {
	canvas.Typeface
	Name() string
	Style() string
}

// Provider loads and manages fonts.
type Provider interface {
	Load(family, style string) (Font, error)
	Default() Font
}

// Stub is a no-op Font for use when no real font is available.
type Stub struct{}

func (Stub) Name() string                                                           { return "Stub" }
func (Stub) Style() string                                                          { return "Regular" }
func (Stub) DrawRune(_ rune, _ float64, _ *image.RGBA, _, _ float64, _ color.Color) {}
func (Stub) RuneAdvance(_ rune, size float64) float64                               { return size * 0.6 }
func (Stub) LineHeight(size float64) float64                                        { return size * 1.2 }

// StubProvider is a Provider that always returns Stub.
type StubProvider struct{ f Stub }

func (p *StubProvider) Load(_, _ string) (Font, error) { return p.f, nil }
func (p *StubProvider) Default() Font                  { return p.f }

// MeasureString returns the bounding box of text rendered with f at size.
func MeasureString(text string, f canvas.Typeface, size float64) geom.Size {
	var w float64
	for _, r := range text {
		w += f.RuneAdvance(r, size)
	}
	return geom.Sz(w, f.LineHeight(size))
}
