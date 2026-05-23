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

// FAB is a floating action button.
//
// Set Label to render an extended FAB (icon + label). Leave it empty for the
// circular icon-only variant. The FAB is positioned by [Application]; use it
// standalone only when you manage layout yourself.
//
// FAB is a [widget.Buildable].
type FAB struct {
	// Icon name for widget.Icon. Falls back to "+" text if empty.
	Icon string
	// Label — if non-empty, renders an extended pill-shaped FAB.
	Label string
	// OnPressed is called on tap.
	OnPressed func()
	// Tone controls colour: uses widget.ButtonPrimary by default.
	Tone widget.ButtonTone
}

func (f FAB) Build(ctx widget.BuildContext) widget.Widget {
	th := ctx.Theme
	tone := f.Tone

	var iconW widget.Widget
	if f.Icon != "" {
		iconW = widget.Icon{Name: f.Icon, Size: 24}
	} else {
		st := th.TextTheme.TitleMedium
		st.Color = th.ColorScheme.OnPrimary
		iconW = widget.Text{Content: "+", Style: &st}
	}

	var child widget.Widget
	if f.Label != "" {
		labelSt := th.TextTheme.LabelLarge
		labelSt.Color = th.ColorScheme.OnPrimary
		child = widget.Row{
			CrossAxisAlignment: layout.CrossCenter,
			Children: []widget.Widget{
				iconW,
				widget.SizedBox{Width: th.Space.Unit(2)},
				widget.Text{Content: f.Label, Style: &labelSt},
			},
		}
	} else {
		child = iconW
	}

	return widget.Button{
		Child:     child,
		Variant:   widget.ButtonSolid,
		Tone:      tone,
		OnPressed: f.OnPressed,
	}
}
