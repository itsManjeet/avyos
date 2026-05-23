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

// Button is the single action control. It is configured by three orthogonal
// dimensions: visual Variant, semantic Tone, and sizing Size.
//
//	// Primary solid button:
//	widget.Button{Child: widget.Text{Content: "Save"}, OnPressed: save}
//
//	// Danger ghost button:
//	widget.Button{
//	    Child: widget.Text{Content: "Delete"},
//	    Tone:  widget.ButtonToneDanger,
//	    Variant: widget.ButtonVariantGhost,
//	    OnPressed: delete,
//	}
package widget

import (
	"avyos.dev/lib/graphics/color"
	"avyos.dev/lib/graphics/geom"
	"avyos.dev/lib/graphics/layout"
	"avyos.dev/lib/graphics/paint"
	"avyos.dev/lib/graphics/theme"
)

// ButtonVariant selects the visual recipe.
type ButtonVariant uint8

const (
	ButtonSolid   ButtonVariant = iota // filled background
	ButtonOutline                      // light filled background
	ButtonGhost                        // subtle filled background
)

// ButtonTone selects the semantic color palette.
type ButtonTone uint8

const (
	ButtonPrimary ButtonTone = iota // brand color
	ButtonDanger                    // destructive action
	ButtonNeutral                   // no semantic meaning
)

// ButtonSize controls the padding and minimum height.
type ButtonSize uint8

const (
	ButtonSmall  ButtonSize = iota
	ButtonMedium            // default
	ButtonLarge
)

// Button is a tappable action control.
// Set Child to any widget (typically [Text] or a [Row] with an [Icon] and text).
type Button struct {
	Child     Widget
	OnPressed func()
	Variant   ButtonVariant
	Tone      ButtonTone
	Size      ButtonSize
}

func (b Button) Build(ctx BuildContext) Widget {
	palette := buttonPaletteFor(ctx.Theme, b.Tone)
	style, padding, minHeight, radius := buttonMetrics(ctx.Theme, b.Size)

	return GestureDetector{
		OnTap: b.OnPressed,
		Builder: func(state InteractionState) Widget {
			return Animated{
				Value:    interactionTarget(state),
				Duration: ctx.Theme.Motion.Fast,
				Curve:    EaseOut,
				Builder: func(v float64) Widget {
					fill, border, label, shadow, glow := buttonVisuals(ctx.Theme, palette, b.Variant, v)
					if state.Pressed {
						fill = mixColor(fill, palette.Emphasized, 0.32)
					}
					style.Color = label
					return buttonLeaf{
						child:      buttonTextChild(b.Child, style),
						padding:    padding,
						minHeight:  minHeight,
						radius:     radius,
						fill:       fill,
						border:     border,
						shadow:     shadow,
						glow:       glow,
						shadowSpec: buttonShadowSpec(ctx.Theme, b.Variant),
					}
				},
			}
		},
	}
}

type buttonLeaf struct {
	child      Widget
	padding    layout.EdgeInsets
	minHeight  float64
	radius     float64
	fill       color.Color
	border     color.Color
	shadow     color.Color
	glow       color.Color
	shadowSpec theme.ShadowSpec
}

type buttonPalette struct {
	Solid      color.Color
	Contrast   color.Color
	Fg         color.Color
	Subtle     color.Color
	Emphasized color.Color
	FocusRing  color.Color
	Border     color.Color
}

func (bl buttonLeaf) RenderChildren(c layout.BoxConstraints, pctx *paint.Context, offset geom.Point, cr ChildRenderer) geom.Size {
	innerC := bl.padding.Deflate(c)
	var childSz geom.Size
	if bl.child != nil {
		childSz = cr.Measure(bl.child, innerC.Loosen(), "child")
	}

	sz := c.Constrain(geom.Sz(
		childSz.Width+bl.padding.Horizontal(),
		maxf(childSz.Height+bl.padding.Vertical(), bl.minHeight),
	))

	rect := geom.NewRect(offset.X, offset.Y, sz.Width, sz.Height)
	if pctx != nil {
		if bl.shadow.A > 0 && bl.shadowSpec.Blur > 0 {
			drawButtonShadow(pctx, rect, bl.radius, bl.shadow, bl.shadowSpec)
		}
		if bl.glow.A > 0 && bl.shadowSpec.GlowSpread > 0 {
			drawGlow(pctx, rect, bl.radius, bl.glow, bl.shadowSpec.GlowSpread)
		}
		if bl.fill.A > 0 {
			pctx.FillRoundedRect(rect, bl.radius, bl.fill)
		}
		if bl.border.A > 0 {
			pctx.StrokeRoundedRect(rect.Inset(0.5, 0.5), insetCornerRadius(bl.radius, 0.5), 1, bl.border)
		}
	}

	if bl.child != nil {
		childH := maxf(childSz.Height, sz.Height-bl.padding.Vertical())
		childOffset := geom.Pt(
			offset.X+bl.padding.Left,
			offset.Y+(sz.Height-childH)/2,
		)
		cr.Render(bl.child, layout.Tight(sz.Width-bl.padding.Horizontal(), childH), childOffset, "child")
	}
	return sz
}

