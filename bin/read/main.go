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

package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"regexp"
	"strings"

	"avyos.dev/lib/format"
)

var flagNumbers bool

func init() {
	flag.BoolVar(&flagNumbers, "numbers", false, "Show line numbers")
	flag.Usage = func() {
		fmt.Fprintln(os.Stderr, "read - Read and inspect file content")
		fmt.Fprintln(os.Stderr)
		fmt.Fprintln(os.Stderr, "Usage:")
		fmt.Fprintln(os.Stderr, "  read <file> [--numbers]")
		fmt.Fprintln(os.Stderr, "  read top <file> [--lines=N]")
		fmt.Fprintln(os.Stderr, "  read last <file> [--lines=N] [--follow]")
		fmt.Fprintln(os.Stderr, "  read pattern <pattern> <file> [--ignorecase] [--numbers] [--count]")
		fmt.Fprintln(os.Stderr, "  read count <file>")
		fmt.Fprintln(os.Stderr)
		fmt.Fprintln(os.Stderr, "Subcommands:")
		fmt.Fprintln(os.Stderr, "  top      Show first N lines")
		fmt.Fprintln(os.Stderr, "  last     Show last N lines")
		fmt.Fprintln(os.Stderr, "  pattern  Search for pattern")
		fmt.Fprintln(os.Stderr, "  count    Count lines/words/chars")
		fmt.Fprintln(os.Stderr)
		flag.PrintDefaults()
		fmt.Fprintln(os.Stderr)
		fmt.Fprintln(os.Stderr, "Exit Codes:")
		fmt.Fprintln(os.Stderr, "  0  Success")
		fmt.Fprintln(os.Stderr, "  1  Runtime/command error")
		fmt.Fprintln(os.Stderr, "  2  Invalid flags/usage")
	}
}

func main() {
	flag.Parse()
	args := flag.Args()

	commands := map[string]func(args []string) error{
		"top":     cmdTop,
		"last":    cmdLast,
		"pattern": cmdPattern,
		"count":   cmdCount,
	}

	if len(args) > 0 {
		if cmd, ok := commands[args[0]]; ok {
			if err := cmd(args[1:]); err != nil {
				format.Error("%s", err)
				os.Exit(1)
			}
			return
		}
	}

	if err := run(args); err != nil {
		format.Error("%s", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: read <file>")
	}

	file, err := os.Open(args[0])
	if err != nil {
		return err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	lineNum := 1
	for scanner.Scan() {
		if flagNumbers {
			fmt.Printf("%4d  %s\n", lineNum, scanner.Text())
		} else {
			fmt.Println(scanner.Text())
		}
		lineNum++
	}

	return scanner.Err()
}

func cmdTop(args []string) error {
	fs := flag.NewFlagSet("top", flag.ContinueOnError)
	lines := fs.Int("lines", 10, "Number of lines to show")
	fs.SetOutput(os.Stderr)
	if err := fs.Parse(args); err != nil {
		return err
	}
	args = fs.Args()

	if len(args) < 1 {
		return fmt.Errorf("usage: read top <file>")
	}

	file, err := os.Open(args[0])
	if err != nil {
		return err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	count := 0
	for scanner.Scan() && count < *lines {
		fmt.Println(scanner.Text())
		count++
	}
	return scanner.Err()
}

func cmdLast(args []string) error {
	fs := flag.NewFlagSet("last", flag.ContinueOnError)
	lines := fs.Int("lines", 10, "Number of lines to show")
	follow := fs.Bool("follow", false, "Follow file changes")
	fs.SetOutput(os.Stderr)
	if err := fs.Parse(args); err != nil {
		return err
	}
	args = fs.Args()

	if len(args) < 1 {
		return fmt.Errorf("usage: read last <file>")
	}

	path := args[0]
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()

	var linesBuf []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		linesBuf = append(linesBuf, scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		return err
	}

	start := max(len(linesBuf)-*lines, 0)
	for _, line := range linesBuf[start:] {
		fmt.Println(line)
	}

	if *follow {
		fi, err := file.Stat()
		if err != nil {
			return err
		}
		offset := fi.Size()
		for {
			fi, err := os.Stat(path)
			if err != nil {
				continue
			}
			if fi.Size() > offset {
				f, err := os.Open(path)
				if err != nil {
					continue
				}
				_, _ = f.Seek(offset, 0)
				s := bufio.NewScanner(f)
				for s.Scan() {
					fmt.Println(s.Text())
				}
				fi, _ = f.Stat()
				offset = fi.Size()
				f.Close()
			}
		}
	}

	return nil
}

func cmdPattern(args []string) error {
	fs := flag.NewFlagSet("pattern", flag.ContinueOnError)
	ignoreCase := fs.Bool("ignorecase", false, "Case-insensitive search")
	showNumbers := fs.Bool("numbers", true, "Show line numbers")
	countOnly := fs.Bool("count", false, "Only show match count")
	fs.SetOutput(os.Stderr)
	if err := fs.Parse(args); err != nil {
		return err
	}
	args = fs.Args()

	if len(args) < 2 {
		return fmt.Errorf("usage: read pattern <pattern> <file>")
	}

	pattern := args[0]
	path := args[1]
	if *ignoreCase {
		pattern = "(?i)" + pattern
	}

	re, err := regexp.Compile(pattern)
	if err != nil {
		return fmt.Errorf("invalid pattern: %w", err)
	}

	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	lineNum := 0
	matchCount := 0

	for scanner.Scan() {
		lineNum++
		line := scanner.Text()
		if !re.MatchString(line) {
			continue
		}
		matchCount++
		if *countOnly {
			continue
		}

		highlighted := re.ReplaceAllStringFunc(line, func(s string) string {
			return format.Color(format.Red+format.Bold, s)
		})

		if *showNumbers {
			fmt.Printf("%s:%s\n", format.Color(format.Green, fmt.Sprintf("%d", lineNum)), highlighted)
		} else {
			fmt.Println(highlighted)
		}
	}

	if *countOnly {
		fmt.Printf("%d matches\n", matchCount)
	} else if matchCount == 0 {
		fmt.Println("No matches found")
	}

	return scanner.Err()
}

func cmdCount(args []string) error {
	fs := flag.NewFlagSet("count", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	if err := fs.Parse(args); err != nil {
		return err
	}
	args = fs.Args()

	if len(args) < 1 {
		return fmt.Errorf("usage: read count <file>")
	}

	file, err := os.Open(args[0])
	if err != nil {
		return err
	}
	defer file.Close()

	lines := 0
	words := 0
	chars := 0

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		lines++
		chars += len(line) + 1
		words += len(strings.Fields(line))
	}
	if err := scanner.Err(); err != nil {
		return err
	}

	table := format.NewTable("Metric", "Count")
	table.AddRow("Lines", fmt.Sprintf("%d", lines))
	table.AddRow("Words", fmt.Sprintf("%d", words))
	table.AddRow("Characters", fmt.Sprintf("%d", chars))
	table.Print()
	return nil
}
