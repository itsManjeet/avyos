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

package pty

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"unsafe"

	"avyos.dev/pkg/fs"
)

// PTY represents a pseudo-terminal pair.
type PTY struct {
	Master *os.File
	Slave  *os.File
	Name   string
}

// Open creates a new pseudo-terminal pair.
func Open() (*PTY, error) {
	// Open the master side
	master, err := openMasterPTY()
	if err != nil {
		return nil, err
	}

	// Get the slave name
	name, err := ptsname(master)
	if err != nil {
		master.Close()
		return nil, err
	}

	// Unlock the slave
	if err := unlockpt(master); err != nil {
		master.Close()
		return nil, err
	}

	// Open the slave side
	// Note: We don't use O_NOCTTY here because the child process may need
	// the slave to become its controlling terminal for proper terminal I/O
	slave, err := os.OpenFile(name, os.O_RDWR, 0)
	if err != nil {
		master.Close()
		return nil, err
	}

	return &PTY{
		Master: master,
		Slave:  slave,
		Name:   name,
	}, nil
}

func openMasterPTY() (*os.File, error) {
	candidates := []string{
		fs.Resolve("device:ptmx"),
		fs.Resolve("device:pts/ptmx"),
	}

	var errs []string
	seen := make(map[string]struct{}, len(candidates))
	for _, path := range candidates {
		path = strings.TrimSpace(path)
		if path == "" {
			continue
		}
		if _, ok := seen[path]; ok {
			continue
		}
		seen[path] = struct{}{}

		f, err := os.OpenFile(path, os.O_RDWR, 0)
		if err == nil {
			return f, nil
		}
		errs = append(errs, fmt.Sprintf("%s: %v", path, err))
	}

	return nil, fmt.Errorf("cannot open PTY master (ptmx): %s", strings.Join(errs, "; "))
}

// Close closes both ends of the PTY.
func (p *PTY) Close() error {
	var err error
	if p.Slave != nil {
		if e := p.Slave.Close(); e != nil {
			err = e
		}
	}
	if p.Master != nil {
		if e := p.Master.Close(); e != nil {
			err = e
		}
	}
	return err
}

// SetSize sets the terminal size.
func (p *PTY) SetSize(rows, cols int) error {
	ws := &winsize{
		Row: uint16(rows),
		Col: uint16(cols),
	}
	_, _, errno := syscall.Syscall(
		syscall.SYS_IOCTL,
		p.Master.Fd(),
		syscall.TIOCSWINSZ,
		uintptr(unsafe.Pointer(ws)),
	)
	if errno != 0 {
		return errno
	}
	return nil
}

// Read reads from the master side.
func (p *PTY) Read(b []byte) (int, error) {
	return p.Master.Read(b)
}

// Write writes to the master side.
func (p *PTY) Write(b []byte) (int, error) {
	return p.Master.Write(b)
}

type winsize struct {
	Row    uint16
	Col    uint16
	Xpixel uint16
	Ypixel uint16
}

func ptsname(f *os.File) (string, error) {
	var n uint32
	_, _, errno := syscall.Syscall(
		syscall.SYS_IOCTL,
		f.Fd(),
		syscall.TIOCGPTN,
		uintptr(unsafe.Pointer(&n)),
	)
	if errno != 0 {
		return "", errno
	}
	return "/cache/kernel/devices/pts/" + itoa(int(n)), nil
}

func unlockpt(f *os.File) error {
	var unlock int32 = 0
	_, _, errno := syscall.Syscall(
		syscall.SYS_IOCTL,
		f.Fd(),
		syscall.TIOCSPTLCK,
		uintptr(unsafe.Pointer(&unlock)),
	)
	if errno != 0 {
		return errno
	}
	return nil
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	result := ""
	for n > 0 {
		result = string(rune('0'+n%10)) + result
		n /= 10
	}
	return result
}

// StartProcess starts a process attached to the PTY.
func (p *PTY) StartProcess(name string, args []string, env []string) (*os.Process, error) {
	if env == nil {
		env = os.Environ()
	}

	// Add TERM environment variable
	hasTerm := false
	hasPath := false
	for _, e := range env {
		if len(e) >= 5 && e[:5] == "TERM=" {
			hasTerm = true
		}
		if len(e) >= 5 && e[:5] == "PATH=" {
			hasPath = true
		}
	}
	if !hasTerm {
		env = append(env, "TERM=xterm-256color")
	}
	if !hasPath {
		env = append(env, "PATH=/cmd:/avyos/cmd:/bin:/usr/bin")
	}

	attr := &os.ProcAttr{
		Files: []*os.File{p.Slave, p.Slave, p.Slave},
		Env:   env,
		Sys: &syscall.SysProcAttr{
			Setsid:  true,
			Setctty: true,
			Ctty:    0,
		},
	}

	proc, err := os.StartProcess(name, append([]string{name}, args...), attr)
	if err != nil {
		return nil, err
	}

	return proc, nil
}

