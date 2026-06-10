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
	"flag"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"avyos.dev/lib/format"
)

var (
	flagOut     string
	flagNewline bool
	flagAppend  bool
)

func init() {
	flag.StringVar(&flagOut, "out", "stdout", "Output destination: stdout, stderr, or filename")
	flag.BoolVar(&flagNewline, "newline", true, "Add newline at end")
	flag.BoolVar(&flagAppend, "append", false, "Append to file instead of overwriting")
	flag.Usage = func() {
		fmt.Fprintln(os.Stderr, "write - Print or write formatted text")
		fmt.Fprintln(os.Stderr)
		fmt.Fprintln(os.Stderr, "Usage:")
		fmt.Fprintln(os.Stderr, "  write <format> [args...] [--out=stdout|stderr|file] [--newline] [--append]")
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

func run(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: write <format> [args...]")
	}

	formatStr := args[0]
	formatArgs := args[1:]

	convertedArgs := make([]any, len(formatArgs))
	for i, arg := range formatArgs {
		convertedArgs[i] = parseArg(arg)
	}

	output := fmt.Sprintf(formatStr, convertedArgs...)

	var writer io.Writer
	var file *os.File

	switch strings.ToLower(flagOut) {
	case "stdout", "":
		writer = os.Stdout
	case "stderr":
		writer = os.Stderr
	default:
		openFlags := os.O_CREATE | os.O_WRONLY
		if flagAppend {
			openFlags |= os.O_APPEND
		} else {
			openFlags |= os.O_TRUNC
		}
		var err error
		file, err = os.OpenFile(flagOut, openFlags, 0644)
		if err != nil {
			return fmt.Errorf("failed to open file: %w", err)
		}
		defer file.Close()
		writer = file
	}

	if flagNewline {
		fmt.Fprintln(writer, output)
	} else {
		fmt.Fprint(writer, output)
	}

	return nil
}

func parseArg(arg string) any {
	if i, err := strconv.ParseInt(arg, 10, 64); err == nil {
		return i
	}
	if f, err := strconv.ParseFloat(arg, 64); err == nil {
		return f
	}
	if b, err := strconv.ParseBool(arg); err == nil {
		return b
	}
	return arg
}
