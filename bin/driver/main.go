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
	"strings"
	"syscall"
	"unsafe"

	"avyos.dev/lib/format"
)

const (
	modulesPath = "/cache/kernel/processes/modules"
	sysModPath  = "/cache/kernel/sysfs/module"
	driversPath = "/drivers"
)

type moduleEntry struct {
	name    string
	size    string
	used    string
	usedBy  string
	state   string
	address string
}

func init() {
	flag.Usage = func() {
		fmt.Fprintln(os.Stderr, "driver - Kernel module management")
		fmt.Fprintln(os.Stderr)
		fmt.Fprintln(os.Stderr, "Usage:")
		fmt.Fprintln(os.Stderr, "  driver <subcommand> [options] [args]")
		fmt.Fprintln(os.Stderr)
		fmt.Fprintln(os.Stderr, "Subcommands:")
		fmt.Fprintln(os.Stderr, "  list    List loaded modules")
		fmt.Fprintln(os.Stderr, "  load    Load a module file or module name")
		fmt.Fprintln(os.Stderr, "  unload  Unload a loaded module")
		fmt.Fprintln(os.Stderr, "  info    Show module details from sysfs")
		fmt.Fprintln(os.Stderr, "  deps    Show module dependency information")
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
		"list":   cmdList,
		"load":   cmdLoad,
		"unload": cmdUnload,
		"info":   cmdInfo,
		"deps":   cmdDeps,
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
	showAll := fs.Bool("all", false, "Show all modules including built-in")
	sortBy := fs.String("sort", "name", "Sort by: name, size, used")
	fs.SetOutput(os.Stderr)
	if err := fs.Parse(args); err != nil {
		return err
	}

	modules, err := parseModules()
	if err != nil {
		return fmt.Errorf("failed to read modules: %w", err)
	}

	switch *sortBy {
	case "size":
		sort.Slice(modules, func(i, j int) bool {
			return modules[i].size > modules[j].size
		})
	case "used":
		sort.Slice(modules, func(i, j int) bool {
			return modules[i].used > modules[j].used
		})
	default:
		sort.Slice(modules, func(i, j int) bool {
			return modules[i].name < modules[j].name
		})
	}

	table := format.NewTable("Module", "Size", "Used", "By", "State")

	for _, m := range modules {
		table.AddRow(m.name, m.size, m.used, m.usedBy, m.state)
	}

	table.Print()

	if *showAll {
		builtins, err := listBuiltinModules()
		if err == nil && len(builtins) > 0 {
			fmt.Printf("\nBuilt-in modules:\n")
			for _, name := range builtins {
				fmt.Printf("  %s\n", name)
			}
		}
	}

	return nil
}

func cmdLoad(args []string) error {
	fs := flag.NewFlagSet("load", flag.ContinueOnError)
	params := fs.String("params", "", "Module parameters (key=value,...)")
	force := fs.Bool("force", false, "Force load even if version mismatch")
	fs.SetOutput(os.Stderr)
	if err := fs.Parse(args); err != nil {
		return err
	}
	args = fs.Args()

	if len(args) < 1 {
		return fmt.Errorf("usage: driver load <module.ko|name>")
	}

	target := args[0]

	modulePath := target
	if !strings.Contains(target, "/") && !strings.HasSuffix(target, ".ko") {
		resolved, err := findModule(target)
		if err != nil {
			return err
		}
		modulePath = resolved
	}

	if err := loadModule(modulePath, *params, *force); err != nil {
		return fmt.Errorf("failed to load %s: %w", filepath.Base(modulePath), err)
	}

	name := strings.TrimSuffix(filepath.Base(modulePath), ".ko")
	name = strings.TrimSuffix(name, ".ko.xz")
	name = strings.TrimSuffix(name, ".ko.zst")
	format.Success("Loaded module %s", name)
	return nil
}

func cmdUnload(args []string) error {
	fs := flag.NewFlagSet("unload", flag.ContinueOnError)
	force := fs.Bool("force", false, "Force unload")
	fs.SetOutput(os.Stderr)
	if err := fs.Parse(args); err != nil {
		return err
	}
	args = fs.Args()

	if len(args) < 1 {
		return fmt.Errorf("usage: driver unload <name>")
	}

	name := args[0]

	if err := unloadModule(name, *force); err != nil {
		return fmt.Errorf("failed to unload %s: %w", name, err)
	}

	format.Success("Unloaded module %s", name)
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
		return fmt.Errorf("usage: driver info <name>")
	}

	name := args[0]
	modDir := filepath.Join(sysModPath, name)

	if _, err := os.Stat(modDir); err != nil {
		return fmt.Errorf("module %s not found", name)
	}

	fmt.Printf("Module:      %s\n", name)

	if v, err := readSysFile(filepath.Join(modDir, "version")); err == nil {
		fmt.Printf("Version:     %s\n", v)
	}

	if v, err := readSysFile(filepath.Join(modDir, "srcversion")); err == nil {
		fmt.Printf("Source:      %s\n", v)
	}

	if v, err := readSysFile(filepath.Join(modDir, "refcnt")); err == nil {
		fmt.Printf("Refcount:    %s\n", v)
	}

	if v, err := readSysFile(filepath.Join(modDir, "coresize")); err == nil {
		fmt.Printf("Core size:   %s\n", v)
	}

	if v, err := readSysFile(filepath.Join(modDir, "initsize")); err == nil {
		fmt.Printf("Init size:   %s\n", v)
	}

	if v, err := readSysFile(filepath.Join(modDir, "taint")); err == nil && v != "" {
		fmt.Printf("Taint:       %s\n", v)
	}

	// Show holders (modules depending on this one)
	holdersDir := filepath.Join(modDir, "holders")
	if entries, err := os.ReadDir(holdersDir); err == nil && len(entries) > 0 {
		var holders []string
		for _, e := range entries {
			holders = append(holders, e.Name())
		}
		fmt.Printf("Holders:     %s\n", strings.Join(holders, ", "))
	}

	// Show parameters
	paramsDir := filepath.Join(modDir, "parameters")
	if entries, err := os.ReadDir(paramsDir); err == nil && len(entries) > 0 {
		fmt.Printf("\nParameters:\n")
		for _, e := range entries {
			if v, err := readSysFile(filepath.Join(paramsDir, e.Name())); err == nil {
				fmt.Printf("  %-20s = %s\n", e.Name(), v)
			}
		}
	}

	return nil
}