// StartShell starts the default shell attached to the PTY.
func (p *PTY) StartShell() (*os.Process, error) {
	shell, err := resolveShellExecutable()
	if err != nil {
		return nil, err
	}
	return p.StartProcess(shell, nil, nil)
}

func resolveShellExecutable() (string, error) {
	seen := make(map[string]struct{}, 8)
	candidates := []string{
		strings.TrimSpace(os.Getenv("SHELL")),
		fs.Resolve("cmd:shell"),
		"/avyos/cmd/shell",
	}

	for _, candidate := range candidates {
		candidate = strings.TrimSpace(candidate)
		if candidate == "" {
			continue
		}
		if _, ok := seen[candidate]; ok {
			continue
		}
		seen[candidate] = struct{}{}

		if strings.Contains(candidate, "/") {
			if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
				return candidate, nil
			}
			continue
		}

		if resolved, err := exec.LookPath(candidate); err == nil && strings.TrimSpace(resolved) != "" {
			return resolved, nil
		}
	}

	return "", errors.New("no shell executable found")
}

// Color represents a terminal color.
type Color int32

// Standard ANSI colors
const (
	ColorDefault Color = -1
	ColorBlack   Color = 0
	ColorRed     Color = 1
	ColorGreen   Color = 2
	ColorYellow  Color = 3
	ColorBlue    Color = 4
	ColorMagenta Color = 5
	ColorCyan    Color = 6
	ColorWhite   Color = 7

	// Bright colors (8-15)
	ColorBrightBlack   Color = 8
	ColorBrightRed     Color = 9
	ColorBrightGreen   Color = 10
	ColorBrightYellow  Color = 11
	ColorBrightBlue    Color = 12
	ColorBrightMagenta Color = 13
	ColorBrightCyan    Color = 14
	ColorBrightWhite   Color = 15
)

// RGB256 creates a 256-color palette color (16-255).
func RGB256(n int) Color {
	return Color(n)
}

// RGBTrue creates a true color (24-bit). Encoded as 256 + (r<<16 | g<<8 | b).
func RGBTrue(r, g, b uint8) Color {
	return Color(256 + int(r)<<16 + int(g)<<8 + int(b))
}

// Style holds text attributes for a cell.
type Style struct {
	Fg        Color
	Bg        Color
	Bold      bool
	Dim       bool
	Italic    bool
	Underline bool
	Blink     bool
	Reverse   bool
}

// DefaultStyle returns the default terminal style.
func DefaultStyle() Style {
	return Style{Fg: ColorDefault, Bg: ColorDefault}
}

// Cell represents a single character cell with style.
type Cell struct {
	Char  rune
	Style Style
}

// Terminal represents a terminal session with a PTY and process.
type Terminal struct {
	pty          *PTY
	process      *os.Process
	buffer       [][]Cell
	scrollback   [][]Cell
	scrollStart  int
	scrollLimit  int
	cursorX      int
	cursorY      int
	savedX       int
	savedY       int
	scrollY      int
	scrollTop    int // Top of scroll region (0-based, inclusive)
	scrollBottom int // Bottom of scroll region (0-based, inclusive)
	width        int
	height       int
	closed       bool
	escState     int    // 0=normal, 1=ESC, 2=CSI, 3=OSC
	escBuf       []byte // Buffer for escape sequence
	utf8Buf      []byte // Buffer for incomplete UTF-8 sequences
	style        Style  // Current text style
	mu           sync.Mutex
	version      atomic.Uint64
}

// Escape sequence states
const (
	stateNormal = iota
	stateESC
	stateCSI
	stateOSC
)

const defaultScrollbackLimit = 2000

// NewTerminal creates a new terminal with the given size.
func NewTerminal(rows, cols int) (*Terminal, error) {
	p, err := Open()
	if err != nil {
		return nil, err
	}

	if err := p.SetSize(rows, cols); err != nil {
		p.Close()
		return nil, err
	}

	proc, err := p.StartShell()
	if err != nil {
		p.Close()
		return nil, err
	}

	t := &Terminal{
		pty:          p,
		process:      proc,
		buffer:       make([][]Cell, rows),
		scrollback:   make([][]Cell, 0, defaultScrollbackLimit),
		scrollLimit:  defaultScrollbackLimit,
		width:        cols,
		height:       rows,
		scrollTop:    0,
		scrollBottom: rows - 1,
		escBuf:       make([]byte, 0, 64),
		style:        DefaultStyle(),
	}

	// Initialize buffer
	for i := range t.buffer {
		t.buffer[i] = make([]Cell, cols)
		for j := range t.buffer[i] {
			t.buffer[i][j] = Cell{Char: ' ', Style: DefaultStyle()}
		}
	}

	// Start reading from PTY
	go t.readLoop()

	return t, nil
}

func (t *Terminal) readLoop() {
	buf := make([]byte, 4096)
	for !t.closed {
		n, err := t.pty.Read(buf)
		if err != nil {
			break
		}
		if n > 0 {
			t.mu.Lock()
			t.processOutput(buf[:n])
			t.mu.Unlock()
			t.version.Add(1)
		}
	}
}

