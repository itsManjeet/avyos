package main

import (
	"fmt"
	"math"

	"avyos.dev/pkg/graphics/color"
	"avyos.dev/pkg/graphics/font/ttf"
	"avyos.dev/pkg/graphics/geom"
	"avyos.dev/pkg/graphics/layout"
	"avyos.dev/pkg/graphics/paint"
	"avyos.dev/pkg/graphics/theme"
	"avyos.dev/pkg/graphics/widget"
)

type notepadEditor struct {
	Content string
	Cursor  int
	Focused bool
}

func (e notepadEditor) Build(ctx widget.BuildContext) widget.Widget {
	return notepadEditorLeaf{
		content:     e.Content,
		cursor:      e.Cursor,
		focused:     e.Focused,
		style:       editorTextStyle(ctx),
		gutterStyle: editorGutterStyle(ctx),
		paddingX:    editorPaddingX(ctx),
		paddingY:    editorPaddingY(ctx),
		gutterWidth: editorGutterWidth(ctx),
		cursorColor: ctx.Theme.ColorScheme.Primary,
	}
}

type notepadEditorLeaf struct {
	content     string
	cursor      int
	focused     bool
	style       theme.TextStyle
	gutterStyle theme.TextStyle
	paddingX    float64
	paddingY    float64
	gutterWidth float64
	cursorColor color.Color
}

func (e notepadEditorLeaf) Layout(c layout.BoxConstraints) geom.Size {
	lineHeight := editorLineHeight(e.style)
	height := e.paddingY*2 + float64(maxInt(1, len(noteLines(e.content))))*lineHeight
	width := c.MaxWidth
	if width >= layout.Inf {
		width = 720
	}
	minWidth := e.gutterWidth + e.paddingX*2 + 200
	width = math.Max(width, minWidth)
	return c.Constrain(geom.Sz(width, height))
}

func (e notepadEditorLeaf) Paint(ctx *paint.Context, offset geom.Point, size geom.Size) {
	if e.style.Face == nil || e.gutterStyle.Face == nil {
		return
	}

	lineHeight := editorLineHeight(e.style)
	lines := noteLines(e.content)
	textX := offset.X + e.paddingX + e.gutterWidth
	gutterRight := textX - e.paddingX/2

	ctx.Save()
	ctx.ClipRect(geom.NewRect(offset.X, offset.Y, size.Width, size.Height))

	for i, line := range lines {
		y := offset.Y + e.paddingY + float64(i)*lineHeight
		ctx.DrawText(fmt.Sprintf("%d", i+1), geom.Pt(offset.X+e.paddingX, y), e.gutterStyle.Face, e.gutterStyle.Size, e.gutterStyle.Color)
		if line.Text != "" {
			ctx.DrawText(line.Text, geom.Pt(textX, y), e.style.Face, e.style.Size, e.style.Color)
		}
		ctx.FillRect(geom.NewRect(gutterRight, y+2, 1, lineHeight-4), e.gutterStyle.Color.WithAlpha(0.2))
	}

	if e.focused {
		line, col := cursorLineColumn(e.content, e.cursor)
		if line < 0 {
			line = 0
		}
		if line >= len(lines) {
			line = len(lines) - 1
		}
		cursorX := textX + editorTextWidthPrefix(lines[line].Text, col, e.style)
		cursorY := offset.Y + e.paddingY + float64(line)*lineHeight
		ctx.FillRect(geom.NewRect(cursorX, cursorY+2, 2, lineHeight-4), e.cursorColor)
	}

	ctx.Restore()
}

func (e notepadEditorLeaf) HitTest(pos, offset geom.Point, size geom.Size) bool {
	return geom.NewRect(offset.X, offset.Y, size.Width, size.Height).Contains(pos)
}

func editorTextStyle(ctx widget.BuildContext) theme.TextStyle {
	style := ctx.Theme.TextTheme.BodyMedium
	style.Face = ttf.DefaultMono()
	style.Color = ctx.Theme.ColorScheme.OnSurface
	return style
}

func editorGutterStyle(ctx widget.BuildContext) theme.TextStyle {
	style := ctx.Theme.TextTheme.LabelSmall
	style.Face = ttf.DefaultMono()
	style.Color = ctx.Theme.ColorScheme.OnSurfaceVariant
	return style
}

func editorLineHeight(style theme.TextStyle) float64 {
	if style.LineHeight > 0 {
		return style.LineHeight
	}
	return style.Face.LineHeight(style.Size) + 2
}

func editorPaddingX(ctx widget.BuildContext) float64 {
	return ctx.Theme.Space.Unit(3)
}

func editorPaddingY(ctx widget.BuildContext) float64 {
	return ctx.Theme.Space.Unit(3)
}

func editorGutterWidth(ctx widget.BuildContext) float64 {
	return ctx.Theme.Space.Unit(8)
}

func editorTextX(ctx widget.BuildContext) float64 {
	return editorPaddingX(ctx) + editorGutterWidth(ctx)
}

func editorTextWidthPrefix(text string, col int, style theme.TextStyle) float64 {
	width := 0.0
	runes := []rune(text)
	if col > len(runes) {
		col = len(runes)
	}
	for i := 0; i < col; i++ {
		width += style.Face.RuneAdvance(runes[i], style.Size)
	}
	return width
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
