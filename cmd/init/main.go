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
	"os"
	"os/signal"
	"syscall"

	"avyos.dev/cmd/init/supervisor"
	"avyos.dev/pkg/fs"
	"avyos.dev/pkg/logger"
)

var log = logger.New("init")

func main() {
	// Check if we're PID 1
	if os.Getpid() != 1 {
		log.Warn("This program should run as PID 1")
		log.Info("Starting in debug mode...")
	}

	log.Info("Welcome to avyos")

	// Ensure inside real root else switch_root
	ensureRealRootfs()

	if err := logger.SetupLog(); err != nil {
		log.Error("Failed to setup system log: %v", err)
	} else {
		log.Info("System logging enabled at %s", fs.Resolve("cache:log/services/boot.log"))
	}

	// Do startup (which will setup signal handlers with supervisor's PID channel)
	startup()
}

func setupSignalHandlers(exitChan chan<- supervisor.ProcessExit) {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGTERM, syscall.SIGINT, syscall.SIGCHLD)

	go func() {
		for sig := range sigChan {
			switch sig {
			case syscall.SIGTERM, syscall.SIGINT:
				log.Info("Received shutdown signal")
				shutdownPoweroff()
			case syscall.SIGCHLD:
				reapZombies(exitChan)
			}
		}
	}()
}

func reapZombies(exitChan chan<- supervisor.ProcessExit) {
	for {
		var ws syscall.WaitStatus
		pid, err := syscall.Wait4(-1, &ws, syscall.WNOHANG, nil)
		if err != nil || pid <= 0 {
			break
		}

		// Send process exit info to supervisor (non-blocking)
		select {
		case exitChan <- supervisor.ProcessExit{PID: pid, Status: ws}:
			log.Debug("Sent exit notification for PID %d to supervisor", pid)
		default:
			log.Warn("Supervisor exit channel full, dropping PID %d", pid)
		}
	}
}
