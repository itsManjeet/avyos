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
	"sort"
	"strconv"
	"strings"
	"syscall"

	"avyos.dev/lib/format"
)

// ProcessInfo holds information about a process.
type ProcessInfo struct {
	PID     int
	PPID    int
	Name    string
	State   string
	Memory  int64 // RSS in KB
	Command string
	User    string
}

func init() {
	flag.Usage = func() {
		fmt.Fprintln(os.Stderr, "process - Process management utilities")
		fmt.Fprintln(os.Stderr)
		fmt.Fprintln(os.Stderr, "Usage:")
		fmt.Fprintln(os.Stderr, "  process <subcommand> [options] [args]")
		fmt.Fprintln(os.Stderr)
		fmt.Fprintln(os.Stderr, "Subcommands:")
		fmt.Fprintln(os.Stderr, "  list     List running processes")
		fmt.Fprintln(os.Stderr, "  info     Show detailed information for a PID")
		fmt.Fprintln(os.Stderr, "  kill     Send a signal to a process")
		fmt.Fprintln(os.Stderr, "  tree     Show process tree")
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
		"list": cmdList,
		"info": cmdInfo,
		"kill": cmdKill,
		"tree": cmdTree,
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

	if err := cmd(args[1:]); err != nil {
		format.Error("%s", err)
		os.Exit(1)
	}
}

func cmdList(args []string) error {
	fs := flag.NewFlagSet("list", flag.ContinueOnError)
	showAll := fs.Bool("all", false, "Show all processes")
	showTree := fs.Bool("tree", false, "Show process tree")
	fs.SetOutput(os.Stderr)
	if err := fs.Parse(args); err != nil {
		return err
	}
	args = fs.Args()

	if *showTree {
		return cmdTree(args)
	}

	procs, err := listProcesses()
	if err != nil {
		return err
	}

	// Filter to user processes if not --all
	myUID := os.Getuid()
	var filtered []*ProcessInfo
	for _, p := range procs {
		if *showAll || isOwnedByUser(p.PID, myUID) {
			filtered = append(filtered, p)
		}
	}

	// Sort by PID
	sort.Slice(filtered, func(i, j int) bool {
		return filtered[i].PID < filtered[j].PID
	})

	table := format.NewTable("PID", "PPID", "State", "Memory", "Name")
	for _, p := range filtered {
		mem := format.Size(p.Memory * 1024)
		table.AddRow(
			fmt.Sprintf("%d", p.PID),
			fmt.Sprintf("%d", p.PPID),
			p.State,
			mem,
			p.Name,
		)
	}
	table.Print()

	return nil
}

func cmdInfo(args []string) error {
	fs := flag.NewFlagSet("info", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	if err := fs.Parse(args); err != nil {
		return err
	}
	args = fs.Args()

	if len(args) < 1 {
		return fmt.Errorf("usage: process info <pid>")
	}

	pid, err := strconv.Atoi(args[0])
	if err != nil {
		return fmt.Errorf("invalid PID: %s", args[0])
	}

	proc, err := getProcessInfo(pid)
	if err != nil {
		return err
	}

	fmt.Printf("PID:         %d\n", proc.PID)
	fmt.Printf("Parent PID:  %d\n", proc.PPID)
	fmt.Printf("Name:        %s\n", proc.Name)
	fmt.Printf("State:       %s\n", stateDescription(proc.State))
	fmt.Printf("Memory:      %s\n", format.Size(proc.Memory*1024))
	fmt.Printf("Command:     %s\n", proc.Command)

	// Show file descriptors
	fdPath := fmt.Sprintf("/cache/kernel/processes/%d/fd", pid)
	if fds, err := os.ReadDir(fdPath); err == nil {
		fmt.Printf("Open files:  %d\n", len(fds))
	}

	// Show threads
	taskPath := fmt.Sprintf("/cache/kernel/processes/%d/task", pid)
	if threads, err := os.ReadDir(taskPath); err == nil {
		fmt.Printf("Threads:     %d\n", len(threads))
	}

	return nil
}

func cmdKill(args []string) error {
	fs := flag.NewFlagSet("kill", flag.ContinueOnError)
	sigName := fs.String("signal", "TERM", "Signal to send")
	force := fs.Bool("force", false, "Send SIGKILL instead of SIGTERM")
	fs.SetOutput(os.Stderr)
	if err := fs.Parse(args); err != nil {
		return err
	}
	args = fs.Args()

	if len(args) < 1 {
		return fmt.Errorf("usage: process kill <pid>")
	}

	pid, err := strconv.Atoi(args[0])
	if err != nil {
		return fmt.Errorf("invalid PID: %s", args[0])
	}

	var sig syscall.Signal
	if *force {
		sig = syscall.SIGKILL
	} else {
		sig = parseSignal(*sigName)
	}

	process, err := os.FindProcess(pid)
	if err != nil {
		return fmt.Errorf("process not found: %d", pid)
	}

	if err := process.Signal(sig); err != nil {
		return fmt.Errorf("failed to send signal: %w", err)
	}

	format.Success("Sent %s to process %d", *sigName, pid)
	return nil
}

func cmdTree(args []string) error {
	fs := flag.NewFlagSet("tree", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	if err := fs.Parse(args); err != nil {
		return err
	}
	args = fs.Args()

	rootPID := 1
	if len(args) > 0 {
		pid, err := strconv.Atoi(args[0])
		if err != nil {
			return fmt.Errorf("invalid PID: %s", args[0])
		}
		rootPID = pid
	}

	procs, err := listProcesses()
	if err != nil {
		return err
	}

	// Build process map
	procMap := make(map[int]*ProcessInfo)
	children := make(map[int][]*ProcessInfo)
	for _, p := range procs {
		procMap[p.PID] = p
		children[p.PPID] = append(children[p.PPID], p)
	}

	// Print tree
	if root, ok := procMap[rootPID]; ok {
		printProcessTree(root, children, "", true)
	} else {
		// Start from all root processes (PPID=0 or PPID=1)
		for _, p := range procs {
			if p.PPID == 0 || (p.PPID == 1 && p.PID != 1) {
				printProcessTree(p, children, "", true)
			}
		}
	}

	return nil
}

func printProcessTree(proc *ProcessInfo, children map[int][]*ProcessInfo, prefix string, last bool) {
	connector := "├── "
	if last {
		connector = "└── "
	}

	if prefix == "" {
		fmt.Printf("%s (%d)\n", proc.Name, proc.PID)
	} else {
		fmt.Printf("%s%s%s (%d)\n", prefix, connector, proc.Name, proc.PID)
	}

	childPrefix := prefix
	if prefix != "" {
		if last {
			childPrefix += "    "
		} else {
			childPrefix += "│   "
		}
	}

	kids := children[proc.PID]
	for i, child := range kids {
		printProcessTree(child, children, childPrefix, i == len(kids)-1)
	}
}

func listProcesses() ([]*ProcessInfo, error) {
	entries, err := os.ReadDir("/cache/kernel/processes")
	if err != nil {
		return nil, err
	}

	var procs []*ProcessInfo
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		pid, err := strconv.Atoi(entry.Name())
		if err != nil {
			continue
		}

		proc, err := getProcessInfo(pid)
		if err != nil {
			continue
		}

		procs = append(procs, proc)
	}

	return procs, nil
}

func getProcessInfo(pid int) (*ProcessInfo, error) {
	statPath := fmt.Sprintf("/cache/kernel/processes/%d/stat", pid)
	data, err := os.ReadFile(statPath)
	if err != nil {
		return nil, err
	}

	// Parse stat file
	// Format: pid (comm) state ppid ...
	stat := string(data)

	// Find command name (between parentheses)
	start := strings.Index(stat, "(")
	end := strings.LastIndex(stat, ")")
	if start == -1 || end == -1 {
		return nil, fmt.Errorf("invalid stat format")
	}

	name := stat[start+1 : end]
	rest := strings.Fields(stat[end+2:])

	if len(rest) < 22 {
		return nil, fmt.Errorf("invalid stat format")
	}

	ppid, _ := strconv.Atoi(rest[1])
	rss, _ := strconv.ParseInt(rest[21], 10, 64)

	// Get command line
	cmdPath := fmt.Sprintf("/cache/kernel/processes/%d/cmdline", pid)
	cmdData, _ := os.ReadFile(cmdPath)
	cmd := strings.ReplaceAll(string(cmdData), "\x00", " ")
	cmd = strings.TrimSpace(cmd)
	if cmd == "" {
		cmd = "[" + name + "]"
	}

	return &ProcessInfo{
		PID:     pid,
		PPID:    ppid,
		Name:    name,
		State:   rest[0],
		Memory:  rss * 4, // Pages to KB (assuming 4KB pages)
		Command: cmd,
	}, nil
}

func isOwnedByUser(pid, uid int) bool {
	path := fmt.Sprintf("/cache/kernel/processes/%d", pid)
	fi, err := os.Stat(path)
	if err != nil {
		return false
	}
	stat, ok := fi.Sys().(*syscall.Stat_t)
	if !ok {
		return false
	}
	return int(stat.Uid) == uid
}

func stateDescription(state string) string {
	switch state {
	case "R":
		return "Running"
	case "S":
		return "Sleeping"
	case "D":
		return "Disk sleep"
	case "Z":
		return "Zombie"
	case "T":
		return "Stopped"
	case "t":
		return "Tracing stop"
	case "X":
		return "Dead"
	case "I":
		return "Idle"
	default:
		return state
	}
}

func parseSignal(name string) syscall.Signal {
	name = strings.ToUpper(name)
	name = strings.TrimPrefix(name, "SIG")

	switch name {
	case "HUP":
		return syscall.SIGHUP
	case "INT":
		return syscall.SIGINT
	case "QUIT":
		return syscall.SIGQUIT
	case "ILL":
		return syscall.SIGILL
	case "TRAP":
		return syscall.SIGTRAP
	case "ABRT":
		return syscall.SIGABRT
	case "BUS":
		return syscall.SIGBUS
	case "FPE":
		return syscall.SIGFPE
	case "KILL":
		return syscall.SIGKILL
	case "USR1":
		return syscall.SIGUSR1
	case "SEGV":
		return syscall.SIGSEGV
	case "USR2":
		return syscall.SIGUSR2
	case "PIPE":
		return syscall.SIGPIPE
	case "ALRM":
		return syscall.SIGALRM
	case "TERM":
		return syscall.SIGTERM
	case "STKFLT":
		return syscall.Signal(16)
	case "CHLD":
		return syscall.SIGCHLD
	case "CONT":
		return syscall.SIGCONT
	case "STOP":
		return syscall.SIGSTOP
	case "TSTP":
		return syscall.SIGTSTP
	case "TTIN":
		return syscall.SIGTTIN
	case "TTOU":
		return syscall.SIGTTOU
	case "URG":
		return syscall.SIGURG
	case "XCPU":
		return syscall.SIGXCPU
	case "XFSZ":
		return syscall.SIGXFSZ
	case "VTALRM":
		return syscall.SIGVTALRM
	case "PROF":
		return syscall.SIGPROF
	case "WINCH":
		return syscall.SIGWINCH
	case "IO", "POLL":
		return syscall.SIGIO
	case "PWR":
		return syscall.Signal(30)
	case "SYS":
		return syscall.SIGSYS
	default:
		// Try to parse as number
		if n, err := strconv.Atoi(name); err == nil {
			return syscall.Signal(n)
		}
		return syscall.SIGTERM
	}
}

// readFile reads a file from /proc
func readFile(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	var lines []string
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	return strings.Join(lines, "\n"), scanner.Err()
}

// Ensure filepath is used to avoid import error
var _ = filepath.Join