func drawButtonShadow(pctx *paint.Context, rect geom.Rect, radius float64, shadow color.Color, spec theme.ShadowSpec) {
	drawSoftShadow(pctx, rect, radius, shadow.WithAlpha(minf(1, shadow.A/shadowAlphaScale)), spec.Blur, spec.OffsetY)
}

// --- palette / metrics / visuals ---

func buttonPaletteFor(th *theme.ThemeData, tone ButtonTone) buttonPalette {
	switch tone {
	case ButtonDanger:
		return buttonPalette{
			Solid:      th.ColorScheme.Error,
			Contrast:   th.ColorScheme.OnError,
			Fg:         th.ColorScheme.Error,
			Subtle:     th.ColorScheme.ErrorContainer,
			Emphasized: th.ColorScheme.Error,
			FocusRing:  th.ColorScheme.Error,
			Border:     th.ColorScheme.Error,
		}
	case ButtonNeutral:
		return buttonPalette{
			Solid:      th.ColorScheme.OnSurface,
			Contrast:   th.ColorScheme.OnInverseSurface,
			Fg:         th.ColorScheme.OnSurface,
			Subtle:     th.ColorScheme.SurfaceVariant,
			Emphasized: th.ColorScheme.SurfaceContainerHighest,
			FocusRing:  th.ColorScheme.FocusRing,
			Border:     th.ColorScheme.Outline,
		}
	default:
		return buttonPalette{
			Solid:      th.ColorScheme.Primary,
			Contrast:   th.ColorScheme.OnPrimary,
			Fg:         th.ColorScheme.Primary,
			Subtle:     th.ColorScheme.PrimaryContainer,
			Emphasized: th.ColorScheme.Primary,
			FocusRing:  th.ColorScheme.FocusRing,
			Border:     th.ColorScheme.Primary,
		}
	}
}

// buttonMetrics returns (textStyle, padding, minHeight, borderRadius).
//
//	sm: h=32px (unit 8), px=12px (token 3)
//	md: h=40px (unit 10), px=16px (token 4)  ← default
//	lg: h=48px (unit 12), px=24px (token 6)
func buttonMetrics(th *theme.ThemeData, size ButtonSize) (theme.TextStyle, layout.EdgeInsets, float64, float64) {
	r := th.Shape.FullRadius
	switch size {
	case ButtonSmall:
		return th.TextTheme.LabelSmall, th.Space.Symmetric(3, 2), th.Space.Unit(8), r
	case ButtonLarge:
		return th.TextTheme.LabelLarge, th.Space.Symmetric(6, 3), th.Space.Unit(12), r
	default:
		return th.TextTheme.LabelMedium, th.Space.Symmetric(4, 2.5), th.Space.Unit(10), r
	}
}

func buttonVisuals(th *theme.ThemeData, p buttonPalette, v ButtonVariant, interaction float64) (fill, border, label, shadow, glow color.Color) {
	switch v {
	case ButtonOutline:
		fill = mixColor(th.ColorScheme.Surface, p.Subtle, 0.10+0.12*interaction)
		border = color.Transparent
		label = p.Fg
		shadow = color.FromHex(0x3E2D1C).WithAlpha(0.08 + 0.04*interaction)
		glow = color.Transparent
	case ButtonGhost:
		fill = mixColor(th.ColorScheme.Surface, p.Subtle, 0.08+0.14*interaction)
		border = color.Transparent
		label = p.Fg
		shadow = color.FromHex(0x3E2D1C).WithAlpha(0.08 + 0.03*interaction)
		glow = color.Transparent
	default: // ButtonSolid
		fill = mixColor(p.Solid, p.Emphasized, 0.18*interaction)
		border = color.Transparent
		label = p.Contrast
		shadow = p.Solid.WithAlpha(0.22 + 0.04*interaction)
		if p.Solid == th.ColorScheme.OnSurface {
			shadow = color.FromHex(0x3E2D1C).WithAlpha(0.30 + 0.04*interaction)
		}
		glow = color.Transparent
	}
	return
}

func buttonShadowSpec(_ *theme.ThemeData, v ButtonVariant) theme.ShadowSpec {
	switch v {
	case ButtonSolid:
		return theme.ShadowSpec{Blur: 5, OffsetY: 3}
	case ButtonOutline:
		return theme.ShadowSpec{Blur: 5, OffsetY: 2}
	default:
		spec := theme.ShadowSpec{Blur: 5, OffsetY: 2}
		spec.GlowSpread = 2
		return spec
	}
}

// buttonTextChild inherits the button's label style when Child is a plain Text.
func buttonTextChild(child Widget, style theme.TextStyle) Widget {
	switch t := child.(type) {
	case Text:
		if t.Style == nil {
			s := style
			t.Style = &s
		}
		return t
	case nil:
		s := style
		return Text{Content: "", Style: &s}
	default:
		return child
	}
}