func (t *Terminal) processOutput(data []byte) {
	// Prepend any incomplete UTF-8 sequence from previous call
	if len(t.utf8Buf) > 0 {
		data = append(t.utf8Buf, data...)
		t.utf8Buf = nil
	}

	i := 0
	for i < len(data) {
		b := data[i]

		// If we're in an escape sequence state, process as bytes
		if t.escState != stateNormal {
			t.processByte(b)
			i++
			continue
		}

		// Check for escape character
		if b == 0x1b {
			t.processByte(b)
			i++
			continue
		}

		// Check for control characters (0x00-0x1F except ESC)
		if b < 0x20 {
			t.processByte(b)
			i++
			continue
		}

		// Handle UTF-8 decoding
		if b < 0x80 {
			// ASCII character
			t.putChar(rune(b))
			i++
		} else {
			// UTF-8 multi-byte sequence
			r, size := decodeUTF8(data[i:])
			if size == 0 {
				// Incomplete sequence - save for next call
				t.utf8Buf = append(t.utf8Buf, data[i:]...)
				break
			}
			if r != 0xFFFD { // Valid rune (not replacement character for invalid sequence)
				t.putChar(r)
			}
			i += size
		}
	}
}

// decodeUTF8 decodes a UTF-8 character from bytes.
// Returns the rune and the number of bytes consumed.
// Returns (0, 0) if the sequence is incomplete.
// Returns (0xFFFD, 1) for invalid sequences.
func decodeUTF8(data []byte) (rune, int) {
	if len(data) == 0 {
		return 0, 0
	}

	b := data[0]

	// Determine expected length from first byte
	var expectedLen int
	var r rune

	switch {
	case b&0x80 == 0:
		// ASCII
		return rune(b), 1
	case b&0xE0 == 0xC0:
		// 2-byte sequence
		expectedLen = 2
		r = rune(b & 0x1F)
	case b&0xF0 == 0xE0:
		// 3-byte sequence
		expectedLen = 3
		r = rune(b & 0x0F)
	case b&0xF8 == 0xF0:
		// 4-byte sequence
		expectedLen = 4
		r = rune(b & 0x07)
	default:
		// Invalid UTF-8 start byte
		return 0xFFFD, 1
	}

	// Check if we have enough bytes
	if len(data) < expectedLen {
		return 0, 0 // Incomplete
	}

	// Decode continuation bytes
	for i := 1; i < expectedLen; i++ {
		if data[i]&0xC0 != 0x80 {
			// Invalid continuation byte
			return 0xFFFD, 1
		}
		r = (r << 6) | rune(data[i]&0x3F)
	}

	return r, expectedLen
}

func (t *Terminal) processByte(b byte) {
	switch t.escState {
	case stateNormal:
		t.processNormal(b)
	case stateESC:
		t.processESC(b)
	case stateCSI:
		t.processCSI(b)
	case stateOSC:
		t.processOSC(b)
	}
}

func (t *Terminal) processNormal(b byte) {
	switch b {
	case 0x1b: // ESC
		t.escState = stateESC
		t.escBuf = t.escBuf[:0]
	case '\n': // Line feed
		if t.cursorY == t.scrollBottom {
			// At bottom of scroll region, scroll up
			t.scrollUp(1)
		} else if t.cursorY < t.height-1 {
			// Not at bottom, just move down
			t.cursorY++
		}
	case '\r': // Carriage return
		t.cursorX = 0
	case '\b': // Backspace
		if t.cursorX > 0 {
			t.cursorX--
		}
	case '\t': // Tab
		t.cursorX = (t.cursorX + 8) &^ 7
		if t.cursorX >= t.width {
			t.cursorX = t.width - 1
		}
	case 0x07: // Bell - ignore
	case 0x0e, 0x0f: // Shift In/Out - ignore
	default:
		if b >= 32 {
			t.putChar(rune(b))
		}
	}
}

func (t *Terminal) processESC(b byte) {
	switch b {
	case '[': // CSI
		t.escState = stateCSI
		t.escBuf = t.escBuf[:0]
	case ']': // OSC
		t.escState = stateOSC
		t.escBuf = t.escBuf[:0]
	case '(', ')': // Character set - ignore next byte
		t.escState = stateNormal
	case '7': // Save cursor
		t.savedX = t.cursorX
		t.savedY = t.cursorY
		t.escState = stateNormal
	case '8': // Restore cursor
		t.cursorX = t.savedX
		t.cursorY = t.savedY
		t.escState = stateNormal
	case 'c': // Reset
		t.reset()
		t.escState = stateNormal
	case 'D': // Index (move down) - respects scroll region
		if t.cursorY == t.scrollBottom {
			// At bottom of scroll region, scroll up
			t.scrollUp(1)
		} else if t.cursorY < t.height-1 {
			t.cursorY++
		}
		t.escState = stateNormal
	case 'E': // Next line - respects scroll region
		t.cursorX = 0
		if t.cursorY == t.scrollBottom {
			t.scrollUp(1)
		} else if t.cursorY < t.height-1 {
			t.cursorY++
		}
		t.escState = stateNormal
	case 'M': // Reverse index (move up) - respects scroll region
		if t.cursorY == t.scrollTop {
			// At top of scroll region, scroll down
			t.scrollDown(1)
		} else if t.cursorY > 0 {
			t.cursorY--
		}
		t.escState = stateNormal
	default:
		t.escState = stateNormal
	}
}

