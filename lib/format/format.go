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

package format

import (
	"fmt"
	"os"
	"strings"
)

// ANSI color codes
const (
	Reset     = "\033[0m"
	Bold      = "\033[1m"
	Dim       = "\033[2m"
	Underline = "\033[4m"

	Black   = "\033[30m"
	Red     = "\033[31m"
	Green   = "\033[32m"
	Yellow  = "\033[33m"
	Blue    = "\033[34m"
	Magenta = "\033[35m"
	Cyan    = "\033[36m"
	White   = "\033[37m"

	BgBlack   = "\033[40m"
	BgRed     = "\033[41m"
	BgGreen   = "\033[42m"
	BgYellow  = "\033[43m"
	BgBlue    = "\033[44m"
	BgMagenta = "\033[45m"
	BgCyan    = "\033[46m"
	BgWhite   = "\033[47m"
)

// colorEnabled tracks if color output is enabled
var colorEnabled = true

func init() {
	// Disable colors if not a terminal or NO_COLOR is set
	if os.Getenv("NO_COLOR") != "" {
		colorEnabled = false
	}
	// Check if stdout is a terminal
	if fi, _ := os.Stdout.Stat(); (fi.Mode() & os.ModeCharDevice) == 0 {
		colorEnabled = false
	}
}

// DisableColor disables color output.
func DisableColor() {
	colorEnabled = false
}

// EnableColor enables color output.
func EnableColor() {
	colorEnabled = true
}

// Color applies a color to text if colors are enabled.
func Color(color, text string) string {
	if !colorEnabled {
		return text
	}
	return color + text + Reset
}

// Size formats a byte count as a human-readable string.
func Size(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

// Percent formats a percentage.
func Percent(value, total float64) string {
	if total == 0 {
		return "0%"
	}
	return fmt.Sprintf("%.1f%%", (value/total)*100)
}

// Table represents a formatted table.
type Table struct {
	headers []string
	rows    [][]string
	widths  []int
}

// NewTable creates a new table with the given headers.
func NewTable(headers ...string) *Table {
	t := &Table{
		headers: headers,
		rows:    make([][]string, 0),
		widths:  make([]int, len(headers)),
	}
	for i, h := range headers {
		t.widths[i] = len(h)
	}
	return t
}

// AddRow adds a row to the table.
func (t *Table) AddRow(cols ...string) {
	// Pad row if needed
	for len(cols) < len(t.headers) {
		cols = append(cols, "")
	}
	t.rows = append(t.rows, cols)
	for i, c := range cols {
		if i < len(t.widths) && len(c) > t.widths[i] {
			t.widths[i] = len(c)
		}
	}
}

// String returns the formatted table as a string.
func (t *Table) String() string {
	var sb strings.Builder

	// Print headers
	for i, h := range t.headers {
		if i > 0 {
			sb.WriteString("  ")
		}
		sb.WriteString(Color(Bold, padRight(h, t.widths[i])))
	}
	sb.WriteString("\n")

	// Print rows
	for _, row := range t.rows {
		for i, c := range row {
			if i >= len(t.widths) {
				break
			}
			if i > 0 {
				sb.WriteString("  ")
			}
			sb.WriteString(padRight(c, t.widths[i]))
		}
		sb.WriteString("\n")
	}

	return sb.String()
}

// Print prints the table to stdout.
func (t *Table) Print() {
	fmt.Print(t.String())
}

func padRight(s string, width int) string {
	if len(s) >= width {
		return s
	}
	return s + strings.Repeat(" ", width-len(s))
}

// ProgressBar represents a progress bar.
type ProgressBar struct {
	total   int64
	current int64
	width   int
	label   string
}

// NewProgressBar creates a new progress bar.
func NewProgressBar(total int64, width int) *ProgressBar {
	if width <= 0 {
		width = 40
	}
	return &ProgressBar{
		total: total,
		width: width,
	}
}

// SetLabel sets the label for the progress bar.
func (p *ProgressBar) SetLabel(label string) {
	p.label = label
}

// Update updates the progress bar.
func (p *ProgressBar) Update(current int64) {
	p.current = current
	p.render()
}

// Increment increments the progress by n.
func (p *ProgressBar) Increment(n int64) {
	p.current += n
	if p.current > p.total {
		p.current = p.total
	}
	p.render()
}

// Finish completes the progress bar.
func (p *ProgressBar) Finish() {
	p.current = p.total
	p.render()
	fmt.Println()
}

func (p *ProgressBar) render() {
	percent := float64(p.current) / float64(p.total)
	filled := int(percent * float64(p.width))

	bar := strings.Repeat("=", filled)
	if filled < p.width {
		bar += ">"
		bar += strings.Repeat(" ", p.width-filled-1)
	}

	label := p.label
	if label != "" {
		label += " "
	}

	fmt.Printf("\r%s[%s] %s", label, bar, Percent(float64(p.current), float64(p.total)))
}

// Tree represents a tree structure for display.
type Tree struct {
	label    string
	children []*Tree
}

// NewTree creates a new tree node.
func NewTree(label string) *Tree {
	return &Tree{
		label:    label,
		children: make([]*Tree, 0),
	}
}

// AddChild adds a child node to the tree.
func (t *Tree) AddChild(child *Tree) *Tree {
	t.children = append(t.children, child)
	return t
}

// AddChildLabel adds a child with the given label.
func (t *Tree) AddChildLabel(label string) *Tree {
	child := NewTree(label)
	t.children = append(t.children, child)
	return child
}

// String returns the tree as a formatted string.
func (t *Tree) String() string {
	var sb strings.Builder
	t.render(&sb, "", true)
	return sb.String()
}

func (t *Tree) render(sb *strings.Builder, prefix string, last bool) {
	connector := "├── "
	if last {
		connector = "└── "
	}

	if prefix == "" {
		sb.WriteString(t.label + "\n")
	} else {
		sb.WriteString(prefix + connector + t.label + "\n")
	}

	childPrefix := prefix
	if prefix != "" {
		if last {
			childPrefix += "    "
		} else {
			childPrefix += "│   "
		}
	}

	for i, child := range t.children {
		child.render(sb, childPrefix, i == len(t.children)-1)
	}
}

// Print prints the tree to stdout.
func (t *Tree) Print() {
	fmt.Print(t.String())
}

// Error formats an error message in red.
func Error(format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	fmt.Fprintln(os.Stderr, Color(Red, "Error: "+msg))
}

// Warn formats a warning message in yellow.
func Warn(format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	fmt.Fprintln(os.Stderr, Color(Yellow, "Warning: "+msg))
}

// Success formats a success message in green.
func Success(format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	fmt.Println(Color(Green, msg))
}

// Info formats an info message in cyan.
func Info(format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	fmt.Println(Color(Cyan, msg))
}
