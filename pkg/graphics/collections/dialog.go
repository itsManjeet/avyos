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

// ─── Dialog ───────────────────────────────────────────────────────────────────

// Dialog describes a modal dialog to be shown via [DialogController.Show].
//
// Width defaults to 480. Title is optional. Actions are right-aligned buttons
// rendered in a row below the body.
type Dialog struct {
	Title   string
	Body    widget.Widget
	Actions []widget.Widget
	// Width of the dialog panel. 0 defaults to 480.
	Width float64
}

// ─── DialogController ─────────────────────────────────────────────────────────

// DialogController shows and hides modal dialogs using the shared [OverlayManager].
// Create one with [NewDialogController] and pass it to components that trigger dialogs.
type DialogController struct {
	overlay *OverlayManager
}

// NewDialogController creates a DialogController backed by oc.
func NewDialogController(oc *OverlayManager) *DialogController {
	return &DialogController{overlay: oc}
}

// Show pushes d onto the overlay stack. The returned function closes the dialog.
// Tapping the scrim also closes it.
func (dc *DialogController) Show(d Dialog) (close func()) {
	e := &OverlayEntry{}
	e.Builder = func(_ widget.BuildContext) widget.Widget {
		return buildDialogWidget(d, e.Remove)
	}
	e.Modal = true
	dc.overlay.Insert(e)
	return e.Remove
}

// buildDialogWidget builds the centered dialog panel as a Positioned overlay.
func buildDialogWidget(d Dialog, dismiss func()) widget.Widget {
	return dialogBuildable{d: d, dismiss: dismiss}
}

type dialogBuildable struct {
	d       Dialog
	dismiss func()
}

func (db dialogBuildable) Build(ctx widget.BuildContext) widget.Widget {
	th := ctx.Theme

	w := db.d.Width
	if w <= 0 {
		w = 480
	}
	// Clamp to screen width with side margins.
	if ctx.ScreenSize.Width > 0 && w > ctx.ScreenSize.Width-48 {
		w = ctx.ScreenSize.Width - 48
	}

	// ── title
	children := make([]widget.Widget, 0, 5)
	if db.d.Title != "" {
		titleSt := th.TextTheme.TitleLarge
		titleSt.Color = th.ColorScheme.OnSurface
		children = append(children,
			widget.Text{Content: db.d.Title, Style: &titleSt},
			widget.SizedBox{Height: th.Space.Unit(4)},
		)
	}

	// ── body
	if db.d.Body != nil {
		children = append(children, db.d.Body)
	}

	// ── actions
	if len(db.d.Actions) > 0 {
		actRow := make([]widget.Widget, 0, len(db.d.Actions)*2)
		for i, a := range db.d.Actions {
			if i > 0 {
				actRow = append(actRow, widget.SizedBox{Width: th.Space.Unit(2)})
			}
			actRow = append(actRow, a)
		}
		children = append(children,
			widget.SizedBox{Height: th.Space.Unit(4)},
			widget.Row{
				MainAxisAlignment:  layout.MainEnd,
				CrossAxisAlignment: layout.CrossCenter,
				Children:           actRow,
			},
		)
	}

	panel := widget.SizedBox{
		Width: w,
		Child: widget.Container{
			Fill:          th.ColorScheme.Surface,
			Radius:        th.Shape.XLargeRadius,
			Shadow:        th.ColorScheme.Shadow,
			ShadowBlur:    th.Shadow.XL.Blur,
			ShadowOffsetY: th.Shadow.XL.OffsetY,
			Padding:       layout.All(th.Space.Unit(6)),
			Child: widget.Column{
				MainAxisSize:       layout.MainMin,
				CrossAxisAlignment: layout.CrossStretch,
				Children:           children,
			},
		},
	}

	return widget.Positioned{
		Top:    widget.Ptr(0),
		Right:  widget.Ptr(0),
		Bottom: widget.Ptr(0),
		Left:   widget.Ptr(0),
		Child:  widget.Center(panel),
	}
}
