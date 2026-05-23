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

// Icon renders a themed icon by name from the default icon provider.
//
//	widget.Icon{Name: "folder", Size: 24}
package widget

import (
	"avyos.dev/pkg/graphics/icons"
)

// Icon renders a single themed icon.
// Name is the icon name within the theme. Use the exact icon name, including
// any "-symbolic" suffix when you want that variant.
// Theme is the icon theme name; empty means the provider default.
// Size is in logical pixels; 0 defaults to 24.
// Fallback is rendered if the icon cannot be loaded.
type Icon struct {
	Name     string
	Theme    string
	Size     float64
	Fallback Widget
}

func (ic Icon) Build(_ BuildContext) Widget {
	size := ic.Size
	if size <= 0 {
		size = 24
	}

	img, err := icons.Load(ic.Name, int(size))
	if err != nil || img == nil {
		if ic.Fallback != nil {
			return ic.Fallback
		}
		return SizedBox{Width: size, Height: size}
	}
	return SizedBox{
		Width:  size,
		Height: size,
		Child:  Image{Source: img},
	}
}
