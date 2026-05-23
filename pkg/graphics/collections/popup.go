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
	"avyos.dev/pkg/graphics/color"
	"avyos.dev/pkg/graphics/geom"
	"avyos.dev/pkg/graphics/layout"
	"avyos.dev/pkg/graphics/widget"
)

// ─── MenuItem ─────────────────────────────────────────────────────────────────

// MenuItem is a single item in a [PopupMenu].
// Set Divider to true to render a horizontal rule instead of a label row.
type MenuItem struct {
	Label    string
	Icon     string
	OnTap    func()
	Divider  bool
	Disabled bool
}

// ─── PopupMenuController ──────────────────────────────────────────────────────

// PopupMenuController opens and closes a [PopupMenu] via the shared [OverlayManager].
// Create one with [NewPopupMenuController].
type PopupMenuController struct {
	overlay *OverlayManager
}

// NewPopupMenuController creates a PopupMenuController backed by oc.
func NewPopupMenuController(oc *OverlayManager) *PopupMenuController {
	return &PopupMenuController{overlay: oc}
}

// Show renders a popup menu anchored near anchorRect (in screen coordinates).
// Items are shown below the anchor when space permits.
// The returned function closes the menu; tapping outside also closes it.
func (pc *PopupMenuController) Show(items []MenuItem, anchorRect geom.Rect) (close func()) {
	e := &OverlayEntry{}
	e.Builder = func(_ widget.BuildContext) widget.Widget {
		return buildPopupWidget(items, anchorRect, e.Remove)
	}
	pc.overlay.Insert(e)
	return e.Remove
}

// buildPopupWidget produces the positioned popup container.
func buildPopupWidget(items []MenuItem, anchor geom.Rect, dismiss func()) widget.Widget {
	return popupBuildable{items: items, anchor: anchor, dismiss: dismiss}
}

type popupBuildable struct {
	items   []MenuItem
	anchor  geom.Rect
	dismiss func()
}

func (pb popupBuildable) Build(ctx widget.BuildContext) widget.Widget {
	th := ctx.Theme

	// ── item rows
	rows := make([]widget.Widget, 0, len(pb.items))
	for _, item := range pb.items {
		item := item
		if item.Divider {
			rows = append(rows, widget.Separator{Color: th.ColorScheme.Outline})
			continue
		}
		rows = append(rows, menuItemWidget(item, pb.dismiss, ctx))
	}

	menu := widget.Container{
		Fill:          th.ColorScheme.Surface,
		Border:        th.ColorScheme.Outline,
		BorderWidth:   1,
		Radius:        th.Shape.LargeRadius,
		Shadow:        th.ColorScheme.Shadow,
		ShadowBlur:    th.Shadow.MD.Blur,
		ShadowOffsetY: th.Shadow.MD.OffsetY,
		Padding:       layout.Symmetric(0, th.Space.Unit(1)),
		Child: widget.Column{
			CrossAxisAlignment: layout.CrossStretch,
			Children:           rows,
		},
	}

	// Place menu below anchor; fall back to above if it would go offscreen.
	top := pb.anchor.Max.Y + 4
	left := pb.anchor.Min.X

	// Full-screen tap to dismiss (non-modal: no scrim, but catches outside taps).
	backdrop := widget.GestureDetector{
		OnTap: pb.dismiss,
		Child: widget.Positioned{
			Top:    widget.Ptr(0),
			Right:  widget.Ptr(0),
			Bottom: widget.Ptr(0),
			Left:   widget.Ptr(0),
			Child:  widget.Container{Fill: color.Transparent},
		},
	}

	return widget.Stack{
		Children: []widget.Widget{
			backdrop,
			widget.Positioned{
				Top:  widget.Ptr(top),
				Left: widget.Ptr(left),
				Child: widget.SizedBox{
					Width: 220,
					Child: menu,
				},
			},
		},
	}
}

func menuItemWidget(item MenuItem, dismiss func(), ctx widget.BuildContext) widget.Widget {
	th := ctx.Theme

	labelSt := th.TextTheme.BodyMedium
	if item.Disabled {
		labelSt.Color = th.ColorScheme.OnSurfaceVariant
	} else {
		labelSt.Color = th.ColorScheme.OnSurface
	}

	rowChildren := make([]widget.Widget, 0, 4)
	if item.Icon != "" {
		rowChildren = append(rowChildren,
			widget.Icon{Name: item.Icon, Size: 16},
			widget.SizedBox{Width: th.Space.Unit(3)},
		)
	}
	rowChildren = append(rowChildren, widget.Text{Content: item.Label, Style: &labelSt})

	inner := widget.Row{
		CrossAxisAlignment: layout.CrossCenter,
		Children:           rowChildren,
	}

	if item.Disabled {
		return widget.Container{
			Padding: layout.Symmetric(th.Space.Unit(4), th.Space.Unit(2)),
			Child:   inner,
		}
	}

	onTap := func() {
		if dismiss != nil {
			dismiss()
		}
		if item.OnTap != nil {
			item.OnTap()
		}
	}

	return widget.GestureDetector{
		OnTap: onTap,
		Builder: func(state widget.InteractionState) widget.Widget {
			bg := color.Transparent
			if state.Hovered {
				bg = th.ColorScheme.SurfaceContainer
			}
			return widget.Container{
				Fill:    bg,
				Radius:  th.Shape.SmallRadius,
				Padding: layout.Symmetric(th.Space.Unit(4), th.Space.Unit(2)),
				Child:   inner,
			}
		},
	}
}
