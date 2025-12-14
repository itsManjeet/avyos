/*
 * Copyright (c) 2025 Manjeet Singh <itsmanjeet1998@gmail.com>.
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
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"

	"avyos.dev/pkg/ensure"
)

var (
	errors     []string
	rootfs     string
	rootfsType string
	live       bool
)

func isInsideInitramfs() bool {
	// TODO: check using /proc/mount for rootfs if mounted to tmpfs
	return os.Args[0] == "/init"
}

func ensureRealRootfs() {
	if !isInsideInitramfs() {
		return
	}

	safeCall("mount(devtmpfs)", syscall.Mount("devtmpfs", "/dev", "devtmpfs", syscall.MS_NOSUID, "mode=0755"))

	safeCall("mkdir(proc)", os.MkdirAll("/proc", 0755))
	safeCall("mount(proc)", syscall.Mount("proc", "/proc", "proc", syscall.MS_NOSUID|syscall.MS_NOEXEC|syscall.MS_NODEV, ""))

	safeCall("mkdir(sysfs)", os.MkdirAll("/sys", 0755))
	safeCall("mount(sysfs)", syscall.Mount("sysfs", "/sys", "sysfs", syscall.MS_NOSUID|syscall.MS_NOEXEC|syscall.MS_NODEV, ""))

	safeCall("mkdir(tmpfs)", os.MkdirAll("/run", 0755))
	safeCall("mount(tmpfs)", syscall.Mount("tmpfs", "/run", "tmpfs", syscall.MS_NOSUID|syscall.MS_NODEV, "mode=0755"))

	safeCall("mkdir(devpts)", os.MkdirAll("/dev/pts", 0755))
	safeCall("mount(devpts)", syscall.Mount("devpts", "/dev/pts", "devpts", syscall.MS_NOSUID|syscall.MS_NOEXEC, "mode=0620,gid=5"))

	safeCall("mkdir(shm)", os.MkdirAll("/dev/shm", 0755))
	safeCall("mount(shm)", syscall.Mount("tmpfs", "/dev/shm", "tmpfs", syscall.MS_NOSUID|syscall.MS_NODEV, "mode=1777"))

	ensureStage("kernel pseudo filesystem mount")

	safeCall("parse(/proc/cmdline)", parseKernelFlags())
	ensureStage("parsing kernel args")

	if rootfs == "" {
		log.Fatal("no root device specified")
	}

	if _, err := os.Stat(rootfs); err != nil {
		blocks, err := os.ReadDir("/sys/block")
		if err != nil {
			log.Fatal("failed to read /sys/block ", err)
		}
		for i, block := range blocks {
			log.Println(i, block.Name())
		}
		log.Fatal("no root device present ", rootfs)
	}
	for _, dir := range []string{"/run/overlay", "/run/overlay/ro", "/run/overlay/rw", "/run/overlay/work", "/rootfs"} {
		safeCall("mkdir("+dir+")", os.MkdirAll(dir, 0755))
	}

	var rootPath string
	if live {
		rootPath = "/run/iso"
		safeCall("mkdir("+rootPath+")", os.MkdirAll(rootPath, 0755))
	} else {
		rootPath = "/run/overlay/ro"
	}

	safeCall("mount(rootfs)", syscall.Mount(rootfs, rootPath, rootfsType, syscall.MS_RDONLY, ""))
	if live {
		safeCall("mount(squashfs)", exec.Command("/cmd/busybox", "mount", filepath.Join(rootPath, "system.img"), "/run/overlay/ro").Run())
	}

	safeCall("mount(overlay)", syscall.Mount("overlay", "/rootfs", "overlay", 0, "lowerdir=/run/overlay/ro,upperdir=/run/overlay/rw,workdir=/run/overlay/work"))
	ensureStage("prepare real rootfs")

	for _, fs := range []string{"proc", "sys", "dev", "run"} {
		safeCall("mkdir("+fs+")", os.MkdirAll("/rootfs/"+fs, 0755))
		safeCall("mount("+fs+")", syscall.Mount("/"+fs, "/rootfs/"+fs, "", syscall.MS_MOVE, ""))
	}

	safeCall("chdir(rootfs)", syscall.Chdir("/rootfs"))
	safeCall("chroot(rootfs)", syscall.Chroot("/rootfs"))
	ensureStage("switch to real rootfs")

	if err := syscall.Exec("/cmd/init", []string{"/cmd/init"}, []string{}); err != nil {
		log.Fatal(err)
	}
}

func safeCall(msg string, err error) {
	if err != nil {
		errors = append(errors, fmt.Sprintf("%s: %v", msg, err))
	}
}

func ensureStage(stage string) {
	if errors != nil {
		ensure.Foreach(errors, func(msg string) error {
			log.Println(msg)
			return nil
		})

		log.Fatal("failed to complete ", stage)
	}
}

func parseKernelFlags() error {
	data, err := os.ReadFile("/proc/cmdline")
	if err != nil {
		return fmt.Errorf("failed to read kernel cmdline flags %v", err)
	}

	for _, a := range strings.Fields(string(data)) {
		k, v := a, ""
		if i := strings.Index(k, "="); i != -1 {
			v = k[i+1:]
			k = k[:i]
		}

		switch k {
		case "root":
			rootfs = v
		case "rootfstype":
			rootfsType = v
		case "live":
			live = true
		}
	}

	return nil
}
