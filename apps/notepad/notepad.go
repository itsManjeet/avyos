package main

import (
	"fmt"
	"runtime"
	"time"

	"avyos.dev/lib/graphics/app"
	"avyos.dev/lib/graphics/collections"
	"avyos.dev/lib/graphics/event"
	"avyos.dev/lib/graphics/geom"
	"avyos.dev/lib/graphics/layout"
	"avyos.dev/lib/graphics/theme"
	"avyos.dev/lib/graphics/widget"
)

type noteTemplate struct {
	Label   string
	Icon    string
	Title   string
	Content string
}

var noteTemplates = []noteTemplate{
	{
		Label:   "Scratch",
		Icon:    "notepad",
		Title:   "Scratch Pad",
		Content: "",
	},
	{
		Label:   "Meeting",
		Icon:    "clipboard",
		Title:   "Meeting Notes",
		Content: "Meeting Notes\n\nAttendees:\n- \n\nAgenda:\n- \n\nNotes:\n- \n\nActions:\n- ",
	},
	{
		Label:   "Journal",
		Icon:    "bookmark",
		Title:   "Journal Entry",
		Content: "Journal Entry\n\nWhat happened today?\n\nWhat needs follow-up?\n\nNext steps:\n- ",
	},
	{
		Label:   "Checklist",
		Icon:    "checklist",
		Title:   "Checklist",
		Content: "Checklist\n\n[ ] First item\n[ ] Second item\n[ ] Third item",
	},
}

type NotepadApp struct{}

func (NotepadApp) CreateState() widget.State { return &NotepadState{} }

type NotepadState struct {
	widget.StateBase

	appCtrl *collections.ApplicationController

	docIndex        int
	title           string
	content         string
	cursor          int
	preferredColumn int
	focused         bool
	dirty           bool
}

func (s *NotepadState) InitState() {
	s.appCtrl = collections.NewApplicationController()
	s.applyTemplate(0)
	s.focused = true

	app.EventHandler = func(e event.Event) {
		if s.handleEditorEvent(e) {
			return
		}
		app.DefaultHandler(e)
	}
}

func (s *NotepadState) Build(ctx widget.BuildContext) widget.Widget {
	return collections.Application{
		Controller: s.appCtrl,
		AppBar: &collections.AppBar{
			TitleWidget: s.buildTitle(ctx),
			Actions: []widget.Widget{
				widget.Button{
					Child:     widget.Text{Content: "New"},
					Variant:   widget.ButtonOutline,
					Tone:      widget.ButtonNeutral,
					OnPressed: s.newNote,
				},
				widget.Button{
					Child:     widget.Text{Content: "Timestamp"},
					Variant:   widget.ButtonOutline,
					Tone:      widget.ButtonNeutral,
					OnPressed: s.insertTimestamp,
				},
				widget.Button{
					Child:     widget.Text{Content: "Clear"},
					Variant:   widget.ButtonGhost,
					Tone:      widget.ButtonNeutral,
					OnPressed: s.clearNote,
				},
			},
		},
		NavBar: &collections.NavBar{
			Destinations: noteDestinations(),
			Selected:     s.docIndex,
			OnSelected:   s.selectTemplate,
			Header:       widget.Text{Content: "Templates", Style: sectionTitleStyle(ctx)},
			Footer:       widget.Text{Content: "Use Ctrl+N for a blank note.", Style: mutedStyle(ctx)},
		},
		StatusBar: s.buildStatusBar(ctx),
		Body: widget.Padding{
			Insets: layout.All(ctx.Theme.Space.Unit(4)),
			Child:  s.buildEditorSurface(ctx),
		},
	}
}

func (s *NotepadState) buildTitle(ctx widget.BuildContext) widget.Widget {
	th := ctx.Theme
	return widget.Column{
		CrossAxisAlignment: layout.CrossStretch,
		MainAxisSize:       layout.MainMin,
		Children: []widget.Widget{
			widget.Text{Content: "Notepad", Style: sectionTitleStyle(ctx)},
			widget.SizedBox{Height: th.Space.Unit(0.5)},
			widget.Text{Content: s.title, Style: mutedStyle(ctx)},
		},
	}
}

