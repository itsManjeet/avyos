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
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
)

type shutdownMode int

const (
	shutdownModePoweroff shutdownMode = iota
	shutdownModeReboot
)

var shutdownOnce sync.Once
var shutdownRequested atomic.Bool

func shutdownPoweroff() {
	shutdown(shutdownModePoweroff)
}

func shutdownReboot() {
	shutdown(shutdownModeReboot)
}

func shutdown(mode shutdownMode) {
	shutdownRequested.Store(true)
	shutdownOnce.Do(func() {
		log.Info("Shutting down services...")

		if managerServer != nil {
			_ = managerServer.Close()
		}

		// Stopping supervisor
		sv.Stop()

		// Sync filesystems
		syscall.Sync()

		// Unmount filesystems
		log.Info("Unmounting filesystems...")
		unmountAll()
		syscall.Sync()

		switch mode {
		case shutdownModeReboot:
			log.Info("System rebooting")
			if err := syscall.Reboot(syscall.LINUX_REBOOT_CMD_RESTART); err != nil {
				log.Error("reboot syscall failed: %v", err)
			}
		default:
			log.Info("System powering off")
			if err := syscall.Reboot(syscall.LINUX_REBOOT_CMD_POWER_OFF); err != nil {
				log.Error("poweroff syscall failed: %v", err)
			}
		}

		// PID1 must not exit even if reboot syscall fails.
		for {
			syscall.Pause()
		}
	})
}

func isShutdownRequested() bool {
	return shutdownRequested.Load()
}

func unmountAll() {
	// Read mounts in reverse order
	file, err := os.Open("/proc/mounts")
	if err != nil {
		return
	}
	defer file.Close()

	var mounts []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) >= 2 {
			mounts = append(mounts, fields[1])
		}
	}
	_ = scanner.Err()

	// Unmount in reverse order, skipping essential ones
	essential := map[string]bool{
		"/":        true,
		"/usr":     true,
		"/proc":    true,
		"/sys":     true,
		"/dev/pts": true,
		"/dev":     true,
	}

	for i := len(mounts) - 1; i >= 0; i-- {
		mp := mounts[i]
		if essential[mp] {
			continue
		}
		syscall.Unmount(mp, 0)
	}
}
