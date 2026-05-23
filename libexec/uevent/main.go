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

	"avyos.dev/lib/logger"
)

var serviceLog = logger.New("uevent")

func main() {
	if err := logger.SetupLog(); err != nil {
		serviceLog.Error("failed to setup system log: %v", err)
	}

	srv, err := NewServer()
	if err != nil {
		serviceLog.Error("failed to create server: %v", err)
		os.Exit(1)
	}

	serviceLog.Info("starting uevent service")

	if err := srv.Run(); err != nil {
		serviceLog.Error("uevent service error: %v", err)
		os.Exit(1)
	}

	serviceLog.Info("uevent service stopped")
}