func (s *NotepadState) buildEditorSurface(ctx widget.BuildContext) widget.Widget {
	th := ctx.Theme
	editor := widget.GestureDetector{
		OnPointerDownLocal: func(local geom.Point) {
			s.placeCursorFromPoint(local, ctx)
		},
		Child: notepadEditor{
			Content: s.content,
			Cursor:  s.cursor,
			Focused: s.focused,
		},
	}

	return widget.Container{
		Fill:        th.ColorScheme.Surface,
		Radius:      th.Shape.XLargeRadius,
		Border:      th.ColorScheme.OutlineVariant,
		BorderWidth: 1,
		Padding:     layout.All(th.Space.Unit(2)),
		Child: widget.Scroll{
			Axis:  layout.Vertical,
			Child: editor,
		},
	}
}

func (s *NotepadState) buildStatusBar(ctx widget.BuildContext) widget.Widget {
	line, col := cursorLineColumn(s.content, s.cursor)
	state := "Saved"
	if s.dirty {
		state = "Unsaved"
	}
	return widget.Row{
		MainAxisAlignment:  layout.MainSpaceBetween,
		CrossAxisAlignment: layout.CrossCenter,
		Children: []widget.Widget{
			widget.Text{Content: fmt.Sprintf("%s · %d chars", state, len([]rune(s.content))), Style: mutedStyle(ctx)},
			widget.Text{Content: fmt.Sprintf("Line %d, Column %d", line+1, col+1), Style: mutedStyle(ctx)},
		},
	}
}

func noteDestinations() []collections.NavDestination {
	out := make([]collections.NavDestination, 0, len(noteTemplates))
	for _, t := range noteTemplates {
		out = append(out, collections.NavDestination{
			Label: t.Label,
			Icon:  t.Icon,
		})
	}
	return out
}

func (s *NotepadState) selectTemplate(i int) {
	if i < 0 || i >= len(noteTemplates) {
		return
	}
	s.SetState(func() {
		s.applyTemplate(i)
	})
}

func (s *NotepadState) applyTemplate(i int) {
	tpl := noteTemplates[i]
	s.docIndex = i
	s.title = tpl.Title
	s.content = tpl.Content
	s.cursor = len([]rune(s.content))
	s.preferredColumn = currentColumn(s.content, s.cursor)
	s.focused = true
	s.dirty = false
}

func (s *NotepadState) newNote() {
	s.SetState(func() {
		s.docIndex = -1
		s.title = "Untitled Note"
		s.content = ""
		s.cursor = 0
		s.preferredColumn = 0
		s.focused = true
		s.dirty = false
	})
}

func (s *NotepadState) clearNote() {
	s.SetState(func() {
		s.content = ""
		s.cursor = 0
		s.preferredColumn = 0
		s.focused = true
		s.dirty = true
	})
}

func (s *NotepadState) insertTimestamp() {
	s.insertText(time.Now().Format("2006-01-02 15:04"))
}

func (s *NotepadState) handleEditorEvent(e event.Event) bool {
	if !s.focused {
		return false
	}

	switch ev := e.(type) {
	case event.TextInputEvent:
		if ev.Mods&event.ModCtrl != 0 || ev.Mods&event.ModSuper != 0 {
			return false
		}
		s.insertText(string(ev.Rune))
		return true
	case event.KeyEvent:
		if !ev.Down {
			return false
		}
		if ev.Mods&event.ModCtrl != 0 || ev.Mods&event.ModSuper != 0 {
			return s.handleShortcut(ev.Key)
		}
		switch ev.Key {
		case event.KeyEnter:
			s.insertText("\n")
			return true
		case event.KeyTab:
			s.insertText("    ")
			return true
		case event.KeyBackspace:
			s.deleteBackward()
			return true
		case event.KeyDelete:
			s.deleteForward()
			return true
		case event.KeyArrowLeft:
			s.moveCursorLeft()
			return true
		case event.KeyArrowRight:
			s.moveCursorRight()
			return true
		case event.KeyArrowUp:
			s.moveCursorUp()
			return true
		case event.KeyArrowDown:
			s.moveCursorDown()
			return true
		case event.KeyHome:
			s.moveCursorHome()
			return true
		case event.KeyEnd:
			s.moveCursorEnd()
			return true
		}
		if runtime.GOOS != "darwin" {
			if r := printableRuneFromKey(ev.Key, ev.Mods); r != 0 {
				s.insertText(string(r))
				return true
			}
		}
	}

	return false
}

func (s *NotepadState) handleShortcut(key event.KeyCode) bool {
	switch key {
	case event.KeyN:
		s.newNote()
		return true
	case event.KeyT:
		s.insertTimestamp()
		return true
	case event.KeyK:
		s.clearNote()
		return true
	}
	return false
}