func (t *Terminal) processCSI(b byte) {
	// Collect parameters
	if (b >= '0' && b <= '9') || b == ';' || b == '?' || b == '>' || b == '!' {
		t.escBuf = append(t.escBuf, b)
		return
	}

	// Execute CSI command
	t.executeCSI(b)
	t.escState = stateNormal
}

func (t *Terminal) processOSC(b byte) {
	// OSC ends with BEL (0x07) or ST (ESC \)
	if b == 0x07 {
		t.escState = stateNormal
		return
	}
	if b == 0x1b {
		// Might be ST, just reset
		t.escState = stateNormal
		return
	}
	// Ignore OSC content
}

func (t *Terminal) executeCSI(cmd byte) {
	params := t.parseCSIParams()

	switch cmd {
	case 'A': // Cursor up
		n := 1
		if len(params) > 0 && params[0] > 0 {
			n = params[0]
		}
		t.cursorY -= n
		if t.cursorY < 0 {
			t.cursorY = 0
		}

	case 'B': // Cursor down
		n := 1
		if len(params) > 0 && params[0] > 0 {
			n = params[0]
		}
		t.cursorY += n
		if t.cursorY >= t.height {
			t.cursorY = t.height - 1
		}

	case 'C': // Cursor forward
		n := 1
		if len(params) > 0 && params[0] > 0 {
			n = params[0]
		}
		t.cursorX += n
		if t.cursorX >= t.width {
			t.cursorX = t.width - 1
		}

	case 'D': // Cursor backward
		n := 1
		if len(params) > 0 && params[0] > 0 {
			n = params[0]
		}
		t.cursorX -= n
		if t.cursorX < 0 {
			t.cursorX = 0
		}

	case 'E': // Cursor next line
		n := 1
		if len(params) > 0 && params[0] > 0 {
			n = params[0]
		}
		t.cursorX = 0
		t.cursorY += n
		if t.cursorY >= t.height {
			t.cursorY = t.height - 1
		}

	case 'F': // Cursor previous line
		n := 1
		if len(params) > 0 && params[0] > 0 {
			n = params[0]
		}
		t.cursorX = 0
		t.cursorY -= n
		if t.cursorY < 0 {
			t.cursorY = 0
		}

	case 'G': // Cursor horizontal absolute
		n := 1
		if len(params) > 0 && params[0] > 0 {
			n = params[0]
		}
		t.cursorX = n - 1
		if t.cursorX >= t.width {
			t.cursorX = t.width - 1
		}

	case 'H', 'f': // Cursor position
		row, col := 1, 1
		if len(params) > 0 && params[0] > 0 {
			row = params[0]
		}
		if len(params) > 1 && params[1] > 0 {
			col = params[1]
		}
		t.cursorY = row - 1
		t.cursorX = col - 1
		if t.cursorY >= t.height {
			t.cursorY = t.height - 1
		}
		if t.cursorX >= t.width {
			t.cursorX = t.width - 1
		}
		if t.cursorY < 0 {
			t.cursorY = 0
		}
		if t.cursorX < 0 {
			t.cursorX = 0
		}

	case 'J': // Erase in display
		n := 0
		if len(params) > 0 {
			n = params[0]
		}
		switch n {
		case 0: // Clear from cursor to end
			t.clearLine(t.cursorY, t.cursorX, t.width)
			for y := t.cursorY + 1; y < t.height; y++ {
				t.clearLine(y, 0, t.width)
			}
		case 1: // Clear from start to cursor
			for y := 0; y < t.cursorY; y++ {
				t.clearLine(y, 0, t.width)
			}
			t.clearLine(t.cursorY, 0, t.cursorX+1)
		case 2, 3: // Clear entire screen
			// Note: ED (Erase in Display) should NOT move the cursor
			// Cursor positioning is done separately via CSI H
			for y := 0; y < t.height; y++ {
				t.clearLine(y, 0, t.width)
			}
		}

	case 'K': // Erase in line
		n := 0
		if len(params) > 0 {
			n = params[0]
		}
		switch n {
		case 0: // Clear from cursor to end of line
			t.clearLine(t.cursorY, t.cursorX, t.width)
		case 1: // Clear from start of line to cursor
			t.clearLine(t.cursorY, 0, t.cursorX+1)
		case 2: // Clear entire line
			t.clearLine(t.cursorY, 0, t.width)
		}

	case 'L': // Insert lines
		n := 1
		if len(params) > 0 && params[0] > 0 {
			n = params[0]
		}
		t.insertLines(n)

	case 'M': // Delete lines
		n := 1
		if len(params) > 0 && params[0] > 0 {
			n = params[0]
		}
		t.deleteLines(n)

	case 'P': // Delete characters
		n := 1
		if len(params) > 0 && params[0] > 0 {
			n = params[0]
		}
		t.deleteChars(n)

	case '@': // Insert characters
		n := 1
		if len(params) > 0 && params[0] > 0 {
			n = params[0]
		}
		t.insertChars(n)

	case 'd': // Vertical position absolute
		n := 1
		if len(params) > 0 && params[0] > 0 {
			n = params[0]
		}
		t.cursorY = n - 1
		if t.cursorY >= t.height {
			t.cursorY = t.height - 1
		}

	case 'm': // SGR - Select Graphic Rendition (colors/attributes)
		t.processSGR(params)

	case 's': // Save cursor
		t.savedX = t.cursorX
		t.savedY = t.cursorY

	case 'u': // Restore cursor
		t.cursorX = t.savedX
		t.cursorY = t.savedY

	case 'h', 'l': // Set/reset mode
		// Check for DEC private modes (sequences starting with ?)
		isPrivate := len(t.escBuf) > 0 && t.escBuf[0] == '?'
		if isPrivate && len(params) > 0 {
			// Handle common DEC private modes - most are no-ops for our emulator
			// but we need to acknowledge them to prevent issues
			// 25 = cursor visibility (handled by app)
			// 1049 = alternate screen buffer (handled by app)
			// 7 = autowrap mode
			// 1 = application cursor keys
			// 12 = cursor blink
			// etc.
		}

	case 'r': // DECSTBM - Set Top and Bottom Margins (scroll region)
		top := 1
		bottom := t.height
		if len(params) > 0 && params[0] > 0 {
			top = params[0]
		}
		if len(params) > 1 && params[1] > 0 {
			bottom = params[1]
		}
		// Convert to 0-based and clamp
		t.scrollTop = top - 1
		t.scrollBottom = bottom - 1
		if t.scrollTop < 0 {
			t.scrollTop = 0
		}
		if t.scrollBottom >= t.height {
			t.scrollBottom = t.height - 1
		}
		if t.scrollTop > t.scrollBottom {
			t.scrollTop = 0
			t.scrollBottom = t.height - 1
		}
		// Move cursor to home position
		t.cursorX = 0
		t.cursorY = 0

	case 'S': // SU - Scroll Up (pan down)
		n := 1
		if len(params) > 0 && params[0] > 0 {
			n = params[0]
		}
		t.scrollUp(n)

	case 'T': // SD - Scroll Down (pan up)
		n := 1
		if len(params) > 0 && params[0] > 0 {
			n = params[0]
		}
		t.scrollDown(n)

	case 'c': // Device attributes - ignore

	case 'n': // Device status report - ignore
	}
}