func cmdDeps(args []string) error {
	fs := flag.NewFlagSet("deps", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	if err := fs.Parse(args); err != nil {
		return err
	}
	args = fs.Args()

	if len(args) < 1 {
		return fmt.Errorf("usage: driver deps <name>")
	}

	name := args[0]

	modules, err := parseModules()
	if err != nil {
		return fmt.Errorf("failed to read modules: %w", err)
	}

	var found *moduleEntry
	for i := range modules {
		if modules[i].name == name {
			found = &modules[i]
			break
		}
	}

	if found == nil {
		return fmt.Errorf("module %s is not loaded", name)
	}

	tree := format.NewTree(name)

	if found.usedBy != "" && found.usedBy != "-" {
		depNames := strings.SplitSeq(found.usedBy, ",")
		for dep := range depNames {
			dep = strings.TrimSpace(dep)
			if dep != "" {
				tree.AddChildLabel(dep)
			}
		}
	}

	// Show reverse: what does this module depend on
	holdersDir := filepath.Join(sysModPath, name, "holders")
	if entries, err := os.ReadDir(holdersDir); err == nil {
		for _, e := range entries {
			tree.AddChildLabel(format.Color(format.Cyan, e.Name()+" (holder)"))
		}
	}

	if len(tree.String()) <= len(name)+1 {
		fmt.Printf("%s has no dependencies\n", name)
		return nil
	}

	tree.Print()
	return nil
}

// parseModules reads /proc/modules (mapped to /cache/kernel/processes/modules).
func parseModules() ([]moduleEntry, error) {
	file, err := os.Open(modulesPath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var modules []moduleEntry
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) < 4 {
			continue
		}

		entry := moduleEntry{
			name: fields[0],
			size: fields[1],
			used: fields[2],
		}

		if len(fields) > 3 {
			entry.usedBy = strings.TrimSuffix(fields[3], ",")
		}
		if len(fields) > 4 {
			entry.state = fields[4]
		}
		if len(fields) > 5 {
			entry.address = fields[5]
		}

		modules = append(modules, entry)
	}

	return modules, scanner.Err()
}