func (s *NotepadState) insertText(text string) {
	insert := []rune(text)
	s.SetState(func() {
		runes := []rune(s.content)
		head := append([]rune{}, runes[:s.cursor]...)
		head = append(head, insert...)
		head = append(head, runes[s.cursor:]...)
		s.content = string(head)
		s.cursor += len(insert)
		s.preferredColumn = currentColumn(s.content, s.cursor)
		s.focused = true
		s.dirty = true
	})
}

func (s *NotepadState) deleteBackward() {
	if s.cursor == 0 {
		return
	}
	s.SetState(func() {
		runes := []rune(s.content)
		s.content = string(append(append([]rune{}, runes[:s.cursor-1]...), runes[s.cursor:]...))
		s.cursor--
		s.preferredColumn = currentColumn(s.content, s.cursor)
		s.dirty = true
	})
}

func (s *NotepadState) deleteForward() {
	runes := []rune(s.content)
	if s.cursor >= len(runes) {
		return
	}
	s.SetState(func() {
		s.content = string(append(append([]rune{}, runes[:s.cursor]...), runes[s.cursor+1:]...))
		s.preferredColumn = currentColumn(s.content, s.cursor)
		s.dirty = true
	})
}

func (s *NotepadState) moveCursorLeft() {
	if s.cursor == 0 {
		return
	}
	s.SetState(func() {
		s.cursor--
		s.preferredColumn = currentColumn(s.content, s.cursor)
	})
}

func (s *NotepadState) moveCursorRight() {
	if s.cursor >= len([]rune(s.content)) {
		return
	}
	s.SetState(func() {
		s.cursor++
		s.preferredColumn = currentColumn(s.content, s.cursor)
	})
}

func (s *NotepadState) moveCursorUp() {
	line, _ := cursorLineColumn(s.content, s.cursor)
	if line == 0 {
		return
	}
	s.SetState(func() {
		s.cursor = cursorForLineColumn(s.content, line-1, s.preferredColumn)
	})
}

func (s *NotepadState) moveCursorDown() {
	lines := noteLines(s.content)
	line, _ := cursorLineColumn(s.content, s.cursor)
	if line >= len(lines)-1 {
		return
	}
	s.SetState(func() {
		s.cursor = cursorForLineColumn(s.content, line+1, s.preferredColumn)
	})
}

func (s *NotepadState) moveCursorHome() {
	line, _ := cursorLineColumn(s.content, s.cursor)
	s.SetState(func() {
		s.cursor = cursorForLineColumn(s.content, line, 0)
		s.preferredColumn = 0
	})
}

func (s *NotepadState) moveCursorEnd() {
	line, _ := cursorLineColumn(s.content, s.cursor)
	lines := noteLines(s.content)
	if line < 0 || line >= len(lines) {
		return
	}
	s.SetState(func() {
		s.cursor = lines[line].End
		s.preferredColumn = currentColumn(s.content, s.cursor)
	})
}

func (s *NotepadState) placeCursorFromPoint(local geom.Point, ctx widget.BuildContext) {
	style := editorTextStyle(ctx)
	lines := noteLines(s.content)
	lineHeight := editorLineHeight(style)

	line := int((local.Y - editorPaddingY(ctx)) / lineHeight)
	if line < 0 {
		line = 0
	}
	if line >= len(lines) {
		line = len(lines) - 1
	}

	targetX := local.X - editorTextX(ctx)
	col := 0
	if targetX > 0 {
		width := 0.0
		for i, r := range []rune(lines[line].Text) {
			adv := style.Face.RuneAdvance(r, style.Size)
			if width+adv/2 >= targetX {
				col = i
				break
			}
			width += adv
			col = i + 1
		}
	}

	s.SetState(func() {
		s.cursor = lines[line].Start + col
		s.preferredColumn = col
		s.focused = true
	})
}

func cursorLineColumn(content string, cursor int) (line int, column int) {
	runes := []rune(content)
	if cursor < 0 {
		cursor = 0
	}
	if cursor > len(runes) {
		cursor = len(runes)
	}
	for i := 0; i < cursor; i++ {
		if runes[i] == '\n' {
			line++
			column = 0
			continue
		}
		column++
	}
	return line, column
}

func currentColumn(content string, cursor int) int {
	_, col := cursorLineColumn(content, cursor)
	return col
}

