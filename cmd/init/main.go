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
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"avyos.dev/pkg/ensure"
)

func main() {
	ensure.Output(os.Getpid(), 1, "INIT must run as PID 1")
	ensureRealRootfs()

	kmsg, err := os.OpenFile("/dev/kmsg", os.O_RDWR, 0)
	if err != nil {
		log.SetOutput(kmsg)
	}
	defer func() {
		if kmsg != nil {
			kmsg.Close()
		}
	}()

	for k, v := range map[string]string{
		"PATH":            "/cmd",
		"USER":            "root",
		"TERM":            "linux",
		"XDG_DATA_DIRS":   "/data",
		"XDG_CONFIG_DIRS": "/config",
		"TMPDIR":          "/cache",
	} {
		os.Setenv(k, v)
	}

	ctxt, cancel := context.WithCancel(context.Background())

	go func() {
		defer cancel()
		if err := startup(); err != nil {
			log.Printf("ERROR in startup = %v", err)
		}
	}()

	_ = syscall.Reboot(syscall.LINUX_REBOOT_CMD_CAD_OFF)

	signalChannel := make(chan os.Signal, 4)
	signal.Notify(signalChannel,
		syscall.SIGUSR1,
		syscall.SIGUSR2,
		syscall.SIGCHLD,
		syscall.SIGINT)

	var rebootCommand int
	for rebootCommand == 0 {
		select {
		case sig := <-signalChannel:
			switch sig {
			case syscall.SIGUSR1:
				rebootCommand = syscall.LINUX_REBOOT_CMD_KEXEC

			case syscall.SIGUSR2:
				rebootCommand = syscall.LINUX_REBOOT_CMD_POWER_OFF

			case syscall.SIGINT, syscall.SIGTERM:
				rebootCommand = syscall.LINUX_REBOOT_CMD_RESTART

			case syscall.SIGCHLD:
				waitForChildProcesses(syscall.WNOHANG)
			}
		case <-ctxt.Done():
			rebootCommand = syscall.LINUX_REBOOT_CMD_POWER_OFF
		}
	}

	shutdown()

	_ = syscall.Reboot(syscall.LINUX_REBOOT_CMD_CAD_ON)

	waitForChildProcesses(syscall.WNOHANG)
	<-ctxt.Done()

	syscall.Sync()
	if rebootCommand == 0 {
		rebootCommand = syscall.LINUX_REBOOT_CMD_POWER_OFF
	}

	if err := syscall.Reboot(rebootCommand); err != nil {
		log.Fatal(err)
	}
}

func waitForChildProcesses(options int) {
	for {
		if pid, _ := syscall.Wait4(-1, nil, options, nil); pid <= 0 {
			break
		}
	}
}
