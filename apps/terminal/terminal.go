package main

import (
	"fmt"
	"strings"
	"time"

	"avyos.dev/pkg/graphics/app"
	"avyos.dev/pkg/graphics/collections"
	"avyos.dev/pkg/graphics/color"
	"avyos.dev/pkg/graphics/event"
	"avyos.dev/pkg/graphics/font/ttf"
	"avyos.dev/pkg/graphics/geom"
	"avyos.dev/pkg/graphics/layout"
	"avyos.dev/pkg/graphics/paint"
	"avyos.dev/pkg/graphics/theme"
	"avyos.dev/pkg/graphics/widget"
	"avyos.dev/pkg/pty"
)

type TerminalApp struct{}

func (TerminalApp) CreateState() widget.State { return &TerminalState{} }

type TerminalState struct {
	widget.StateBase

	appCtrl *collections.ApplicationController
	term    *pty.Terminal
	version uint64
	rows    int
	cols    int
	errText string
}

func (s *TerminalState) InitState() {
	s.appCtrl = collections.NewApplicationController()
	term, err := pty.NewTerminal(28, 100)
	if err != nil {
		s.errText = err.Error()
		return
	}
	s.term = term
	s.version = term.Version()
	s.rows = 28
	s.cols = 100

	app.EventHandler = func(e event.Event) {
		switch ev := e.(type) {
		case event.TextInputEvent:
			if s.term != nil {
				_ = s.term.WriteString(string(ev.Rune))
			}
			return
		case event.KeyEvent:
			if s.term != nil && ev.Down {
				if data, ok := encodeTerminalKey(ev); ok {
					_ = s.term.Write(data)
					return
				}
			}
		}
		app.DefaultHandler(e)
	}

	go s.watchTerminal()
}

func (s *TerminalState) watchTerminal() {
	ticker := time.NewTicker(33 * time.Millisecond)
	defer ticker.Stop()
	for range ticker.C {
		if s.term == nil {
			return
		}
		v := s.term.Version()
		if v == s.version {
			continue
		}
		s.SetState(func() { s.version = v })
	}
}

func (s *TerminalState) Build(ctx widget.BuildContext) widget.Widget {
	if s.errText != "" {
		return collections.Application{
			Controller: s.appCtrl,
			AppBar:     &collections.AppBar{Title: "Terminal"},
			StatusBar:  s.buildStatusBar(ctx),
			Body: widget.Padding{
				Insets: layout.All(24),
				Child:  widget.Text{Content: s.errText},
			},
		}
	}

	return collections.Application{
		Controller: s.appCtrl,
		AppBar:     &collections.AppBar{Title: "Terminal"},
		StatusBar:  s.buildStatusBar(ctx),
		Body: terminalView{
			Term:     s.term,
			OnResize: s.handleResize,
		},
	}
}

func (s *TerminalState) buildStatusBar(ctx widget.BuildContext) widget.Widget {
	status := "Disconnected"
	if s.term != nil && s.term.IsRunning() {
		status = fmt.Sprintf("%dx%d", s.cols, s.rows)
	}
	return widget.Row{
		MainAxisAlignment:  layout.MainSpaceBetween,
		CrossAxisAlignment: layout.CrossCenter,
		Children: []widget.Widget{
			widget.Text{Content: status, Style: terminalMetaStyle(ctx)},
			widget.Text{Content: "Interactive PTY", Style: terminalMetaStyle(ctx)},
		},
	}
}

func (s *TerminalState) buildHeader(ctx widget.BuildContext) widget.Widget {
	th := ctx.Theme
	titleSt := th.TextTheme.TitleMedium
	titleSt.Color = th.ColorScheme.OnSurface
	metaSt := th.TextTheme.LabelSmall
	metaSt.Color = th.ColorScheme.OnSurfaceVariant

	status := "Disconnected"
	if s.term != nil && s.term.IsRunning() {
		status = fmt.Sprintf("%dx%d", s.cols, s.rows)
	}

	return widget.Container{
		Fill:    th.ColorScheme.Surface,
		Border:  th.ColorScheme.Outline,
		Padding: layout.Symmetric(16, 12),
		Child: widget.Row{
			CrossAxisAlignment: layout.CrossCenter,
			Children: []widget.Widget{
				widget.Text{Content: "Terminal", Style: &titleSt},
				widget.Spacer{},
				widget.Text{Content: status, Style: &metaSt},
			},
		},
	}
}