func cursorForLineColumn(content string, targetLine, targetColumn int) int {
	lines := noteLines(content)
	if targetLine < 0 {
		targetLine = 0
	}
	if targetLine >= len(lines) {
		targetLine = len(lines) - 1
	}
	line := lines[targetLine]
	width := len([]rune(line.Text))
	if targetColumn < 0 {
		targetColumn = 0
	}
	if targetColumn > width {
		targetColumn = width
	}
	return line.Start + targetColumn
}

type noteLine struct {
	Text  string
	Start int
	End   int
}

func noteLines(content string) []noteLine {
	runes := []rune(content)
	lines := make([]noteLine, 0, 8)
	start := 0
	for i, r := range runes {
		if r == '\n' {
			lines = append(lines, noteLine{
				Text:  string(runes[start:i]),
				Start: start,
				End:   i,
			})
			start = i + 1
		}
	}
	lines = append(lines, noteLine{
		Text:  string(runes[start:]),
		Start: start,
		End:   len(runes),
	})
	return lines
}

func printableRuneFromKey(key event.KeyCode, mods event.Modifiers) rune {
	shift := mods&event.ModShift != 0
	switch key {
	case event.KeySpace:
		return ' '
	case event.KeyA:
		if shift {
			return 'A'
		}
		return 'a'
	case event.KeyB:
		if shift {
			return 'B'
		}
		return 'b'
	case event.KeyC:
		if shift {
			return 'C'
		}
		return 'c'
	case event.KeyD:
		if shift {
			return 'D'
		}
		return 'd'
	case event.KeyE:
		if shift {
			return 'E'
		}
		return 'e'
	case event.KeyF:
		if shift {
			return 'F'
		}
		return 'f'
	case event.KeyG:
		if shift {
			return 'G'
		}
		return 'g'
	case event.KeyH:
		if shift {
			return 'H'
		}
		return 'h'
	case event.KeyI:
		if shift {
			return 'I'
		}
		return 'i'
	case event.KeyJ:
		if shift {
			return 'J'
		}
		return 'j'
	case event.KeyK:
		if shift {
			return 'K'
		}
		return 'k'
	case event.KeyL:
		if shift {
			return 'L'
		}
		return 'l'
	case event.KeyM:
		if shift {
			return 'M'
		}
		return 'm'
	case event.KeyN:
		if shift {
			return 'N'
		}
		return 'n'
	case event.KeyO:
		if shift {
			return 'O'
		}
		return 'o'
	case event.KeyP:
		if shift {
			return 'P'
		}
		return 'p'
	case event.KeyQ:
		if shift {
			return 'Q'
		}
		return 'q'
	case event.KeyR:
		if shift {
			return 'R'
		}
		return 'r'
	case event.KeyS:
		if shift {
			return 'S'
		}
		return 's'
	case event.KeyT:
		if shift {
			return 'T'
		}
		return 't'
	case event.KeyU:
		if shift {
			return 'U'
		}
		return 'u'
	case event.KeyV:
		if shift {
			return 'V'
		}
		return 'v'
	case event.KeyW:
		if shift {
			return 'W'
		}
		return 'w'
	case event.KeyX:
		if shift {
			return 'X'
		}
		return 'x'
	case event.KeyY:
		if shift {
			return 'Y'
		}
		return 'y'
	case event.KeyZ:
		if shift {
			return 'Z'
		}
		return 'z'
	case event.Key0:
		if shift {
			return ')'
		}
		return '0'
	case event.Key1:
		if shift {
			return '!'
		}
		return '1'
	case event.Key2:
		if shift {
			return '@'
		}
		return '2'
	case event.Key3:
		if shift {
			return '#'
		}
		return '3'
	case event.Key4:
		if shift {
			return '$'
		}
		return '4'
	case event.Key5:
		if shift {
			return '%'
		}
		return '5'
	case event.Key6:
		if shift {
			return '^'
		}
		return '6'
	case event.Key7:
		if shift {
			return '&'
		}
		return '7'
	case event.Key8:
		if shift {
			return '*'
		}
		return '8'
	case event.Key9:
		if shift {
			return '('
		}
		return '9'
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

func sectionTitleStyle(ctx widget.BuildContext) *theme.TextStyle {
	style := ctx.Theme.TextTheme.TitleMedium
	style.Color = ctx.Theme.ColorScheme.OnSurface
	return &style
}

func mutedStyle(ctx widget.BuildContext) *theme.TextStyle {
	style := ctx.Theme.TextTheme.BodySmall
	style.Color = ctx.Theme.ColorScheme.OnSurfaceVariant
	return &style
}