func (t *Terminal) parseCSIParams() []int {
	params := make([]int, 0, 4)
	num := 0
	hasNum := false

	for _, b := range t.escBuf {
		if b >= '0' && b <= '9' {
			num = num*10 + int(b-'0')
			hasNum = true
		} else if b == ';' {
			if hasNum {
				params = append(params, num)
			} else {
				params = append(params, 0)
			}
			num = 0
			hasNum = false
		}
		// Skip '?' and other modifiers
	}

	if hasNum {
		params = append(params, num)
	}

	return params
}

// processSGR processes SGR (Select Graphic Rendition) escape sequences for colors and attributes.
func (t *Terminal) processSGR(params []int) {
	if len(params) == 0 {
		// ESC[m is equivalent to ESC[0m (reset)
		params = []int{0}
	}

	i := 0
	for i < len(params) {
		p := params[i]
		switch p {
		case 0: // Reset all attributes
			t.style = DefaultStyle()
		case 1: // Bold
			t.style.Bold = true
		case 2: // Dim
			t.style.Dim = true
		case 3: // Italic
			t.style.Italic = true
		case 4: // Underline
			t.style.Underline = true
		case 5, 6: // Blink (slow/rapid)
			t.style.Blink = true
		case 7: // Reverse video
			t.style.Reverse = true
		case 21: // Bold off (or double underline)
			t.style.Bold = false
		case 22: // Normal intensity (bold/dim off)
			t.style.Bold = false
			t.style.Dim = false
		case 23: // Italic off
			t.style.Italic = false
		case 24: // Underline off
			t.style.Underline = false
		case 25: // Blink off
			t.style.Blink = false
		case 27: // Reverse off
			t.style.Reverse = false

		// Foreground colors (30-37)
		case 30:
			t.style.Fg = ColorBlack
		case 31:
			t.style.Fg = ColorRed
		case 32:
			t.style.Fg = ColorGreen
		case 33:
			t.style.Fg = ColorYellow
		case 34:
			t.style.Fg = ColorBlue
		case 35:
			t.style.Fg = ColorMagenta
		case 36:
			t.style.Fg = ColorCyan
		case 37:
			t.style.Fg = ColorWhite
		case 38: // Extended foreground color
			if i+1 < len(params) {
				switch params[i+1] {
				case 5: // 256-color palette
					if i+2 < len(params) {
						t.style.Fg = Color(params[i+2])
						i += 2
					}
				case 2: // True color RGB
					if i+4 < len(params) {
						r, g, b := params[i+2], params[i+3], params[i+4]
						t.style.Fg = RGBTrue(uint8(r), uint8(g), uint8(b))
						i += 4
					}
				}
			}
		case 39: // Default foreground
			t.style.Fg = ColorDefault

		// Background colors (40-47)
		case 40:
			t.style.Bg = ColorBlack
		case 41:
			t.style.Bg = ColorRed
		case 42:
			t.style.Bg = ColorGreen
		case 43:
			t.style.Bg = ColorYellow
		case 44:
			t.style.Bg = ColorBlue
		case 45:
			t.style.Bg = ColorMagenta
		case 46:
			t.style.Bg = ColorCyan
		case 47:
			t.style.Bg = ColorWhite
		case 48: // Extended background color
			if i+1 < len(params) {
				switch params[i+1] {
				case 5: // 256-color palette
					if i+2 < len(params) {
						t.style.Bg = Color(params[i+2])
						i += 2
					}
				case 2: // True color RGB
					if i+4 < len(params) {
						r, g, b := params[i+2], params[i+3], params[i+4]
						t.style.Bg = RGBTrue(uint8(r), uint8(g), uint8(b))
						i += 4
					}
				}
			}
		case 49: // Default background
			t.style.Bg = ColorDefault

		// Bright foreground colors (90-97)
		case 90:
			t.style.Fg = ColorBrightBlack
		case 91:
			t.style.Fg = ColorBrightRed
		case 92:
			t.style.Fg = ColorBrightGreen
		case 93:
			t.style.Fg = ColorBrightYellow
		case 94:
			t.style.Fg = ColorBrightBlue
		case 95:
			t.style.Fg = ColorBrightMagenta
		case 96:
			t.style.Fg = ColorBrightCyan
		case 97:
			t.style.Fg = ColorBrightWhite

		// Bright background colors (100-107)
		case 100:
			t.style.Bg = ColorBrightBlack
		case 101:
			t.style.Bg = ColorBrightRed
		case 102:
			t.style.Bg = ColorBrightGreen
		case 103:
			t.style.Bg = ColorBrightYellow
		case 104:
			t.style.Bg = ColorBrightBlue
		case 105:
			t.style.Bg = ColorBrightMagenta
		case 106:
			t.style.Bg = ColorBrightCyan
		case 107:
			t.style.Bg = ColorBrightWhite
		}
		i++
	}
}