func (s *TerminalState) handleResize(rows, cols int) {
	if s.term == nil || rows <= 0 || cols <= 0 || (rows == s.rows && cols == s.cols) {
		return
	}
	if err := s.term.Resize(rows, cols); err != nil {
		s.SetState(func() { s.errText = err.Error() })
		return
	}
	s.SetState(func() {
		s.rows = rows
		s.cols = cols
	})
}

type terminalView struct {
	Term     *pty.Terminal
	OnResize func(rows, cols int)
}

func (tv terminalView) Layout(c layout.BoxConstraints) geom.Size {
	return c.Biggest()
}

func (tv terminalView) Paint(ctx *paint.Context, offset geom.Point, size geom.Size) {
	if tv.Term == nil || size.Width <= 0 || size.Height <= 0 {
		return
	}

	face := ttf.DefaultMono()
	fontSize := 8.0
	cellW := face.RuneAdvance('M', fontSize)
	cellH := face.LineHeight(fontSize)
	if cellW <= 0 || cellH <= 0 {
		return
	}

	cols := int(size.Width / cellW)
	rows := int(size.Height / cellH)
	if tv.OnResize != nil {
		tv.OnResize(rows, cols)
	}
	if cols <= 0 || rows <= 0 {
		return
	}

	lines, cursorX, cursorY := tv.Term.SnapshotCells()
	start := 0
	if len(lines) > rows {
		start = len(lines) - rows
	}
	visible := lines[start:]
	palette := terminalPalette()

	ctx.FillRect(geom.NewRect(offset.X, offset.Y, size.Width, size.Height), color.FromRGBA8(12, 16, 24, 255))

	for y, row := range visible {
		if y >= rows {
			break
		}
		for x, cell := range row {
			if x >= cols {
				break
			}
			bg := resolveTerminalColor(cell.Style.Bg, true, palette)
			fg := resolveTerminalColor(cell.Style.Fg, false, palette)
			if cell.Style.Reverse {
				bg, fg = fg, bg
			}

			x0 := offset.X + float64(x)*cellW
			y0 := offset.Y + float64(y)*cellH
			if bg.A > 0 {
				ctx.FillRect(geom.NewRect(x0, y0, cellW, cellH), bg)
			}
			ch := cell.Char
			if ch == 0 {
				ch = ' '
			}
			if ch != ' ' {
				ctx.DrawText(string(ch), geom.Pt(x0, y0), face, fontSize, fg)
			}
		}
	}

	cursorRow := cursorY - start
	if cursorRow >= 0 && cursorRow < rows && cursorX >= 0 && cursorX < cols {
		ctx.FillRect(
			geom.NewRect(offset.X+float64(cursorX)*cellW, offset.Y+float64(cursorRow)*cellH+cellH-2, cellW, 2),
			color.FromRGBA8(95, 215, 255, 255),
		)
	}
}

func (tv terminalView) HitTest(pos, offset geom.Point, size geom.Size) bool {
	return geom.NewRect(offset.X, offset.Y, size.Width, size.Height).Contains(pos)
}

func terminalPalette() [16]color.Color {
	return [16]color.Color{
		color.FromRGBA8(22, 27, 34, 255),
		color.FromRGBA8(255, 123, 114, 255),
		color.FromRGBA8(63, 185, 80, 255),
		color.FromRGBA8(210, 153, 34, 255),
		color.FromRGBA8(88, 166, 255, 255),
		color.FromRGBA8(188, 140, 255, 255),
		color.FromRGBA8(57, 211, 192, 255),
		color.FromRGBA8(201, 209, 217, 255),
		color.FromRGBA8(110, 118, 129, 255),
		color.FromRGBA8(255, 160, 153, 255),
		color.FromRGBA8(86, 211, 100, 255),
		color.FromRGBA8(229, 182, 74, 255),
		color.FromRGBA8(121, 192, 255, 255),
		color.FromRGBA8(214, 171, 255, 255),
		color.FromRGBA8(86, 255, 233, 255),
		color.FromRGBA8(240, 246, 252, 255),
	}
}

