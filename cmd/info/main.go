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

	"avyos.dev/pkg/format"
	"avyos.dev/pkg/fs"
	"avyos.dev/pkg/identity"
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
	info, err := fs.Info(path)
	if err != nil {
		return err
	}

	absPath, _ := filepath.Abs(path)

	id, _ := identity.LookupByID(int(info.UID))
	cap, _ := identity.LookupCapabilityByID(int(info.GID))

	fmt.Printf("Name:        %s\n", info.Name)
	fmt.Printf("Path:        %s\n", absPath)
	fmt.Printf("Type:        %s\n", fileType(info))
	fmt.Printf("Size:        %s (%d bytes)\n", format.Size(info.Size), info.Size)
	fmt.Printf("Permissions: %s\n", fs.PermString(info.Mode))
	fmt.Printf("Modified:    %s\n", time.Unix(info.ModTime, 0).Format("2006-01-02 15:04:05"))

	if id != nil {
		fmt.Printf("Owner:       %s (%d)\n", id.Name, info.UID)
	}
	if cap != nil {
		fmt.Printf("Group:       %s (%d)\n", cap.Name, info.GID)
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
	id, err := identity.LookupByName(s)
	if err != nil {
		return 0, fmt.Errorf("unknown user %q: %w", s, err)
	}
	return id.ID, nil
}

func resolveGID(s string) (int, error) {
	if n, err := strconv.Atoi(s); err == nil {
		return n, nil
	}
	cap, err := identity.LookupCapabilityByName(s)
	if err != nil {
		return 0, fmt.Errorf("unknown group %q: %w", s, err)
	}
	return cap.ID, nil
}

func fileType(info *fs.FileInfo) string {
	if info.IsLink {
		return "symbolic link"
	}
	if info.IsDir {
		return "directory"
	}
	return "regular file"
}