func (t *Terminal) putChar(r rune) {
	if t.cursorY >= 0 && t.cursorY < t.height &&
		t.cursorX >= 0 && t.cursorX < t.width {
		t.buffer[t.cursorY][t.cursorX] = Cell{Char: r, Style: t.style}
	}
	t.cursorX++
	if t.cursorX >= t.width {
		t.cursorX = 0
		t.cursorY++
		// Respect scroll region during line wrap
		if t.cursorY > t.scrollBottom {
			t.cursorY = t.scrollBottom
			t.scrollUp(1)
		}
	}
}

func (t *Terminal) clearLine(y, startX, endX int) {
	if y < 0 || y >= t.height {
		return
	}
	clearCell := Cell{Char: ' ', Style: DefaultStyle()}
	for x := startX; x < endX && x < t.width; x++ {
		if x >= 0 {
			t.buffer[y][x] = clearCell
		}
	}
}

func (t *Terminal) insertLines(n int) {
	// Only works within scroll region and when cursor is within it
	if t.cursorY < t.scrollTop || t.cursorY > t.scrollBottom {
		return
	}
	clearCell := Cell{Char: ' ', Style: DefaultStyle()}
	for range n {
		// Shift lines down within scroll region
		for y := t.scrollBottom; y > t.cursorY; y-- {
			t.buffer[y] = t.buffer[y-1]
		}
		// Clear current line
		t.buffer[t.cursorY] = make([]Cell, t.width)
		for x := range t.buffer[t.cursorY] {
			t.buffer[t.cursorY][x] = clearCell
		}
	}
}

func (t *Terminal) deleteLines(n int) {
	// Only works within scroll region and when cursor is within it
	if t.cursorY < t.scrollTop || t.cursorY > t.scrollBottom {
		return
	}
	clearCell := Cell{Char: ' ', Style: DefaultStyle()}
	for range n {
		// Shift lines up within scroll region
		for y := t.cursorY; y < t.scrollBottom; y++ {
			t.buffer[y] = t.buffer[y+1]
		}
		// Clear bottom line of scroll region
		t.buffer[t.scrollBottom] = make([]Cell, t.width)
		for x := range t.buffer[t.scrollBottom] {
			t.buffer[t.scrollBottom][x] = clearCell
		}
	}
}

