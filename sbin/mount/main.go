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
	"slices"
	"strings"
	"syscall"

	"avyos.dev/lib/format"
)

func init() {
	flag.Usage = func() {
		fmt.Fprintln(os.Stderr, "mount - Filesystem mount management")
		fmt.Fprintln(os.Stderr)
		fmt.Fprintln(os.Stderr, "Usage:")
		fmt.Fprintln(os.Stderr, "  mount <subcommand> [options] [args]")
		fmt.Fprintln(os.Stderr)
		fmt.Fprintln(os.Stderr, "Subcommands:")
		fmt.Fprintln(os.Stderr, "  list    List mounted filesystems")
		fmt.Fprintln(os.Stderr, "  add     Mount a filesystem")
		fmt.Fprintln(os.Stderr, "  remove  Unmount a filesystem")
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
		"add":    cmdAdd,
		"remove": cmdRemove,
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

type mountEntry struct {
	device     string
	mountPoint string
	fsType     string
	options    string
}

func cmdList(args []string) error {
	fs := flag.NewFlagSet("list", flag.ContinueOnError)
	showAll := fs.Bool("all", false, "Show all filesystems including virtual")
	fs.SetOutput(os.Stderr)
	if err := fs.Parse(args); err != nil {
		return err
	}

	mounts, err := parseMounts()
	if err != nil {
		return err
	}

	table := format.NewTable("Device", "Mount Point", "Type", "Options")
	for _, m := range mounts {
		if !*showAll && isVirtualFS(m.fsType) {
			continue
		}
		table.AddRow(m.device, m.mountPoint, m.fsType, truncate(m.options, 40))
	}
	table.Print()
	return nil
}

func cmdAdd(args []string) error {
	fs := flag.NewFlagSet("add", flag.ContinueOnError)
	fsType := fs.String("type", "", "Filesystem type")
	options := fs.String("options", "", "Mount options")
	readonly := fs.Bool("readonly", false, "Mount read-only")
	fs.SetOutput(os.Stderr)
	if err := fs.Parse(args); err != nil {
		return err
	}
	args = fs.Args()

	if len(args) < 2 {
		return fmt.Errorf("usage: mount add <device> <mountpoint>")
	}

	device := args[0]
	mountPoint := args[1]

	var mountFlags uintptr
	if *readonly {
		mountFlags |= syscall.MS_RDONLY
	}

	if *options != "" {
		for opt := range strings.SplitSeq(*options, ",") {
			switch opt {
			case "ro":
				mountFlags |= syscall.MS_RDONLY
			case "noexec":
				mountFlags |= syscall.MS_NOEXEC
			case "nosuid":
				mountFlags |= syscall.MS_NOSUID
			case "nodev":
				mountFlags |= syscall.MS_NODEV
			case "noatime":
				mountFlags |= syscall.MS_NOATIME
			case "nodiratime":
				mountFlags |= syscall.MS_NODIRATIME
			case "bind":
				mountFlags |= syscall.MS_BIND
			case "remount":
				mountFlags |= syscall.MS_REMOUNT
			}
		}
	}

	if _, err := os.Stat(mountPoint); os.IsNotExist(err) {
		if err := os.MkdirAll(mountPoint, 0755); err != nil {
			return fmt.Errorf("failed to create mount point: %w", err)
		}
	}

	if err := syscall.Mount(device, mountPoint, *fsType, mountFlags, *options); err != nil {
		return fmt.Errorf("mount failed: %w", err)
	}

	format.Success("Mounted %s on %s", device, mountPoint)
	return nil
}

func cmdRemove(args []string) error {
	fs := flag.NewFlagSet("remove", flag.ContinueOnError)
	force := fs.Bool("force", false, "Force unmount")
	lazy := fs.Bool("lazy", false, "Lazy unmount")
	fs.SetOutput(os.Stderr)
	if err := fs.Parse(args); err != nil {
		return err
	}
	args = fs.Args()

	if len(args) < 1 {
		return fmt.Errorf("usage: mount remove <mountpoint>")
	}

	umountFlags := 0
	if *force {
		umountFlags |= syscall.MNT_FORCE
	}
	if *lazy {
		umountFlags |= syscall.MNT_DETACH
	}

	if err := syscall.Unmount(args[0], umountFlags); err != nil {
		return fmt.Errorf("unmount failed: %w", err)
	}

	format.Success("Unmounted %s", args[0])
	return nil
}

func parseMounts() ([]mountEntry, error) {
	file, err := os.Open("/proc/mounts")
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var mounts []mountEntry
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) < 4 {
			continue
		}
		mounts = append(mounts, mountEntry{
			device:     fields[0],
			mountPoint: fields[1],
			fsType:     fields[2],
			options:    fields[3],
		})
	}

	return mounts, scanner.Err()
}

func isVirtualFS(fsType string) bool {
	virtual := []string{
		"proc", "sysfs", "devtmpfs", "devpts", "tmpfs",
		"cgroup", "cgroup2", "pstore", "debugfs", "securityfs",
		"hugetlbfs", "mqueue", "fusectl", "configfs", "binfmt_misc",
		"autofs", "efivarfs", "tracefs", "bpf",
	}
	return slices.Contains(virtual, fsType)
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-3] + "..."
}
