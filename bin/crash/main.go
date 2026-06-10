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
	"runtime/debug"
	"syscall"

	"avyos.dev/lib/format"
)

func init() {
	flag.Usage = func() {
		fmt.Fprintln(os.Stderr, "crash - Trigger a controlled crash for testing")
		fmt.Fprintln(os.Stderr)
		fmt.Fprintln(os.Stderr, "Usage:")
		fmt.Fprintln(os.Stderr, "  crash <subcommand>")
		fmt.Fprintln(os.Stderr)
		fmt.Fprintln(os.Stderr, "Subcommands:")
		fmt.Fprintln(os.Stderr, "  segv   Terminate the process with SIGSEGV")
		fmt.Fprintln(os.Stderr, "  abrt   Terminate the process with SIGABRT")
		fmt.Fprintln(os.Stderr, "  panic  Trigger a Go panic with GOTRACEBACK=crash")
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

	commands := map[string]func() error{
		"segv":  crashSegv,
		"abrt":  crashAbrt,
		"panic": crashPanic,
	}

	if len(args) < 1 {
		flag.Usage()
		os.Exit(1)
	}

	cmd, ok := commands[args[0]]
	if !ok {
		format.Error("unknown subcommand: %s", args[0])
		os.Exit(1)
	}

	if err := cmd(); err != nil {
		format.Error("%s", err)
		os.Exit(1)
	}
}

func crashSegv() error {
	fmt.Fprintln(os.Stderr, "triggering SIGSEGV")
	return syscall.Kill(os.Getpid(), syscall.SIGSEGV)
}

func crashAbrt() error {
	fmt.Fprintln(os.Stderr, "triggering SIGABRT")
	return syscall.Kill(os.Getpid(), syscall.SIGABRT)
}

func crashPanic() error {
	_ = os.Setenv("GOTRACEBACK", "crash")
	debug.SetTraceback("crash")
	panic("crash test requested")
}