func (t *Terminal) insertChars(n int) {
	if t.cursorY < 0 || t.cursorY >= t.height {
		return
	}
	line := t.buffer[t.cursorY]
	clearCell := Cell{Char: ' ', Style: DefaultStyle()}
	for range n {
		// Shift characters right
		for x := t.width - 1; x > t.cursorX; x-- {
			line[x] = line[x-1]
		}
		if t.cursorX < t.width {
			line[t.cursorX] = clearCell
		}
	}
}

func (t *Terminal) deleteChars(n int) {
	if t.cursorY < 0 || t.cursorY >= t.height {
		return
	}
	line := t.buffer[t.cursorY]
	clearCell := Cell{Char: ' ', Style: DefaultStyle()}
	for range n {
		// Shift characters left
		for x := t.cursorX; x < t.width-1; x++ {
			line[x] = line[x+1]
		}
		if t.width > 0 {
			line[t.width-1] = clearCell
		}
	}
}

func (t *Terminal) reset() {
	t.cursorX = 0
	t.cursorY = 0
	t.style = DefaultStyle()
	clearCell := Cell{Char: ' ', Style: DefaultStyle()}
	for y := 0; y < t.height; y++ {
		for x := 0; x < t.width; x++ {
			t.buffer[y][x] = clearCell
		}
	}
}

func (t *Terminal) scroll() {
	// Scroll within the scroll region
	t.scrollUp(1)
	// Keep cursor at the bottom of scroll region
	if t.cursorY > t.scrollBottom {
		t.cursorY = t.scrollBottom
	}
}

// scrollUp scrolls the content up within the scroll region (lines move up, new blank lines at bottom)
func (t *Terminal) scrollUp(n int) {
	clearCell := Cell{Char: ' ', Style: DefaultStyle()}
	for range n {
		// Only append to scrollback when the top row leaves the visible screen.
		if t.scrollTop == 0 && t.scrollBottom >= t.scrollTop && t.scrollTop < len(t.buffer) {
			t.pushScrollbackLine(t.buffer[t.scrollTop])
		}
		// Shift lines up within scroll region
		for y := t.scrollTop; y < t.scrollBottom; y++ {
			t.buffer[y] = t.buffer[y+1]
		}
		// Clear the bottom line of scroll region
		t.buffer[t.scrollBottom] = make([]Cell, t.width)
		for x := range t.buffer[t.scrollBottom] {
			t.buffer[t.scrollBottom][x] = clearCell
		}
	}
}

func (t *Terminal) pushScrollbackLine(line []Cell) {
	if t.scrollLimit <= 0 || len(line) == 0 {
		return
	}
	row := make([]Cell, len(line))
	copy(row, line)
	if len(t.scrollback) < t.scrollLimit {
		t.scrollback = append(t.scrollback, row)
		return
	}
	t.scrollback[t.scrollStart] = row
	t.scrollStart = (t.scrollStart + 1) % t.scrollLimit
}

// scrollDown scrolls the content down within the scroll region (lines move down, new blank lines at top)
func (t *Terminal) scrollDown(n int) {
	clearCell := Cell{Char: ' ', Style: DefaultStyle()}
	for range n {
		// Shift lines down within scroll region
		for y := t.scrollBottom; y > t.scrollTop; y-- {
			t.buffer[y] = t.buffer[y-1]
		}
		// Clear the top line of scroll region
		t.buffer[t.scrollTop] = make([]Cell, t.width)
		for x := range t.buffer[t.scrollTop] {
			t.buffer[t.scrollTop][x] = clearCell
		}
	}
}

// Write sends input to the terminal.
func (t *Terminal) Write(data []byte) error {
	_, err := t.pty.Write(data)
	return err
}

// WriteString sends a string to the terminal.
func (t *Terminal) WriteString(s string) error {
	return t.Write([]byte(s))
}

// GetLine returns a line from the buffer as a string (characters only, no styles).
func (t *Terminal) GetLine(y int) string {
	t.mu.Lock()
	defer t.mu.Unlock()
	if y >= 0 && y < len(t.buffer) {
		runes := make([]rune, len(t.buffer[y]))
		for i, cell := range t.buffer[y] {
			runes[i] = cell.Char
		}
		return string(runes)
	}
	return ""
}

// GetBuffer returns the entire buffer as strings (characters only, no styles).
func (t *Terminal) GetBuffer() []string {
	t.mu.Lock()
	defer t.mu.Unlock()
	lines := make([]string, len(t.buffer))
	for i, line := range t.buffer {
		runes := make([]rune, len(line))
		for j, cell := range line {
			runes[j] = cell.Char
		}
		lines[i] = string(runes)
	}
	return lines
}

