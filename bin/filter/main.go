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
	"path/filepath"
	"regexp"
	"strings"

	"avyos.dev/lib/format"
)

var (
	flagIgnoreCase bool
	flagLineNumber bool
	flagCount      bool
	flagInvert     bool
	flagRecursive  bool
	flagFilesOnly  bool
	flagQuiet      bool
	flagBefore     int64
	flagAfter      int64
	flagContext    int64
	flagFixed      bool
)

func init() {
	flag.BoolVar(&flagIgnoreCase, "ignore-case", false, "Case-insensitive search")
	flag.BoolVar(&flagLineNumber, "line-number", false, "Show line numbers")
	flag.BoolVar(&flagCount, "count", false, "Only show match count per file")
	flag.BoolVar(&flagInvert, "invert", false, "Invert match (show non-matching lines)")
	flag.BoolVar(&flagRecursive, "recursive", false, "Search directories recursively")
	flag.BoolVar(&flagFilesOnly, "files-only", false, "Only show filenames with matches")
	flag.BoolVar(&flagQuiet, "quiet", false, "Quiet mode (exit 0 if match found)")
	flag.Int64Var(&flagBefore, "before", 0, "Show N lines before match")
	flag.Int64Var(&flagAfter, "after", 0, "Show N lines after match")
	flag.Int64Var(&flagContext, "context", 0, "Show N lines before and after match")
	flag.BoolVar(&flagFixed, "fixed", false, "Treat pattern as fixed string, not regex")
	flag.Usage = func() {
		fmt.Fprintln(os.Stderr, "filter - Search file content (grep-like)")
		fmt.Fprintln(os.Stderr)
		fmt.Fprintln(os.Stderr, "Usage:")
		fmt.Fprintln(os.Stderr, "  filter <pattern> [file...] [options]")
		fmt.Fprintln(os.Stderr)
		fmt.Fprintln(os.Stderr, "Subcommands:")
		fmt.Fprintln(os.Stderr, "  (none)")
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
	if err := run(flag.Args()); err != nil {
		format.Error("%s", err)
		os.Exit(1)
	}
}

type options struct {
	ignoreCase bool
	lineNumber bool
	count      bool
	invert     bool
	recursive  bool
	filesOnly  bool
	quiet      bool
	before     int
	after      int
	fixed      bool
}

func run(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: filter <pattern> [file...]")
	}

	pattern := args[0]
	files := args[1:]

	opts := options{
		ignoreCase: flagIgnoreCase,
		lineNumber: flagLineNumber,
		count:      flagCount,
		invert:     flagInvert,
		recursive:  flagRecursive,
		filesOnly:  flagFilesOnly,
		quiet:      flagQuiet,
		before:     int(flagBefore),
		after:      int(flagAfter),
		fixed:      flagFixed,
	}

	if ctx := int(flagContext); ctx > 0 {
		opts.before = ctx
		opts.after = ctx
	}

	if opts.fixed {
		pattern = regexp.QuoteMeta(pattern)
	}
	if opts.ignoreCase {
		pattern = "(?i)" + pattern
	}

	re, err := regexp.Compile(pattern)
	if err != nil {
		return fmt.Errorf("invalid pattern: %w", err)
	}

	if len(files) == 0 {
		_, err := searchReader(os.Stdin, "", re, opts)
		return err
	}

	var allFiles []string
	for _, f := range files {
		if opts.recursive {
			expanded, err := expandRecursive(f)
			if err != nil {
				format.Warn("cannot access %s: %s", f, err)
				continue
			}
			allFiles = append(allFiles, expanded...)
		} else {
			allFiles = append(allFiles, f)
		}
	}

	multiFile := len(allFiles) > 1
	matchFound := false

	for _, path := range allFiles {
		file, err := os.Open(path)
		if err != nil {
			if !opts.quiet {
				format.Warn("cannot open %s: %s", path, err)
			}
			continue
		}

		filename := ""
		if multiFile {
			filename = path
		}

		found, err := searchReader(file, filename, re, opts)
		file.Close()

		if err != nil && !opts.quiet {
			format.Warn("error reading %s: %s", path, err)
		}

		if found {
			matchFound = true
			if opts.quiet {
				os.Exit(0)
			}
		}
	}

	if opts.quiet && !matchFound {
		os.Exit(1)
	}

	return nil
}

