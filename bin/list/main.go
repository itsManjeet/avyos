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
	"sort"
	"strings"
	"time"

	"avyos.dev/lib/format"
)

var (
	flagAll   bool
	flagLong  bool
	flagHuman bool
)

func init() {
	flag.BoolVar(&flagAll, "all", false, "Show hidden files")
	flag.BoolVar(&flagLong, "long", false, "Show detailed information")
	flag.BoolVar(&flagHuman, "human", true, "Show human-readable sizes")
	flag.Usage = func() {
		fmt.Fprintln(os.Stderr, "list - List directory contents")
		fmt.Fprintln(os.Stderr)
		fmt.Fprintln(os.Stderr, "Usage:")
		fmt.Fprintln(os.Stderr, "  list [path] [--all] [--long] [--human]")
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
	path := "."
	if len(args) > 0 {
		path = args[0]
	}

	infos, err := listDir(path)
	if err != nil {
		return err
	}

	sort.Slice(infos, func(i, j int) bool {
		return strings.ToLower(infos[i].Name) < strings.ToLower(infos[j].Name)
	})

	if flagLong {
		table := format.NewTable("Permissions", "UID", "GID", "Size", "Modified", "Name")
		for _, info := range infos {
			if !flagAll && strings.HasPrefix(info.Name, ".") {
				continue
			}

			size := fmt.Sprintf("%d", info.Size)
			if flagHuman {
				size = format.Size(info.Size)
			}

			modTime := time.Unix(info.ModTime, 0).Format("2006-01-02 15:04")
			name := info.Name
			if info.IsDir {
				name = format.Color(format.Blue+format.Bold, name+"/")
			} else if info.Mode&0111 != 0 {
				name = format.Color(format.Green, name)
			} else if info.IsLink {
				target := ""
				if info.Target != "" {
					target = " -> " + info.Target
				}
				name = format.Color(format.Cyan, name+target)
			}

			table.AddRow(
				permString(info.Mode),
				fmt.Sprintf("%d", info.UID),
				fmt.Sprintf("%d", info.GID),
				size,
				modTime,
				name,
			)
		}
		table.Print()
		return nil
	}

	cols := 80
	var names []string
	for _, info := range infos {
		if !flagAll && strings.HasPrefix(info.Name, ".") {
			continue
		}
		name := info.Name
		if info.IsDir {
			name = format.Color(format.Blue+format.Bold, name+"/")
		} else if info.Mode&0111 != 0 {
			name = format.Color(format.Green, name)
		} else if info.IsLink {
			name = format.Color(format.Cyan, name)
		}
		names = append(names, name)
	}

	maxLen := 0
	for _, n := range names {
		if len(n) > maxLen {
			maxLen = len(n)
		}
	}
	colWidth := maxLen + 2
	numCols := max(cols/colWidth, 1)

	for i, name := range names {
		fmt.Printf("%-*s", colWidth, name)
		if (i+1)%numCols == 0 {
			fmt.Println()
		}
	}
	if len(names)%numCols != 0 {
		fmt.Println()
	}

	return nil
}
