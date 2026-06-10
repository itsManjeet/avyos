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
	"os"

	"avyos.dev/lib/format"
	"avyos.dev/lib/fs"
)

var (
	flagRecursive bool
	flagForce     bool
)

func init() {
	flag.BoolVar(&flagRecursive, "recursive", false, "Delete directories recursively")
	flag.BoolVar(&flagForce, "force", false, "Ignore nonexistent files")
	flag.Usage = func() {
		fmt.Fprintln(os.Stderr, "delete - Delete files or directories")
		fmt.Fprintln(os.Stderr)
		fmt.Fprintln(os.Stderr, "Usage:")
		fmt.Fprintln(os.Stderr, "  delete <path> [--recursive] [--force]")
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
		return fmt.Errorf("usage: delete <path>")
	}

	path := args[0]

	if !fs.Exists(path) {
		if flagForce {
			return nil
		}
		return fmt.Errorf("%s does not exist", path)
	}

	if err := fs.Remove(path, flagRecursive); err != nil {
		return err
	}

	format.Success("Removed %s", path)
	return nil
}
