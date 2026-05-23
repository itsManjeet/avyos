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

package collections

import (
	"avyos.dev/pkg/graphics/layout"
	"avyos.dev/pkg/graphics/widget"
)

// AppBar is the top application bar.
//
// It renders a horizontally-divided row: Leading | Title (expanded) | Actions.
// All fields are optional; omit Leading for a title-only bar, omit Title for
// a purely icon-driven bar, etc.
//
// AppBar is a [widget.Buildable]; use it standalone or embed it in an [Application].
type AppBar struct {
	// Title text. Ignored if TitleWidget is set.
	Title string
	// TitleWidget replaces the text title with an arbitrary widget.
	TitleWidget widget.Widget
	// Leading is the leftmost widget — typically a hamburger/back button.
	Leading widget.Widget
	// Actions are right-aligned icon buttons.
	Actions []widget.Widget
	// Bottom is rendered below the title row — useful for a search bar or tabs.
	Bottom widget.Widget
	// Compact reduces the bar height (useful on mobile).
	Compact bool
}

func (a AppBar) Build(ctx widget.BuildContext) widget.Widget {
	th := ctx.Theme

	vpad := th.Space.Unit(3)
	if a.Compact {
		vpad = th.Space.Unit(2)
	}

	// ── title widget
	var titleW widget.Widget
	if a.TitleWidget != nil {
		titleW = a.TitleWidget
	} else if a.Title != "" {
		st := th.TextTheme.TitleMedium
		st.Color = th.ColorScheme.OnSurface
		titleW = widget.Text{Content: a.Title, Style: &st}
	} else {
		titleW = widget.SizedBox{}
	}

	// ── top row: [leading] [title⋯] [actions…]
	rowChildren := make([]widget.Widget, 0, 4)
	if a.Leading != nil {
		rowChildren = append(rowChildren,
			a.Leading,
			widget.SizedBox{Width: th.Space.Unit(2)},
		)
	}
	rowChildren = append(rowChildren, widget.Expanded{Child: titleW})
	for _, act := range a.Actions {
		rowChildren = append(rowChildren, widget.SizedBox{Width: th.Space.Unit(1)}, act)
	}

	topRow := widget.Row{
		CrossAxisAlignment: layout.CrossCenter,
		Children:           rowChildren,
	}

	// ── full bar column
	barChildren := []widget.Widget{topRow}
	if a.Bottom != nil {
		barChildren = append(barChildren,
			widget.SizedBox{Height: th.Space.Unit(2)},
			a.Bottom,
		)
	}

	return widget.Container{
		Fill:        th.ColorScheme.Surface,
		Border:      th.ColorScheme.Outline,
		BorderWidth: 1,
		Padding:     layout.Symmetric(th.Space.Unit(4), vpad),
		Child: widget.Column{
			CrossAxisAlignment: layout.CrossStretch,
			Children:           barChildren,
		},
	}
}
