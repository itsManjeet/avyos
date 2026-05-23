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
	"fmt"
	"os"
	"strings"
	"syscall"

	"avyos.dev/pkg/fs"
)

var (
	rootfs      string
	rootfsType  string = "btrfs"
	avyosfs     string
	avysofsType string = "squashfs"
	live        bool
)

func isInsideInitramfs() bool {
	return os.Args[0] == "/init"
}

func ensureRealRootfs() {
	if !isInsideInitramfs() {
		return
	}

	// Mount essential filesystems
	mountEssentialFilesystems()

	// Parser kernel cmdline args
	parseKernelFlags()

	// no rootfs specified
	if rootfs == "" {
		panic("no root device specified")
	}
	safeMount(rootfs, "/rootfs", rootfsType, "", 0)

	if avyosfs == "" {
		if !fs.Exists("/rootfs/avyos") {
			panic("avyos not found")
		}
	} else {
		safeMount(avyosfs, "/rootfs/avyos", avysofsType, "", syscall.MS_RDONLY)
	}

	for _, fs := range []string{
		fs.Resolve("process:"),
		fs.Resolve("sysfs:"),
		fs.Resolve("device:"),
		fs.Resolve("shared:"),
		fs.Resolve("system:"),
	} {
		os.MkdirAll("/rootfs/"+fs, 0755)
		syscall.Mount("/"+fs, "/rootfs/"+fs, "", syscall.MS_MOVE, "")
	}

	syscall.Chdir("/rootfs")
	syscall.Chroot("/rootfs")

	if err := syscall.Exec("/avyos/cmd/init", []string{}, []string{}); err != nil {
		panic(err)
	}
}

func parseKernelFlags() error {
	data, err := os.ReadFile(fs.Resolve("process:cmdline"))
	if err != nil {
		return fmt.Errorf("failed to read kernel cmdline flags %v", err)
	}

	for a := range strings.FieldsSeq(string(data)) {
		k, v := a, ""
		if i := strings.Index(k, "="); i != -1 {
			v = k[i+1:]
			k = k[:i]
		}

		switch k {
		case "root":
			rootfs = fs.Resolve("%s", v)
		case "rootfstype":
			rootfsType = v
		case "avyos":
			avyosfs = fs.Resolve("%s", v)
		case "avyosfstype":
			avysofsType = v
		case "live":
			live = true
		}
	}

	return nil
}

func safeMount(source, target, fstype, options string, flags uintptr) {
	if _, err := os.Stat(source); err != nil {
		blocks, err := os.ReadDir(fs.Resolve("sysfs:block"))
		if err != nil {
			panic("failed to read sysfs:block " + err.Error())
		}
		for i, block := range blocks {
			fmt.Println(i, block.Name())
		}
		panic("no source device present at " + source)
	}

	if err := os.MkdirAll(target, 0755); err != nil {
		panic(err)
	}

	if err := syscall.Mount(source, target, fstype, flags, options); err != nil {
		panic(fmt.Errorf("failed to mount %s: %v", source, err))
	}
}

func mountEssentialFilesystems() {
	mounts := []struct {
		source string
		target string
		fstype string
		flags  uintptr
		data   string
	}{
		{"proc", fs.Resolve("process:"), "proc", 0, ""},
		{"sysfs", fs.Resolve("sysfs:"), "sysfs", 0, ""},
		{"devtmpfs", fs.Resolve("device:"), "devtmpfs", 0, ""},
		{"devpts", fs.Resolve("device:pts"), "devpts", 0, "ptmxmode=0666,mode=0620"},
		{"tmpfs", fs.Resolve("shared:"), "tmpfs", 0, "mode=1777"},
		{"tmpfs", fs.Resolve("system:"), "tmpfs", 0, ""},
	}

	for _, m := range mounts {
		_ = os.MkdirAll(m.target, 0755)

		if isMounted(m.target) {
			continue
		}

		err := syscall.Mount(m.source, m.target, m.fstype, m.flags, m.data)
		if err != nil {
			fmt.Printf("init: Failed to mount %s: %v\n", m.target, err)
		}
	}
}

func isMounted(path string) bool {
	file, err := os.Open(fs.Resolve("process:mounts"))
	if err != nil {
		return false
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) >= 2 && fields[1] == path {
			return true
		}
	}
	return false
}
