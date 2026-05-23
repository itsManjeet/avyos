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

// ─── Card ─────────────────────────────────────────────────────────────────────

// Card is an elevated surface that groups related content.
// It renders as a rounded, shadowed container — nothing more.
//
// Two visual variants:
//   - Raised (default false): outline + subtle shadow
//   - Raised true: drop shadow with no outline
//
// Card does not impose padding; wrap Child in a Padding widget or set the
// Padding field if you want inner spacing.
type Card struct {
	Child   widget.Widget
	Padding layout.EdgeInsets // zero = no forced padding (use theme default via Build)
	Raised  bool
}

func (c Card) Build(ctx widget.BuildContext) widget.Widget {
	th := ctx.Theme

	pad := c.Padding
	if pad == (layout.EdgeInsets{}) {
		pad = layout.All(th.Space.Unit(4))
	}

	ct := widget.Container{
		Fill:    th.ColorScheme.Surface,
		Radius:  th.Shape.LargeRadius,
		Padding: pad,
		Child:   c.Child,
	}
	if c.Raised {
		ct.Shadow = th.ColorScheme.Shadow
		ct.ShadowBlur = th.Shadow.MD.Blur
		ct.ShadowOffsetY = th.Shadow.MD.OffsetY
	} else {
		ct.Border = th.ColorScheme.Outline
		ct.BorderWidth = 1
	}
	return ct
}

// ─── Section ──────────────────────────────────────────────────────────────────

// Section renders a labeled group of content with an optional trailing action.
// It is the standard pattern for form sections, settings groups, and content
// panels where a heading distinguishes the block from its neighbours.
type Section struct {
	Title  string
	Action widget.Widget // optional trailing widget (e.g. a Button)
	Child  widget.Widget
}

func (s Section) Build(ctx widget.BuildContext) widget.Widget {
	th := ctx.Theme

	titleSt := th.TextTheme.LabelLarge
	titleSt.Color = th.ColorScheme.OnSurfaceVariant

	var header widget.Widget
	if s.Action != nil {
		header = widget.Row{
			CrossAxisAlignment: layout.CrossCenter,
			Children: []widget.Widget{
				widget.Text{Content: s.Title, Style: &titleSt},
				widget.Spacer{},
				s.Action,
			},
		}
	} else {
		header = widget.Text{Content: s.Title, Style: &titleSt}
	}

	children := []widget.Widget{
		header,
		widget.SizedBox{Height: th.Space.Unit(3)},
	}
	if s.Child != nil {
		children = append(children, s.Child)
	}
	return widget.Column{
		CrossAxisAlignment: layout.CrossStretch,
		Children:           children,
	}
}
