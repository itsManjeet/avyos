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
	"path/filepath"
	"strconv"
	"syscall"
	"time"

	"avyos.dev/lib/format"
	"os/user"
)

func init() {
	flag.Usage = func() {
		fmt.Fprintln(os.Stderr, "info - Show filesystem metadata for a path")
		fmt.Fprintln(os.Stderr)
		fmt.Fprintln(os.Stderr, "Usage:")
		fmt.Fprintln(os.Stderr, "  info <path>")
		fmt.Fprintln(os.Stderr, "  info perm  <path> <octal>")
		fmt.Fprintln(os.Stderr, "  info owner <path> <name|uid>")
		fmt.Fprintln(os.Stderr, "  info group <path> <name|gid>")
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
		return fmt.Errorf("usage: info <path>")
	}

	switch args[0] {
	case "perm":
		if len(args) != 3 {
			return fmt.Errorf("usage: info perm <path> <octal>")
		}
		mode, err := strconv.ParseUint(args[2], 8, 32)
		if err != nil {
			return fmt.Errorf("invalid octal permission %q: %w", args[2], err)
		}
		if err := os.Chmod(args[1], os.FileMode(mode)); err != nil {
			return err
		}
		format.Success("permissions of %s changed to %04o", args[1], mode)
		return nil

	case "owner":
		if len(args) != 3 {
			return fmt.Errorf("usage: info owner <path> <name|uid>")
		}
		uid, err := resolveUID(args[2])
		if err != nil {
			return err
		}
		info, err := os.Lstat(args[1])
		if err != nil {
			return err
		}
		gid := -1
		if si, ok := info.Sys().(*syscall.Stat_t); ok {
			gid = int(si.Gid)
		}
		if err := os.Lchown(args[1], uid, gid); err != nil {
			return err
		}
		format.Success("owner of %s changed to %d", args[1], uid)
		return nil

	case "group":
		if len(args) != 3 {
			return fmt.Errorf("usage: info group <path> <name|gid>")
		}
		gid, err := resolveGID(args[2])
		if err != nil {
			return err
		}
		info, err := os.Lstat(args[1])
		if err != nil {
			return err
		}
		uid := -1
		if si, ok := info.Sys().(*syscall.Stat_t); ok {
			uid = int(si.Uid)
		}
		if err := os.Lchown(args[1], uid, gid); err != nil {
			return err
		}
		format.Success("group of %s changed to %d", args[1], gid)
		return nil
	}

	path := args[0]
	info, err := infoFor(path)
	if err != nil {
		return err
	}

	absPath, _ := filepath.Abs(path)

	owner, _ := user.LookupId(strconv.Itoa(int(info.UID)))
	group, _ := user.LookupGroupId(strconv.Itoa(int(info.GID)))

	fmt.Printf("Name:        %s\n", info.Name)
	fmt.Printf("Path:        %s\n", absPath)
	fmt.Printf("Type:        %s\n", fileType(info))
	fmt.Printf("Size:        %s (%d bytes)\n", format.Size(info.Size), info.Size)
	fmt.Printf("Permissions: %s\n", permString(info.Mode))
	fmt.Printf("Modified:    %s\n", time.Unix(info.ModTime, 0).Format("2006-01-02 15:04:05"))

	if owner != nil {
		fmt.Printf("Owner:       %s (%d)\n", owner.Username, info.UID)
	}
	if group != nil {
		fmt.Printf("Group:       %s (%d)\n", group.Name, info.GID)
	}

	if info.IsLink && info.Target != "" {
		fmt.Printf("Target:      %s\n", info.Target)
	}

	return nil
}

func resolveUID(s string) (int, error) {
	if n, err := strconv.Atoi(s); err == nil {
		return n, nil
	}
	id, err := user.Lookup(s)
	if err != nil {
		return 0, fmt.Errorf("unknown user %q: %w", s, err)
	}
	uid, err := strconv.Atoi(id.Uid)
	if err != nil {
		return 0, fmt.Errorf("invalid uid for user %q: %w", s, err)
	}
	return uid, nil
}

func resolveGID(s string) (int, error) {
	if n, err := strconv.Atoi(s); err == nil {
		return n, nil
	}
	group, err := user.LookupGroup(s)
	if err != nil {
		return 0, fmt.Errorf("unknown group %q: %w", s, err)
	}
	gid, err := strconv.Atoi(group.Gid)
	if err != nil {
		return 0, fmt.Errorf("invalid gid for group %q: %w", s, err)
	}
	return gid, nil
}

func fileType(info *fileInfo) string {
	if info.IsLink {
		return "symbolic link"
	}
	if info.IsDir {
		return "directory"
	}
	return "regular file"
}
