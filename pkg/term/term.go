/*
 * Copyright (c) 2026 Manjeet Singh <itsmanjeet1998@gmail.com>.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU General Public License as published by
 * the Free Software Foundation, version 3.
 *
 * This program is distributed in the hope that it will be useful, but
 * WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the GNU
 * General Public License for more details.
 *
 * You should have received a copy of the GNU General Public License
 * along with this program. If not, see <http://www.gnu.org/licenses/>.
 *
 */

package term

import (
	"bufio"
	"fmt"
	"os"
	"slices"
	"syscall"
	"unsafe"
)

// Terminal size structure for ioctl
type winsize struct {
	Row    uint16
	Col    uint16
	Xpixel uint16
	Ypixel uint16
}

// Size returns the terminal dimensions (columns, rows).
// It tries stdout, stdin, and stderr in order to find a valid terminal.
func Size() (cols, rows int) {
	ws := &winsize{}

	// Try stdout first
	_, _, errno := syscall.Syscall(
		syscall.SYS_IOCTL,
		uintptr(syscall.Stdout),
		uintptr(syscall.TIOCGWINSZ),
		uintptr(unsafe.Pointer(ws)),
	)
	if errno == 0 && ws.Col > 0 && ws.Row > 0 {
		return int(ws.Col), int(ws.Row)
	}

	// Try stdin
	_, _, errno = syscall.Syscall(
		syscall.SYS_IOCTL,
		uintptr(syscall.Stdin),
		uintptr(syscall.TIOCGWINSZ),
		uintptr(unsafe.Pointer(ws)),
	)
	if errno == 0 && ws.Col > 0 && ws.Row > 0 {
		return int(ws.Col), int(ws.Row)
	}

	// Try stderr
	_, _, errno = syscall.Syscall(
		syscall.SYS_IOCTL,
		uintptr(syscall.Stderr),
		uintptr(syscall.TIOCGWINSZ),
		uintptr(unsafe.Pointer(ws)),
	)
	if errno == 0 && ws.Col > 0 && ws.Row > 0 {
		return int(ws.Col), int(ws.Row)
	}

	// Default fallback for when running in environments without proper terminal
	return 80, 24
}

// IsTerminal returns true if fd is a terminal.
func IsTerminal(fd int) bool {
	var termios syscall.Termios
	_, _, err := syscall.Syscall(
		syscall.SYS_IOCTL,
		uintptr(fd),
		uintptr(syscall.TCGETS),
		uintptr(unsafe.Pointer(&termios)),
	)
	return err == 0
}

// termios holds the terminal state
var origTermios syscall.Termios
var rawModeActive bool

// terminalFd returns the file descriptor to use for terminal operations.
// It checks stdin, stdout, and stderr to find a valid terminal.
func terminalFd() uintptr {
	// Try stdin first (preferred for input operations)
	if IsTerminal(syscall.Stdin) {
		return uintptr(syscall.Stdin)
	}
	// Try stdout
	if IsTerminal(syscall.Stdout) {
		return uintptr(syscall.Stdout)
	}
	// Try stderr
	if IsTerminal(syscall.Stderr) {
		return uintptr(syscall.Stderr)
	}
	// Default to stdin
	return uintptr(syscall.Stdin)
}

// EnableRawMode puts the terminal into raw mode.
func EnableRawMode() error {
	if rawModeActive {
		return nil
	}

	fd := terminalFd()

	// Get current terminal settings
	_, _, errno := syscall.Syscall(
		syscall.SYS_IOCTL,
		fd,
		uintptr(syscall.TCGETS),
		uintptr(unsafe.Pointer(&origTermios)),
	)
	if errno != 0 {
		return errno
	}

	// Copy and modify
	raw := origTermios
	raw.Iflag &^= syscall.BRKINT | syscall.ICRNL | syscall.INPCK | syscall.ISTRIP | syscall.IXON
	// Keep OPOST enabled so \n is converted to \r\n
	raw.Cflag |= syscall.CS8
	raw.Lflag &^= syscall.ECHO | syscall.ICANON | syscall.IEXTEN | syscall.ISIG
	raw.Cc[syscall.VMIN] = 1
	raw.Cc[syscall.VTIME] = 0

	// Apply new settings
	_, _, errno = syscall.Syscall(
		syscall.SYS_IOCTL,
		fd,
		uintptr(syscall.TCSETS),
		uintptr(unsafe.Pointer(&raw)),
	)
	if errno != 0 {
		return errno
	}

	rawModeActive = true
	return nil
}