func searchReader(r *os.File, filename string, re *regexp.Regexp, opts options) (bool, error) {
	scanner := bufio.NewScanner(r)
	lineNum := 0
	matchCount := 0
	matchFound := false

	var lines []string
	if opts.before > 0 || opts.after > 0 {
		for scanner.Scan() {
			lines = append(lines, scanner.Text())
		}
		if err := scanner.Err(); err != nil {
			return false, err
		}

		lastMatchLine := -1
		for i, line := range lines {
			lineNum := i + 1
			matches := re.MatchString(line)
			if opts.invert {
				matches = !matches
			}

			if matches {
				matchFound = true
				matchCount++

				if opts.quiet {
					return true, nil
				}
				if opts.filesOnly {
					if filename != "" {
						fmt.Println(filename)
					}
					return true, nil
				}
				if !opts.count {
					start := max(i-opts.before, 0)
					if start > lastMatchLine+1 && lastMatchLine >= 0 {
						fmt.Println("--")
					}
					for j := start; j < i; j++ {
						if j > lastMatchLine {
							printLine(filename, j+1, lines[j], opts.lineNumber)
						}
					}

					printMatchLine(filename, lineNum, line, re, opts)

					end := i + opts.after
					if end >= len(lines) {
						end = len(lines) - 1
					}
					for j := i + 1; j <= end; j++ {
						printLine(filename, j+1, lines[j], opts.lineNumber)
					}
					lastMatchLine = end
				}
			}
		}
	} else {
		for scanner.Scan() {
			lineNum++
			line := scanner.Text()

			matches := re.MatchString(line)
			if opts.invert {
				matches = !matches
			}

			if matches {
				matchFound = true
				matchCount++

				if opts.quiet {
					return true, nil
				}
				if opts.filesOnly {
					if filename != "" {
						fmt.Println(filename)
					}
					return true, nil
				}
				if !opts.count {
					printMatchLine(filename, lineNum, line, re, opts)
				}
			}
		}

		if err := scanner.Err(); err != nil {
			return matchFound, err
		}
	}

	if opts.count && matchCount > 0 {
		if filename != "" {
			fmt.Printf("%s:%d\n", filename, matchCount)
		} else {
			fmt.Printf("%d\n", matchCount)
		}
	}

	return matchFound, nil
}

func printLine(filename string, lineNum int, line string, showLineNum bool) {
	var parts []string
	if filename != "" {
		parts = append(parts, format.Color(format.Magenta, filename))
	}
	if showLineNum {
		parts = append(parts, format.Color(format.Green, fmt.Sprintf("%d", lineNum)))
	}
	if len(parts) > 0 {
		fmt.Printf("%s-%s\n", strings.Join(parts, ":"), line)
	} else {
		fmt.Println(line)
	}
}

func printMatchLine(filename string, lineNum int, line string, re *regexp.Regexp, opts options) {
	var parts []string

	if filename != "" {
		parts = append(parts, format.Color(format.Magenta, filename))
	}
	if opts.lineNumber {
		parts = append(parts, format.Color(format.Green, fmt.Sprintf("%d", lineNum)))
	}

	displayLine := line
	if !opts.invert {
		displayLine = re.ReplaceAllStringFunc(line, func(s string) string {
			return format.Color(format.Red+format.Bold, s)
		})
	}

	if len(parts) > 0 {
		fmt.Printf("%s:%s\n", strings.Join(parts, ":"), displayLine)
	} else {
		fmt.Println(displayLine)
	}
}

func expandRecursive(path string) ([]string, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}

	if !info.IsDir() {
		return []string{path}, nil
	}

	var files []string
	err = filepath.Walk(path, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() {
			if strings.HasPrefix(info.Name(), ".") && p != path {
				return filepath.SkipDir
			}
			return nil
		}

		ext := strings.ToLower(filepath.Ext(p))
		binaryExts := map[string]bool{
			".exe": true, ".bin": true, ".o": true, ".so": true,
			".a": true, ".dll": true, ".dylib": true,
			".png": true, ".jpg": true, ".jpeg": true, ".gif": true,
			".zip": true, ".tar": true, ".gz": true, ".bz2": true,
			".pdf": true, ".doc": true, ".docx": true,
		}
		if binaryExts[ext] {
			return nil
		}

		files = append(files, p)
		return nil
	})

	return files, err
}
