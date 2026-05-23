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

// Opacity fades its child by painting a semi-transparent overlay.
// This is an approximation suitable for opaque backgrounds.
//
// Value 1.0 is a no-op pass-through. Value 0.0 skips painting entirely.
//
//	widget.Opacity{Value: 0.4, Child: child}
package widget

import (
	"avyos.dev/pkg/graphics/color"
	"avyos.dev/pkg/graphics/geom"
	"avyos.dev/pkg/graphics/layout"
	"avyos.dev/pkg/graphics/paint"
)

// Opacity makes its Child partially transparent.
// Value is clamped to [0, 1]. The zero value (0.0) is fully transparent.
type Opacity struct {
	Value float64
	Child Widget
}

func (o Opacity) RenderChildren(c layout.BoxConstraints, pctx *paint.Context, offset geom.Point, cr ChildRenderer) geom.Size {
	if o.Child == nil {
		return c.Smallest()
	}
	if o.Value >= 1.0 {
		return cr.Render(o.Child, c, offset, "child")
	}
	if o.Value <= 0.0 {
		return cr.Measure(o.Child, c, "child")
	}
	sz := cr.Render(o.Child, c, offset, "child")
	if pctx != nil {
		// Approximate opacity by overlaying a semi-transparent black rect.
		// This works well for opaque backgrounds.
		pctx.FillRect(
			geom.NewRect(offset.X, offset.Y, sz.Width, sz.Height),
			color.Color{A: 1.0 - o.Value},
		)
	}
	return sz
}
