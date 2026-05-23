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

package service

import (
	"os"
)

// Service represents a service configuration.
type Service struct {
	Name        string
	Description string
	Command     string
	Args        []string
	After       []string // Services this depends on
	Type        string   // "oneshot" or "daemon"
	Restart     string   // "always", "on-failure", "never"
	Environment map[string]string
	TTY         string // TTY device path (e.g., "/dev/tty1"), empty for init's stdio

	// Runtime state
	Process *os.Process
	Started bool
	Failed  bool
	Retries int // Number of consecutive restart attempts after failure
}
