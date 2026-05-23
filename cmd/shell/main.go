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
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"avyos.dev/pkg/format"
	"avyos.dev/pkg/fs"
	"avyos.dev/pkg/identity"
	"avyos.dev/pkg/term"
)

var (
	editor   *term.LineEditor
	lastExit int
	aliases  = map[string]string{
		"ls":   "list",
		"cat":  "read",
		"echo": "write",
	}
	shellPath string
)

func main() {
	shellPath, _ = os.Readlink(fs.Resolve("process:self/exe"))

	fmt.Printf("Welcome to avyos Shell\n\n")
	fmt.Println("Use `help` command to print help.")
	fmt.Println()

	// Initialize line editor
	editor = term.NewLineEditor(getPrompt())

	// Main loop
	for {
		editor.SetPrompt(getPrompt())

		line, err := editor.ReadLine()
		if err != nil {
			if err.Error() == "EOF" {
				fmt.Println("\nexit")
				break
			}
			continue
		}

		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// Expand aliases
		line = expandAlias(line)

		// Execute command
		lastExit = execute(line)
	}
}

func getPrompt() string {

	id, err := identity.LookupByID(os.Getuid())
	if err != nil {
		return "> "
	}
	cwd, _ := os.Getwd()
	home := id.Home
	if home != "" && strings.HasPrefix(cwd, home) {
		cwd = "~" + cwd[len(home):]
	}
	username := id.Name
	hostname, _ := os.Hostname()
	if hostname == "" {
		hostname = "avyos"
	}

	promptChar := "$"
	if os.Getuid() == 0 {
		promptChar = "#"
	}

	return fmt.Sprintf("(%s) %s %s %s ",
		format.Color(format.Green+format.Bold, hostname),
		format.Color(format.Bold, username),
		format.Color(format.Blue+format.Bold, cwd),
		format.Color(format.White, promptChar))
}

func expandAlias(line string) string {
	parts := strings.Fields(line)
	if len(parts) == 0 {
		return line
	}

	if alias, ok := aliases[parts[0]]; ok {
		parts[0] = alias
		return strings.Join(parts, " ")
	}

	return line
}

func execute(line string) int {
	// Handle pipes
	if strings.Contains(line, "|") {
		return executePipeline(line)
	}

	// Handle redirects and execute single command
	return executeSingle(line)
}

func executePipeline(line string) int {
	parts := strings.Split(line, "|")
	if len(parts) < 2 {
		return executeSingle(line)
	}

	var cmds []*exec.Cmd
	for _, part := range parts {
		part = strings.TrimSpace(part)
		cmd := parseCommand(part)
		if cmd == nil {
			return 1
		}
		cmds = append(cmds, cmd)
	}

	// Connect pipes
	for i := 0; i < len(cmds)-1; i++ {
		pipe, err := cmds[i].StdoutPipe()
		if err != nil {
			format.Error("pipe error: %s", err)
			return 1
		}
		cmds[i+1].Stdin = pipe
	}

	// First command reads from stdin
	cmds[0].Stdin = os.Stdin

	// Last command writes to stdout
	cmds[len(cmds)-1].Stdout = os.Stdout
	cmds[len(cmds)-1].Stderr = os.Stderr

	// Start all commands
	for _, cmd := range cmds {
		if err := cmd.Start(); err != nil {
			format.Error("%s", err)
			return 1
		}
	}

	// Wait for all commands
	var lastErr error
	for _, cmd := range cmds {
		if err := cmd.Wait(); err != nil {
			lastErr = err
		}
	}

	if lastErr != nil {
		return 1
	}
	return 0
}

func executeSingle(line string) int {
	// Parse redirections
	var stdout, stderr io.Writer = os.Stdout, os.Stderr
	var stdin io.Reader = os.Stdin
	var closeFiles []io.Closer

	// Handle output redirection
	if idx := strings.LastIndex(line, ">>"); idx != -1 {
		filename := strings.TrimSpace(line[idx+2:])
		line = strings.TrimSpace(line[:idx])
		file, err := os.OpenFile(filename, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0644)
		if err != nil {
			format.Error("cannot open %s: %s", filename, err)
			return 1
		}
		stdout = file
		closeFiles = append(closeFiles, file)
	} else if idx := strings.LastIndex(line, ">"); idx != -1 {
		filename := strings.TrimSpace(line[idx+1:])
		line = strings.TrimSpace(line[:idx])
		file, err := os.Create(filename)
		if err != nil {
			format.Error("cannot create %s: %s", filename, err)
			return 1
		}
		stdout = file
		closeFiles = append(closeFiles, file)
	}

	// Handle input redirection
	if idx := strings.LastIndex(line, "<"); idx != -1 {
		filename := strings.TrimSpace(line[idx+1:])
		line = strings.TrimSpace(line[:idx])
		file, err := os.Open(filename)
		if err != nil {
			format.Error("cannot open %s: %s", filename, err)
			return 1
		}
		stdin = file
		closeFiles = append(closeFiles, file)
	}

	defer func() {
		for _, f := range closeFiles {
			f.Close()
		}
	}()

	// Expand environment variables
	line = os.ExpandEnv(line)

	// Parse command
	args := parseArgs(line)
	if len(args) == 0 {
		return 0
	}

	// Check for built-in commands
	switch args[0] {
	case "exit":
		os.Exit(0)
	case "cd":
		return builtinCd(args[1:])
	case "export":
		return builtinExport(args[1:])
	case "unset":
		return builtinUnset(args[1:])
	case "alias":
		return builtinAlias(args[1:])
	case "history":
		return builtinHistory()
	case "help":
		return builtinHelp()
	case "pwd":
		cwd, _ := os.Getwd()
		fmt.Fprintln(stdout, cwd)
		return 0
	case "echo":
		fmt.Fprintln(stdout, strings.Join(args[1:], " "))
		return 0
	case "true":
		return 0
	case "false":
		return 1
	case "clear":
		fmt.Printf("\033[H\033[J")
		return 0
	}

	// Execute external command
	cmd := exec.Command(fs.Resolve("%s", args[0]), args[1:]...)
	cmd.Stdin = stdin
	cmd.Stdout = stdout
	cmd.Stderr = stderr

	if err := cmd.Run(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return exitErr.ExitCode()
		}
		format.Error("%s", err)
		return 127
	}

	return 0
}