// DisableRawMode restores the terminal to its original state.
func DisableRawMode() error {
	if !rawModeActive {
		return nil
	}

	fd := terminalFd()

	_, _, errno := syscall.Syscall(
		syscall.SYS_IOCTL,
		fd,
		uintptr(syscall.TCSETS),
		uintptr(unsafe.Pointer(&origTermios)),
	)
	if errno != 0 {
		return errno
	}

	rawModeActive = false
	return nil
}

// Key constants for special keys
const (
	KeyEnter     = 13 // CR (\r) - Note: ReadKey also accepts LF (10/\n) as Enter for VM TTY compatibility
	KeyEscape    = 27
	KeyBackspace = 127
	KeyTab       = 9

	// Extended keys (returned as negative values)
	KeyUp       = -1
	KeyDown     = -2
	KeyRight    = -3
	KeyLeft     = -4
	KeyHome     = -5
	KeyEnd      = -6
	KeyDel      = -7
	KeyPgUp     = -8
	KeyPgDn     = -9
	KeyShiftTab = -10
	KeyF1       = -11
	KeyF2       = -12
	KeyF3       = -13
	KeyF4       = -14
	KeyF5       = -15
	KeyF6       = -16
	KeyF7       = -17
	KeyF8       = -18
	KeyF9       = -19
	KeyF10      = -20
	KeyF11      = -21
	KeyF12      = -22

	// Mouse events (returned as negative values starting from -200)
	KeyMouseEvent = -200

	// Meta/Super key modified keys (returned as negative values starting from -300)
	KeyMetaBase = -300
)

// MouseEvent holds mouse event data
type MouseEvent struct {
	Button  int // 0=left, 1=middle, 2=right, 64=wheel up, 65=wheel down
	X, Y    int // 1-based coordinates
	Release bool
	Motion  bool
	Mod     int // modifier bits: 4=shift, 8=meta, 16=ctrl
}

// Global to store last mouse event
var LastMouseEvent MouseEvent

// ReadKey reads a single key from stdin (requires raw mode).
func ReadKey() (int, error) {
	// Read a byte, potentially from buffer
	b, err := readByte()
	if err != nil {
		return 0, err
	}

	// Handle both CR (13) and LF (10) as Enter
	// Some TTYs (especially in VMs) send LF instead of CR
	if b == 10 {
		return KeyEnter, nil
	}

	// Not an escape sequence
	if b != 27 {
		return int(b), nil
	}

	// Check if more bytes available (escape sequence)
	// Try to read more bytes with a short delay for escape sequences
	seq := make([]byte, 0, 8)

	// Read next byte to determine if it's a sequence or just ESC
	b2, err := readByteTimeout()
	if err != nil || b2 == 0 {
		return KeyEscape, nil
	}
	seq = append(seq, b2)

	// CSI sequence: ESC [
	if b2 == '[' {
		// Read until we get a letter or ~
		for range 16 { // Increased buffer for mouse events
			b3, err := readByteTimeout()
			if err != nil || b3 == 0 {
				break
			}
			seq = append(seq, b3)
			// End of CSI sequence
			if (b3 >= 'A' && b3 <= 'Z') || (b3 >= 'a' && b3 <= 'z') || b3 == '~' {
				break
			}
		}
		// Check for SGR mouse event: <button;x;yM or <button;x;ym
		if len(seq) > 1 && seq[1] == '<' {
			return parseSGRMouse(seq[2:]), nil
		}
		return parseCSI(seq[1:]), nil
	}

	// SS3 sequence: ESC O
	if b2 == 'O' {
		b3, err := readByteTimeout()
		if err != nil || b3 == 0 {
			return KeyEscape, nil
		}
		return parseSS3(b3), nil
	}

	// Alt+key: ESC followed by a character
	if b2 >= 32 && b2 < 127 {
		// Return the key with alt modifier (negative value)
		return -100 - int(b2), nil
	}

	return KeyEscape, nil
}