// GetBufferRunes returns the entire buffer as rune slices (characters only, no styles).
// This is more efficient for rendering as it avoids UTF-8 encoding/decoding
// and provides correct column indices.
func (t *Terminal) GetBufferRunes() [][]rune {
	t.mu.Lock()
	defer t.mu.Unlock()
	lines := make([][]rune, len(t.buffer))
	for i, line := range t.buffer {
		lines[i] = make([]rune, len(line))
		for j, cell := range line {
			lines[i][j] = cell.Char
		}
	}
	return lines
}

// SnapshotRunes returns scrollback + active buffer and the absolute cursor position.
// Cursor Y is relative to the returned lines.
func (t *Terminal) SnapshotRunes() (lines [][]rune, cursorX, cursorY int) {
	t.mu.Lock()
	defer t.mu.Unlock()

	scrollLen := len(t.scrollback)
	lines = make([][]rune, 0, scrollLen+len(t.buffer))

	for i := range scrollLen {
		idx := i
		if scrollLen > 0 {
			idx = (t.scrollStart + i) % scrollLen
		}
		src := t.scrollback[idx]
		dst := make([]rune, len(src))
		for j := range src {
			dst[j] = src[j].Char
		}
		lines = append(lines, dst)
	}

	for _, line := range t.buffer {
		row := make([]rune, len(line))
		for j, cell := range line {
			row[j] = cell.Char
		}
		lines = append(lines, row)
	}

	cursorX = t.cursorX
	cursorY = scrollLen + t.cursorY
	return lines, cursorX, cursorY
}

// SnapshotCells returns scrollback + active buffer with styles and absolute cursor position.
// Cursor Y is relative to the returned lines.
func (t *Terminal) SnapshotCells() (lines [][]Cell, cursorX, cursorY int) {
	t.mu.Lock()
	defer t.mu.Unlock()

	scrollLen := len(t.scrollback)
	lines = make([][]Cell, 0, scrollLen+len(t.buffer))

	for i := range scrollLen {
		idx := i
		if scrollLen > 0 {
			idx = (t.scrollStart + i) % scrollLen
		}
		src := t.scrollback[idx]
		dst := make([]Cell, len(src))
		copy(dst, src)
		lines = append(lines, dst)
	}

	for _, line := range t.buffer {
		row := make([]Cell, len(line))
		copy(row, line)
		lines = append(lines, row)
	}

	cursorX = t.cursorX
	cursorY = scrollLen + t.cursorY
	return lines, cursorX, cursorY
}

// GetBufferCells returns the entire buffer with styles.
// This is the preferred method for rendering with color support.
func (t *Terminal) GetBufferCells() [][]Cell {
	t.mu.Lock()
	defer t.mu.Unlock()
	// Return a copy to avoid race conditions
	lines := make([][]Cell, len(t.buffer))
	for i, line := range t.buffer {
		lines[i] = make([]Cell, len(line))
		copy(lines[i], line)
	}
	return lines
}

// CursorPos returns the cursor position.
func (t *Terminal) CursorPos() (x, y int) {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.cursorX, t.cursorY
}

// Resize resizes the terminal.
func (t *Terminal) Resize(rows, cols int) error {
	if err := t.pty.SetSize(rows, cols); err != nil {
		return err
	}

	// Send SIGWINCH to the child process to notify it of the size change
	if t.process != nil {
		t.process.Signal(syscall.SIGWINCH)
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	// Resize buffer
	clearCell := Cell{Char: ' ', Style: DefaultStyle()}
	newBuffer := make([][]Cell, rows)
	for i := range newBuffer {
		newBuffer[i] = make([]Cell, cols)
		for j := range newBuffer[i] {
			newBuffer[i][j] = clearCell
		}
		// Copy old content if available
		if i < len(t.buffer) {
			for j := 0; j < cols && j < len(t.buffer[i]); j++ {
				newBuffer[i][j] = t.buffer[i][j]
			}
		}
	}
	t.buffer = newBuffer
	t.width = cols
	t.height = rows

	// Reset scroll region to full screen on resize
	t.scrollTop = 0
	t.scrollBottom = rows - 1

	// Clamp cursor
	if t.cursorX >= cols {
		t.cursorX = cols - 1
	}
	if t.cursorY >= rows {
		t.cursorY = rows - 1
	}

	t.version.Add(1)
	return nil
}

// Close closes the terminal and kills the process.
func (t *Terminal) Close() error {
	t.closed = true
	t.version.Add(1)
	if t.process != nil {
		t.process.Kill()
		t.process.Wait()
	}
	return t.pty.Close()
}

// Version reports a monotonically increasing counter that changes whenever the
// terminal buffer is updated or resized.
func (t *Terminal) Version() uint64 {
	if t == nil {
		return 0
	}
	return t.version.Load()
}

// IsRunning returns true if the process is still running.
func (t *Terminal) IsRunning() bool {
	if t.process == nil || t.closed {
		return false
	}
	// Check if process is still running
	var ws syscall.WaitStatus
	pid, err := syscall.Wait4(t.process.Pid, &ws, syscall.WNOHANG, nil)
	if err != nil || pid != 0 {
		return false
	}
	return true
}

var ErrClosed = errors.New("terminal closed")
