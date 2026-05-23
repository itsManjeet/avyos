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

// Package renderer defines the Renderer interface and provides a Basic implementation.
//
// A Renderer converts a scene.Scene into pixels on a canvas.Canvas.
// Different renderers can apply effects, caching, or GPU acceleration.
package renderer

import (
	"avyos.dev/lib/graphics/canvas"
	"avyos.dev/lib/graphics/scene"
)

// Renderer converts a scene into pixels on a canvas.
type Renderer interface {
	Render(sc *scene.Scene, c canvas.Canvas)
}

// Basic is the default renderer: delegates directly to scene.Scene.Draw.
type Basic struct{}

func NewBasic() *Basic { return &Basic{} }

func (r *Basic) Render(sc *scene.Scene, c canvas.Canvas) { sc.Draw(c) }