func readByte() (byte, error) {
	buf := make([]byte, 1)
	n, err := os.Stdin.Read(buf)
	if err != nil {
		return 0, err
	}
	if n == 0 {
		return 0, nil
	}
	return buf[0], nil
}

func readByteTimeout() (byte, error) {
	fd := terminalFd()

	// Save current settings
	var current syscall.Termios
	syscall.Syscall(
		syscall.SYS_IOCTL,
		fd,
		uintptr(syscall.TCGETS),
		uintptr(unsafe.Pointer(&current)),
	)

	// Set timeout mode: VMIN=0 (don't wait for bytes), VTIME=1 (100ms timeout)
	timeout := current
	timeout.Cc[syscall.VMIN] = 0
	timeout.Cc[syscall.VTIME] = 1
	syscall.Syscall(
		syscall.SYS_IOCTL,
		fd,
		uintptr(syscall.TCSETS),
		uintptr(unsafe.Pointer(&timeout)),
	)

	// Read with timeout
	buf := make([]byte, 1)
	n, err := os.Stdin.Read(buf)

	// Restore original settings
	syscall.Syscall(
		syscall.SYS_IOCTL,
		fd,
		uintptr(syscall.TCSETS),
		uintptr(unsafe.Pointer(&current)),
	)

	if err != nil || n == 0 {
		return 0, err
	}
	return buf[0], nil
}

func parseCSI(seq []byte) int {
	if len(seq) == 0 {
		return KeyEscape
	}

	// Single letter sequences: A, B, C, D, H, F, Z
	if len(seq) == 1 {
		switch seq[0] {
		case 'A':
			return KeyUp
		case 'B':
			return KeyDown
		case 'C':
			return KeyRight
		case 'D':
			return KeyLeft
		case 'H':
			return KeyHome
		case 'F':
			return KeyEnd
		case 'Z':
			return KeyShiftTab
		}
	}

	// Numeric sequences: 1~, 3~, 4~, 5~, 6~
	if len(seq) >= 2 && seq[len(seq)-1] == '~' {
		switch seq[0] {
		case '1':
			return KeyHome
		case '3':
			return KeyDel
		case '4':
			return KeyEnd
		case '5':
			return KeyPgUp
		case '6':
			return KeyPgDn
		case '7':
			return KeyHome
		case '8':
			return KeyEnd
		}
	}

	// Modified sequences: 1;2Z (shift+tab), 1;3A (alt+up), etc.
	if len(seq) >= 4 && seq[0] == '1' && seq[1] == ';' {
		modifier := seq[2]
		key := seq[3]
		if modifier == '2' && key == 'Z' {
			return KeyShiftTab
		}
		// Modifier codes: 2=Shift, 3=Alt, 4=Shift+Alt, 5=Ctrl, etc.
		// Return modified arrow keys with modifier encoded
		var baseKey int
		switch key {
		case 'A':
			baseKey = KeyUp
		case 'B':
			baseKey = KeyDown
		case 'C':
			baseKey = KeyRight
		case 'D':
			baseKey = KeyLeft
		default:
			return KeyEscape
		}
		// Encode modifier: subtract 1000 * modifier from base key
		// modifier '3' = Alt, '5' = Ctrl, '2' = Shift
		return baseKey - 1000*int(modifier-'0')
	}

	return KeyEscape
}

func parseSS3(b byte) int {
	switch b {
	case 'A':
		return KeyUp
	case 'B':
		return KeyDown
	case 'C':
		return KeyRight
	case 'D':
		return KeyLeft
	case 'H':
		return KeyHome
	case 'F':
		return KeyEnd
	case 'P':
		return KeyF1
	case 'Q':
		return KeyF2
	case 'R':
		return KeyF3
	case 'S':
		return KeyF4
	}
	return KeyEscape
}

