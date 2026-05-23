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
	"strings"

	"avyos.dev/lib/format"
)

var (
	flagDepth int
	flagAll   bool
)

func init() {
	flag.IntVar(&flagDepth, "depth", 3, "Maximum depth to display")
	flag.BoolVar(&flagAll, "all", false, "Show hidden files")
	flag.Usage = func() {
		fmt.Fprintln(os.Stderr, "tree - Show directory tree view")
		fmt.Fprintln(os.Stderr)
		fmt.Fprintln(os.Stderr, "Usage:")
		fmt.Fprintln(os.Stderr, "  tree [path] [--depth=N] [--all]")
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

	tree, err := buildTreeRoot(path, flagDepth)
	if err != nil {
		return err
	}

	printTree(tree, "", true, true, flagAll)
	return nil
}

func printTree(entry *treeEntry, prefix string, last bool, isRoot bool, showAll bool) {
	if !showAll && !isRoot && strings.HasPrefix(entry.Info.Name, ".") {
		return
	}

	connector := "├── "
	if last {
		connector = "└── "
	}

	name := entry.Info.Name
	if entry.Info.IsDir {
		name = format.Color(format.Blue+format.Bold, name+"/")
	}

	if isRoot {
		fmt.Println(name)
	} else {
		fmt.Println(prefix + connector + name)
	}

	childPrefix := prefix
	if !isRoot {
		if last {
			childPrefix += "    "
		} else {
			childPrefix += "│   "
		}
	}

	children := entry.Children
	if !showAll {
		var filtered []*treeEntry
		for _, c := range children {
			if !strings.HasPrefix(c.Info.Name, ".") {
				filtered = append(filtered, c)
			}
		}
		children = filtered
	}

	for i, child := range children {
		printTree(child, childPrefix, i == len(children)-1, false, showAll)
	}
}
