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
)

var (
	rootfs     string = "tmpfs"
	rootfstype string = "tmpfs"
	usrfs      string
	usrfstype  string = "squashfs"
	live       bool
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
	safeMount(rootfs, "/rootfs", rootfstype, "", 0)

	if usrfs == "" {
		if !pathExists("/rootfs/usr") {
			panic("usrfs not found")
		}
	} else {
		safeMount(usrfs, "/rootfs/usr", usrfstype, "", syscall.MS_RDONLY)
	}

	for _, fs := range []string{
		"/proc",
		"/sys",
		"/dev",
		"/run",
	} {
		os.MkdirAll("/rootfs/"+fs, 0755)
		syscall.Mount("/"+fs, "/rootfs/"+fs, "", syscall.MS_MOVE, "")
	}

	syscall.Chdir("/rootfs")
	syscall.Chroot("/rootfs")

	if err := syscall.Exec("/usr/sbin/init", []string{}, []string{}); err != nil {
		panic(err)
	}
}

func parseKernelFlags() error {
	data, err := os.ReadFile("/proc/cmdline")
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
			rootfs = v
		case "rootfstype":
			rootfstype = v
		case "usr":
			usrfs = v
		case "usrfstype":
			usrfstype = v
		case "live":
			live = true
		}
	}

	return nil
}

func safeMount(source, target, fstype, options string, flags uintptr) {
	if fstype != "tmpfs" {
		if _, err := os.Stat(source); err != nil {
			blocks, err := os.ReadDir("/sys/class/block")
			if err != nil {
				panic("failed to read /sys/class/block " + err.Error())
			}
			for i, block := range blocks {
				fmt.Println(i, block.Name())
			}
			panic("no source device present at " + source)
		}
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
		{"proc", "/proc", "proc", 0, ""},
		{"sysfs", "/sys", "sysfs", 0, ""},
		{"devtmpfs", "/dev", "devtmpfs", 0, ""},
		{"devpts", "/dev/pts", "devpts", 0, "ptmxmode=0666,mode=0620"},
		{"tmpfs", "/dev/shm", "tmpfs", 0, "mode=1777"},
		{"tmpfs", "/run", "tmpfs", 0, ""},
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
	file, err := os.Open("/proc/mounts")
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

func pathExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