// parseSGRMouse parses SGR mouse events: button;x;yM or button;x;ym
func parseSGRMouse(seq []byte) int {
	if len(seq) < 5 {
		return KeyEscape
	}

	// Find the terminator (M for press, m for release)
	terminator := seq[len(seq)-1]
	if terminator != 'M' && terminator != 'm' {
		return KeyEscape
	}

	// Parse button;x;y
	data := string(seq[:len(seq)-1])
	var button, x, y int
	n, err := fmt.Sscanf(data, "%d;%d;%d", &button, &x, &y)
	if err != nil || n != 3 {
		return KeyEscape
	}

	// Store mouse event details
	LastMouseEvent = MouseEvent{
		Button:  button & 0x43, // Mask to get button number (0-2, 64-65 for wheel)
		X:       x,
		Y:       y,
		Release: terminator == 'm',
		Motion:  (button & 32) != 0,
		Mod:     (button >> 2) & 0x07, // Extract modifier bits
	}

	return KeyMouseEvent
}

// ANSI escape codes
const (
	ClearScreen      = "\033[2J"
	ClearLine        = "\033[2K"
	ClearToEnd       = "\033[K"
	CursorHome       = "\033[H"
	CursorHide       = "\033[?25l"
	CursorShow       = "\033[?25h"
	SaveCursor       = "\033[s"
	RestoreCursor    = "\033[u"
	ScrollUp         = "\033[S"
	ScrollDown       = "\033[T"
	EnableAltScreen  = "\033[?1049h"
	DisableAltScreen = "\033[?1049l"

	// Mouse support (SGR mode for better coordinates)
	EnableMouse  = "\033[?1000h\033[?1002h\033[?1006h" // Basic + button motion + SGR
	DisableMouse = "\033[?1006l\033[?1002l\033[?1000l"
)

// Flush ensures all buffered output is written to stdout.
// This is important when running inside a PTY to ensure immediate display.
func Flush() {
	os.Stdout.Sync()
}

// MoveCursor moves the cursor to the specified position (1-based).
func MoveCursor(row, col int) {
	fmt.Printf("\033[%d;%dH", row, col)
}

// MoveCursorUp moves the cursor up n lines.
func MoveCursorUp(n int) {
	fmt.Printf("\033[%dA", n)
}

// MoveCursorDown moves the cursor down n lines.
func MoveCursorDown(n int) {
	fmt.Printf("\033[%dB", n)
}

// MoveCursorForward moves the cursor forward n columns.
func MoveCursorForward(n int) {
	fmt.Printf("\033[%dC", n)
}

// MoveCursorBack moves the cursor back n columns.
func MoveCursorBack(n int) {
	fmt.Printf("\033[%dD", n)
}

// Clear clears the screen.
func Clear() {
	fmt.Print(ClearScreen + CursorHome)
}

// ClearCurrentLine clears the current line.
func ClearCurrentLine() {
	fmt.Print("\r" + ClearLine)
}

// LineEditor provides simple line editing capabilities.
type LineEditor struct {
	line    []rune
	cursor  int
	history []string
	histIdx int
	prompt  string
}

// NewLineEditor creates a new line editor.
func NewLineEditor(prompt string) *LineEditor {
	return &LineEditor{
		prompt:  prompt,
		history: make([]string, 0),
		histIdx: -1,
	}
}

// SetPrompt sets the prompt string.
func (le *LineEditor) SetPrompt(prompt string) {
	le.prompt = prompt
}

// AddHistory adds a line to history.
func (le *LineEditor) AddHistory(line string) {
	if line == "" {
		return
	}
	// Don't add duplicates
	if len(le.history) > 0 && le.history[len(le.history)-1] == line {
		return
	}
	le.history = append(le.history, line)
	le.histIdx = len(le.history)
}

