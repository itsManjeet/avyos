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

// TextInput is a tappable, editable single-line text field.
// Keyboard events are routed via [Frame.HandleKey] while this input is focused.
//
//	var name string
//	widget.TextInput{Value: &name, Hint: "Full name"}
//
//	// Password field:
//	widget.TextInput{Value: &pass, Hint: "Password", Obscure: true}
package widget

import (
	"strings"

	"avyos.dev/pkg/graphics/color"
	"avyos.dev/pkg/graphics/event"
	"avyos.dev/pkg/graphics/geom"
	"avyos.dev/pkg/graphics/layout"
	"avyos.dev/pkg/graphics/paint"
	"avyos.dev/pkg/graphics/theme"
)

// TextInputVariant selects the visual treatment.
type TextInputVariant uint8

const (
	TextInputOutline TextInputVariant = iota // border on all sides (default)
	TextInputFilled                          // filled background with border
	TextInputFlushed                         // underline only
)

// TextInput is a single-line text editing field.
// Value points to the string being edited; the framework routes keyboard
// events here while this input is focused.
// Obscure replaces each character with "*" (for passwords).
type TextInput struct {
	Value   *string
	Label   string // optional label drawn above the field
	Hint    string // placeholder shown when Value is empty
	Obscure bool
	Style   *theme.TextStyle // nil = theme BodyMedium
	Variant TextInputVariant
}

func (t TextInput) Build(ctx BuildContext) Widget {
	style := t.Style
	if style == nil {
		s := ctx.Theme.TextTheme.BodyMedium
		style = &s
	}

	focused := ctx.frame != nil && ctx.frame.focusedInput == t.Value
	value := t.Value
	frame := ctx.frame

	target := 0.0
	if focused {
		target = 1
	}

	labelStyle := ctx.Theme.TextTheme.LabelSmall
	labelStyle.Color = ctx.Theme.ColorScheme.OnSurfaceVariant

	leaf := textInputLeaf{
		value:      t.Value,
		label:      t.Label,
		hint:       t.Hint,
		obscure:    t.Obscure,
		style:      *style,
		labelStyle: labelStyle,
		variant:    t.Variant,
		radius:     ctx.Theme.Shape.MediumRadius,
		paddingX:   ctx.Theme.Space.Unit(4),
		paddingY:   ctx.Theme.Space.Unit(3),
		border:     ctx.Theme.ColorScheme.Outline,
		borderSoft: ctx.Theme.ColorScheme.OutlineVariant,
		surface:    ctx.Theme.ColorScheme.Surface,
		surfaceAlt: ctx.Theme.ColorScheme.SurfaceContainer,
		hintColor:  ctx.Theme.ColorScheme.OnSurfaceVariant,
		textColor:  ctx.Theme.ColorScheme.OnSurface,
		focusColor: ctx.Theme.ColorScheme.FocusRing,
		active:     target,
	}

	return GestureDetector{
		Cursor: event.CursorText,
		OnTap: func() {
			if frame != nil {
				frame.FocusInputPath(value, ctx.path)
			}
		},
		Builder: func(state InteractionState) Widget {
			visualTarget := target
			if visualTarget < 1 && state.Hovered {
				visualTarget = 0.35
			}
			if state.Pressed {
				visualTarget = maxf(visualTarget, 0.5)
			}
			return Animated{
				Value:    visualTarget,
				Duration: ctx.Theme.Motion.Moderate,
				Curve:    EaseInOut,
				Builder: func(v float64) Widget {
					leaf.active = v
					return leaf
				},
			}
		},
	}
}

type textInputLeaf struct {
	value      *string
	label      string
	hint       string
	obscure    bool
	style      theme.TextStyle
	labelStyle theme.TextStyle
	variant    TextInputVariant
	radius     float64
	paddingX   float64
	paddingY   float64
	border     color.Color
	borderSoft color.Color
	surface    color.Color
	surfaceAlt color.Color
	hintColor  color.Color
	textColor  color.Color
	focusColor color.Color
	active     float64
}

func (ti textInputLeaf) Layout(c layout.BoxConstraints) geom.Size {
	fieldH := textStyleLineHeight(ti.style) + ti.paddingY*2
	totalH := fieldH
	if ti.label != "" {
		totalH += textStyleLineHeight(ti.labelStyle) + 6
	}
	w := c.MaxWidth
	if w >= layout.Inf {
		w = 220
	}
	return c.Constrain(geom.Sz(w, totalH))
}

func (ti textInputLeaf) Paint(ctx *paint.Context, offset geom.Point, size geom.Size) {
	if ti.style.Face == nil {
		return
	}

	y := offset.Y
	if ti.label != "" && ti.labelStyle.Face != nil {
		drawTextStyle(ctx, ti.label, geom.Pt(offset.X, y), ti.labelStyle)
		y += textStyleLineHeight(ti.labelStyle) + 6
	}

	fieldH := size.Height - (y - offset.Y)
	fieldRect := geom.NewRect(offset.X, y, size.Width, fieldH)
	border := mixColor(ti.border, ti.focusColor, ti.active)

	switch ti.variant {
	case TextInputFilled:
		surface := mixColor(ti.surfaceAlt, ti.surface, 0.18*ti.active)
		ctx.FillRoundedRect(fieldRect, ti.radius, surface)
		ctx.StrokeRoundedRect(fieldRect.Inset(0.5, 0.5), insetCornerRadius(ti.radius, 0.5), 1, border)
	case TextInputFlushed:
		ctx.FillRect(geom.NewRect(fieldRect.Min.X, fieldRect.Max.Y-1, fieldRect.Width(), 1), ti.borderSoft)
		ctx.FillRoundedRect(geom.NewRect(fieldRect.Min.X, fieldRect.Max.Y-2, fieldRect.Width(), 2), 1, border)
	default: // TextInputOutline
		ctx.FillRoundedRect(fieldRect, ti.radius, ti.surface)
		ctx.StrokeRoundedRect(fieldRect.Inset(0.5, 0.5), insetCornerRadius(ti.radius, 0.5), 1, border)
	}

	if ti.active > 0 {
		drawGlow(ctx, fieldRect, ti.radius, ti.focusColor.WithAlpha(0.08+0.08*ti.active), 2+2*ti.active)
	}

	val := ""
	if ti.value != nil {
		val = *ti.value
	}
	if ti.obscure {
		val = strings.Repeat("*", len([]rune(val)))
	}

	textStyle := ti.style
	textStyle.Color = ti.textColor
	display := val
	if display == "" {
		display = ti.hint
		textStyle.Color = ti.hintColor
	}

	textPos := geom.Pt(fieldRect.Min.X+ti.paddingX, fieldRect.Min.Y+ti.paddingY)
	text := fitTextToWidth(display, textStyle, fieldRect.Width()-ti.paddingX*2)
	drawTextStyle(ctx, text, textPos, textStyle)
}

func (ti textInputLeaf) HitTest(_, _ geom.Point, _ geom.Size) bool { return true }
