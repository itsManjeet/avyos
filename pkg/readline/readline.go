package readline

import (
	"bufio"
	"fmt"
	"os"
	"syscall"
	"unsafe"
)

// Terminal state storage
type termState struct {
	termios syscall.Termios
}

// Readline represents the main readline instance
type Readline struct {
	prompt  string
	history []string
	histIdx int
	reader  *bufio.Reader
	state   *termState
}

// New creates a new Readline instance
func NewReader(prompt string) *Readline {
	return &Readline{
		prompt:  prompt,
		history: make([]string, 0),
		histIdx: 0,
		reader:  bufio.NewReader(os.Stdin),
	}
}

// setRawMode enables raw terminal mode
func (rl *Readline) setRawMode() error {
	fd := int(os.Stdin.Fd())
	var term syscall.Termios

	if _, _, err := syscall.Syscall(syscall.SYS_IOCTL, uintptr(fd),
		syscall.TCGETS, uintptr(unsafe.Pointer(&term))); err != 0 {
		return err
	}

	rl.state = &termState{termios: term}

	// Set raw mode
	raw := term
	raw.Iflag &^= syscall.IGNBRK | syscall.BRKINT | syscall.PARMRK |
		syscall.ISTRIP | syscall.INLCR | syscall.IGNCR | syscall.ICRNL | syscall.IXON
	raw.Oflag &^= syscall.OPOST
	raw.Lflag &^= syscall.ECHO | syscall.ECHONL | syscall.ICANON |
		syscall.ISIG | syscall.IEXTEN
	raw.Cflag &^= syscall.CSIZE | syscall.PARENB
	raw.Cflag |= syscall.CS8
	raw.Cc[syscall.VMIN] = 1
	raw.Cc[syscall.VTIME] = 0

	if _, _, err := syscall.Syscall(syscall.SYS_IOCTL, uintptr(fd),
		syscall.TCSETS, uintptr(unsafe.Pointer(&raw))); err != 0 {
		return err
	}

	return nil
}

// restoreMode restores the original terminal mode
func (rl *Readline) restoreMode() error {
	if rl.state == nil {
		return nil
	}

	fd := int(os.Stdin.Fd())
	if _, _, err := syscall.Syscall(syscall.SYS_IOCTL, uintptr(fd),
		syscall.TCSETS, uintptr(unsafe.Pointer(&rl.state.termios))); err != 0 {
		return err
	}

	return nil
}

// Readline reads a line with editing capabilities
func (rl *Readline) Readline() (string, error) {
	if err := rl.setRawMode(); err != nil {
		return "", err
	}
	defer rl.restoreMode()

	// Print initial prompt
	os.Stdout.Write([]byte(rl.prompt))

	var line []rune
	cursor := 0
	rl.histIdx = len(rl.history)

	for {
		char, _, err := rl.reader.ReadRune()
		if err != nil {
			return "", err
		}

		// Handle escape sequences
		if char == 27 { // ESC
			next1, _, _ := rl.reader.ReadRune()
			if next1 == '[' {
				next2, _, _ := rl.reader.ReadRune()

				switch next2 {
				case 'A': // Up arrow
					if rl.histIdx > 0 {
						rl.histIdx--
						line = []rune(rl.history[rl.histIdx])
						cursor = len(line)
						rl.refreshLine(line, cursor)
					}
					continue
				case 'B': // Down arrow
					if rl.histIdx < len(rl.history)-1 {
						rl.histIdx++
						line = []rune(rl.history[rl.histIdx])
						cursor = len(line)
						rl.refreshLine(line, cursor)
					} else if rl.histIdx == len(rl.history)-1 {
						rl.histIdx++
						line = []rune{}
						cursor = 0
						rl.refreshLine(line, cursor)
					}
					continue
				case 'C': // Right arrow
					if cursor < len(line) {
						cursor++
						fmt.Print("\033[C")
					}
					continue
				case 'D': // Left arrow
					if cursor > 0 {
						cursor--
						fmt.Print("\033[D")
					}
					continue
				case '3': // Delete key
					rl.reader.ReadRune() // consume '~'
					if cursor < len(line) {
						line = append(line[:cursor], line[cursor+1:]...)
						rl.refreshLine(line, cursor)
					}
					continue
				}
			}
			continue
		}

		switch char {
		case 13, 10: // Enter
			os.Stdout.Write([]byte("\r\n"))
			result := string(line)
			if len(result) > 0 {
				rl.AddHistory(result)
			}
			return result, nil

		case 127, 8: // Backspace
			if cursor > 0 {
				line = append(line[:cursor-1], line[cursor:]...)
				cursor--
				rl.refreshLine(line, cursor)
			}

		case 3: // Ctrl+C
			fmt.Println("^C")
			rl.restoreMode()
			os.Exit(0)

		case 4: // Ctrl+D (EOF)
			if len(line) == 0 {
				fmt.Println()
				return "", fmt.Errorf("EOF")
			}

		case 1: // Ctrl+A - beginning of line
			cursor = 0
			rl.refreshLine(line, cursor)

		case 5: // Ctrl+E - end of line
			cursor = len(line)
			rl.refreshLine(line, cursor)

		case 11: // Ctrl+K - kill to end of line
			line = line[:cursor]
			rl.refreshLine(line, cursor)

		case 12: // Ctrl+L - clear screen
			fmt.Print("\033[2J\033[H")
			fmt.Print(rl.prompt)
			rl.refreshLine(line, cursor)

		default:
			if char >= 32 && char < 127 || char >= 128 { // Printable characters
				line = append(line[:cursor], append([]rune{char}, line[cursor:]...)...)
				cursor++
				rl.refreshLine(line, cursor)
			}
		}
	}
}

// refreshLine redraws the current line
func (rl *Readline) refreshLine(line []rune, cursor int) {
	// Move to start of line, clear it
	os.Stdout.Write([]byte("\r\033[K"))
	// Print prompt and line
	os.Stdout.Write([]byte(rl.prompt))
	os.Stdout.Write([]byte(string(line)))

	// Move cursor to correct position
	if cursor < len(line) {
		// Move back to the cursor position
		moveback := len(line) - cursor
		if moveback > 0 {
			fmt.Printf("\033[%dD", moveback)
		}
	}
}

// AddHistory adds a line to history
func (rl *Readline) AddHistory(line string) {
	if len(line) > 0 && (len(rl.history) == 0 || rl.history[len(rl.history)-1] != line) {
		rl.history = append(rl.history, line)
	}
}

// GetHistory returns all history entries
func (rl *Readline) GetHistory() []string {
	return rl.history
}

// SetPrompt changes the prompt
func (rl *Readline) SetPrompt(prompt string) {
	rl.prompt = prompt
}