func parseCommand(line string) *exec.Cmd {
	line = os.ExpandEnv(line)
	args := parseArgs(line)
	if len(args) == 0 {
		return nil
	}
	return exec.Command(args[0], args[1:]...)
}

func parseArgs(line string) []string {
	var args []string
	var current strings.Builder
	inQuote := false
	quoteChar := rune(0)

	for _, r := range line {
		switch {
		case r == '"' || r == '\'':
			if inQuote && r == quoteChar {
				inQuote = false
				quoteChar = 0
			} else if !inQuote {
				inQuote = true
				quoteChar = r
			} else {
				current.WriteRune(r)
			}
		case r == ' ' && !inQuote:
			if current.Len() > 0 {
				args = append(args, current.String())
				current.Reset()
			}
		default:
			current.WriteRune(r)
		}
	}

	if current.Len() > 0 {
		args = append(args, current.String())
	}

	return args
}

// Built-in commands

func builtinCd(args []string) int {
	var dir string
	if len(args) == 0 {
		dir = os.Getenv("HOME")
		if dir == "" {
			dir = "/"
		}
	} else {
		dir = args[0]
	}

	// Handle ~
	if strings.HasPrefix(dir, "~") {
		home := os.Getenv("HOME")
		if home != "" {
			dir = home + dir[1:]
		}
	}

	if err := os.Chdir(dir); err != nil {
		format.Error("%s", err)
		return 1
	}

	// Update PWD
	cwd, _ := os.Getwd()
	os.Setenv("PWD", cwd)

	return 0
}

func builtinExport(args []string) int {
	if len(args) == 0 {
		// Print all environment variables
		for _, env := range os.Environ() {
			fmt.Println(env)
		}
		return 0
	}

	for _, arg := range args {
		if before, after, ok := strings.Cut(arg, "="); ok {
			key := before
			value := after
			os.Setenv(key, value)
		} else {
			// Just export existing variable
			value := os.Getenv(arg)
			fmt.Printf("%s=%s\n", arg, value)
		}
	}

	return 0
}

func builtinUnset(args []string) int {
	for _, arg := range args {
		os.Unsetenv(arg)
	}
	return 0
}

func builtinAlias(args []string) int {
	if len(args) == 0 {
		for name, value := range aliases {
			fmt.Printf("alias %s='%s'\n", name, value)
		}
		return 0
	}

	for _, arg := range args {
		if before, after, ok := strings.Cut(arg, "="); ok {
			name := before
			value := strings.Trim(after, "'\"")
			aliases[name] = value
		} else {
			if value, ok := aliases[arg]; ok {
				fmt.Printf("alias %s='%s'\n", arg, value)
			}
		}
	}

	return 0
}

func builtinHistory() int {
	for i, line := range editor.GetHistory() {
		fmt.Printf("%4d  %s\n", i+1, line)
	}
	return 0
}

func builtinHelp() int {
	fmt.Println("avyos Shell - Built-in Commands")
	fmt.Println()
	fmt.Println("  cd [dir]         Change directory")
	fmt.Println("  pwd              Print working directory")
	fmt.Println("  echo [text]      Print text")
	fmt.Println("  export [VAR=val] Set environment variable")
	fmt.Println("  unset VAR        Unset environment variable")
	fmt.Println("  alias [name=cmd] Set or show aliases")
	fmt.Println("  history          Show command history")
	fmt.Println("  exit             Exit the shell")
	fmt.Println("  help             Show this help")
	fmt.Println()
	fmt.Println("Available Commands:")
	listAvailableCommands()
	return 0
}

func listAvailableCommands() {
	// Get PATH
	pathEnv := os.Getenv("PATH")
	if pathEnv == "" {
		return
	}

	seen := make(map[string]bool)
	paths := strings.SplitSeq(pathEnv, ":")

	for dir := range paths {
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}

		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}

			name := entry.Name()
			if seen[name] {
				continue
			}

			// Check if executable
			path := filepath.Join(dir, name)
			info, err := os.Stat(path)
			if err != nil {
				continue
			}

			if info.Mode()&0111 != 0 {
				seen[name] = true
			}
		}
	}

	// Print in columns
	var names []string
	for name := range seen {
		names = append(names, name)
	}
	sort.Slice(names, func(i, j int) bool {
		left := strings.ToLower(names[i])
		right := strings.ToLower(names[j])
		if left == right {
			return names[i] < names[j]
		}
		return left < right
	})

	// Simple column output
	cols := 4
	for i, name := range names {
		fmt.Printf("  %-15s", name)
		if (i+1)%cols == 0 {
			fmt.Println()
		}
	}
	if len(names)%cols != 0 {
		fmt.Println()
	}
}