// ReadLine reads a line of input with editing support.
func (le *LineEditor) ReadLine() (string, error) {
	le.line = nil
	le.cursor = 0
	le.histIdx = len(le.history)

	if err := EnableRawMode(); err != nil {
		// Fall back to simple input
		reader := bufio.NewReader(os.Stdin)
		fmt.Print(le.prompt)
		line, err := reader.ReadString('\n')
		if err != nil {
			return "", err
		}
		return line[:len(line)-1], nil
	}
	defer DisableRawMode()

	le.render()

	for {
		key, err := ReadKey()
		if err != nil {
			return "", err
		}

		switch key {
		case KeyEnter:
			fmt.Println()
			result := string(le.line)
			le.AddHistory(result)
			return result, nil

		case KeyBackspace:
			if le.cursor > 0 {
				le.line = append(le.line[:le.cursor-1], le.line[le.cursor:]...)
				le.cursor--
				le.render()
			}

		case KeyDel:
			if le.cursor < len(le.line) {
				le.line = append(le.line[:le.cursor], le.line[le.cursor+1:]...)
				le.render()
			}

		case KeyLeft:
			if le.cursor > 0 {
				le.cursor--
				le.render()
			}

		case KeyRight:
			if le.cursor < len(le.line) {
				le.cursor++
				le.render()
			}

		case KeyHome:
			le.cursor = 0
			le.render()

		case KeyEnd:
			le.cursor = len(le.line)
			le.render()

		case KeyUp:
			if le.histIdx > 0 {
				le.histIdx--
				le.line = []rune(le.history[le.histIdx])
				le.cursor = len(le.line)
				le.render()
			}

		case KeyDown:
			if le.histIdx < len(le.history)-1 {
				le.histIdx++
				le.line = []rune(le.history[le.histIdx])
				le.cursor = len(le.line)
				le.render()
			} else if le.histIdx == len(le.history)-1 {
				le.histIdx = len(le.history)
				le.line = nil
				le.cursor = 0
				le.render()
			}

		case 3: // Ctrl+C
			fmt.Println("^C")
			le.line = nil
			le.cursor = 0
			le.render()

		case 4: // Ctrl+D
			if len(le.line) == 0 {
				fmt.Println()
				return "", fmt.Errorf("EOF")
			}

		case 21: // Ctrl+U - clear line
			le.line = nil
			le.cursor = 0
			le.render()

		case 11: // Ctrl+K - clear to end
			le.line = le.line[:le.cursor]
			le.render()

		case 1: // Ctrl+A - home
			le.cursor = 0
			le.render()

		case 5: // Ctrl+E - end
			le.cursor = len(le.line)
			le.render()

		default:
			if key >= 32 && key < 127 {
				// Insert character
				le.line = append(le.line[:le.cursor], append([]rune{rune(key)}, le.line[le.cursor:]...)...)
				le.cursor++
				le.render()
			}
		}
	}
}

func (le *LineEditor) render() {
	ClearCurrentLine()
	fmt.Print(le.prompt + string(le.line))
	// Move cursor to correct position
	if le.cursor < len(le.line) {
		MoveCursorBack(len(le.line) - le.cursor)
	}
}

// GetHistory returns the command history.
func (le *LineEditor) GetHistory() []string {
	return le.history
}

// SupportsUnicode returns true if the terminal likely supports unicode.
// It checks TERM and LANG environment variables.
func SupportsUnicode() bool {
	// Check for explicit ASCII mode
	if os.Getenv("avyos_ASCII") == "1" {
		return false
	}

	// Check TERM variable for basic terminals
	term := os.Getenv("TERM")
	basicTerminals := []string{"linux", "vt100", "vt220", "ansi", "dumb"}
	if slices.Contains(basicTerminals, term) {
		return false
	}

	// Check LANG for UTF-8
	lang := os.Getenv("LANG")
	if lang == "" || lang == "C" || lang == "POSIX" {
		return false
	}

	// Check if running in basic console (not a PTY with proper unicode)
	// Check if stdin/stdout is a TTY but not a pseudo-terminal
	if !IsTerminal(int(syscall.Stdout)) {
		return false
	}

	return true
}