// listBuiltinModules reads the list of built-in modules from /drivers/.
func listBuiltinModules() ([]string, error) {
	entries, err := os.ReadDir(driversPath)
	if err != nil {
		return nil, err
	}

	var names []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if before, ok := strings.CutSuffix(name, ".ko"); ok {
			names = append(names, before)
		}
	}

	sort.Strings(names)
	return names, nil
}

// findModule searches for a module .ko file by name in /drivers/.
func findModule(name string) (string, error) {
	name = strings.ReplaceAll(name, "-", "_")

	// Direct match
	path := filepath.Join(driversPath, name+".ko")
	if _, err := os.Stat(path); err == nil {
		return path, nil
	}

	// Scan directory for match
	entries, err := os.ReadDir(driversPath)
	if err != nil {
		return "", fmt.Errorf("cannot read %s: %w", driversPath, err)
	}

	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		modName := strings.TrimSuffix(e.Name(), ".ko")
		modName = strings.ReplaceAll(modName, "-", "_")
		if modName == name {
			return filepath.Join(driversPath, e.Name()), nil
		}
	}

	return "", fmt.Errorf("module %s not found in %s", name, driversPath)
}

// loadModule loads a kernel module using finit_module or init_module syscall.
func loadModule(path, params string, force bool) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()

	paramsBytes, err := syscall.BytePtrFromString(params)
	if err != nil {
		return err
	}

	var flags int
	if force {
		flags |= 0x1 // MODULE_INIT_IGNORE_MODVERSIONS
		flags |= 0x2 // MODULE_INIT_IGNORE_VERMAGIC
	}

	// Try finit_module first (takes fd, preferred)
	_, _, errno := syscall.Syscall(
		sysFinitModule,
		file.Fd(),
		uintptr(unsafe.Pointer(paramsBytes)),
		uintptr(flags),
	)
	if errno == 0 {
		return nil
	}

	// Fallback to init_module (load entire file into memory)
	if errno == syscall.ENOSYS {
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}

		_, _, errno = syscall.Syscall(
			syscall.SYS_INIT_MODULE,
			uintptr(unsafe.Pointer(&data[0])),
			uintptr(len(data)),
			uintptr(unsafe.Pointer(paramsBytes)),
		)
		if errno == 0 {
			return nil
		}
		return errno
	}

	return errno
}

// unloadModule removes a kernel module using delete_module syscall.
func unloadModule(name string, force bool) error {
	nameBytes, err := syscall.BytePtrFromString(name)
	if err != nil {
		return err
	}

	var flags int
	flags |= 0x0001 // O_NONBLOCK
	if force {
		flags |= 0x0004 // O_TRUNC = force removal
	}

	_, _, errno := syscall.Syscall(
		syscall.SYS_DELETE_MODULE,
		uintptr(unsafe.Pointer(nameBytes)),
		uintptr(flags),
		0,
	)
	if errno != 0 {
		return errno
	}

	return nil
}

// readSysFile reads a sysfs file and returns its trimmed content.
func readSysFile(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(data)), nil
}