func terminalMetaStyle(ctx widget.BuildContext) *theme.TextStyle {
	style := ctx.Theme.TextTheme.LabelSmall
	style.Color = ctx.Theme.ColorScheme.OnSurfaceVariant
	return &style
}

func resolveTerminalColor(c pty.Color, background bool, palette [16]color.Color) color.Color {
	if c == pty.ColorDefault {
		if background {
			return color.FromRGBA8(12, 16, 24, 255)
		}
		return color.FromRGBA8(230, 237, 243, 255)
	}
	if c >= 0 && int(c) < len(palette) {
		return palette[int(c)]
	}
	if c >= 16 && c < 256 {
		v := uint8(55 + (int(c)-16)%6*40)
		return color.FromRGBA8(v, v, v, 255)
	}
	if c >= 256 {
		raw := int(c) - 256
		return color.FromRGBA8(uint8(raw>>16), uint8(raw>>8), uint8(raw), 255)
	}
	return color.FromRGBA8(230, 237, 243, 255)
}

func encodeTerminalKey(ev event.KeyEvent) ([]byte, bool) {
	if ev.Mods&event.ModCtrl != 0 {
		if ev.Key >= event.KeyA && ev.Key <= event.KeyZ {
			return []byte{byte(ev.Key-event.KeyA) + 1}, true
		}
	}

	switch ev.Key {
	case event.KeyEnter:
		return []byte("\r"), true
	case event.KeyBackspace:
		return []byte{0x7f}, true
	case event.KeyTab:
		return []byte("\t"), true
	case event.KeyEscape:
		return []byte{0x1b}, true
	case event.KeyArrowUp:
		return []byte("\x1b[A"), true
	case event.KeyArrowDown:
		return []byte("\x1b[B"), true
	case event.KeyArrowRight:
		return []byte("\x1b[C"), true
	case event.KeyArrowLeft:
		return []byte("\x1b[D"), true
	case event.KeyHome:
		return []byte("\x1b[H"), true
	case event.KeyEnd:
		return []byte("\x1b[F"), true
	case event.KeyDelete:
		return []byte("\x1b[3~"), true
	case event.KeyInsert:
		return []byte("\x1b[2~"), true
	case event.KeyPageUp:
		return []byte("\x1b[5~"), true
	case event.KeyPageDown:
		return []byte("\x1b[6~"), true
	case event.KeySpace:
		return []byte(" "), true
	}

	if r := keyCodeToRune(ev.Key, ev.Mods); r != 0 {
		return []byte(string(r)), true
	}
	return nil, false
}

func keyCodeToRune(key event.KeyCode, mods event.Modifiers) rune {
	shift := mods&event.ModShift != 0
	switch {
	case key >= event.KeyA && key <= event.KeyZ:
		r := rune('a' + (key - event.KeyA))
		if shift {
			r = rune(strings.ToUpper(string(r))[0])
		}
		return r
	case key >= event.Key0 && key <= event.Key9:
		return rune('0' + (key - event.Key0))
	}
	switch key {
	case event.KeyMinus:
		if shift {
			return '_'
		}
		return '-'
	case event.KeyEqual:
		if shift {
			return '+'
		}
		return '='
	case event.KeyLeftBracket:
		if shift {
			return '{'
		}
		return '['
	case event.KeyRightBracket:
		if shift {
			return '}'
		}
		return ']'
	case event.KeySemicolon:
		if shift {
			return ':'
		}
		return ';'
	case event.KeyApostrophe:
		if shift {
			return '"'
		}
		return '\''
	case event.KeyGrave:
		if shift {
			return '~'
		}
		return '`'
	case event.KeyBackslash:
		if shift {
			return '|'
		}
		return '\\'
	case event.KeyComma:
		if shift {
			return '<'
		}
		return ','
	case event.KeyPeriod:
		if shift {
			return '>'
		}
		return '.'
	case event.KeySlash:
		if shift {
			return '?'
		}
		return '/'
	}
	return 0
}
