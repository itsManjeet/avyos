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
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
)

func (s *Service) Start() error {
	cmd := exec.Command(s.Command, s.Args...)

	// Setup TTY if specified
	var ttyFile *os.File
	var logFile *os.File
	if s.TTY != "" {
		var err error
		ttyFile, err = os.OpenFile(s.TTY, os.O_RDWR, 0)
		if err != nil {
			return fmt.Errorf("failed to open tty %s: %w", s.TTY, err)
		}

		cmd.Stdin = ttyFile
		cmd.Stdout = ttyFile
		cmd.Stderr = ttyFile

		// Create new session and set controlling terminal
		// Ctty must be the fd number in the child process (0 = stdin)
		cmd.SysProcAttr = &syscall.SysProcAttr{
			Setsid:  true,
			Setctty: true,
			Ctty:    0, // stdin fd in child
		}

		// Set TERM environment variable
		if s.Environment == nil {
			s.Environment = make(map[string]string)
		}
		if _, exists := s.Environment["TERM"]; !exists {
			s.Environment["TERM"] = "linux"
		}
	} else {
		logPath := "/var/log/" + serviceLogFileName(s) + ".log"
		logDir := filepath.Dir(logPath)
		if err := os.MkdirAll(logDir, 0755); err != nil {
			return fmt.Errorf("failed to create %s: %w", logDir, err)
		}
		var err error
		logFile, err = os.OpenFile(logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
		if err != nil {
			return fmt.Errorf("failed to open %s: %w", logPath, err)
		}
		cmd.Stdin = os.Stdin
		cmd.Stdout = logFile
		cmd.Stderr = logFile
	}

	// Set environment
	cmd.Env = os.Environ()
	for k, v := range s.Environment {
		cmd.Env = append(cmd.Env, k+"="+v)
	}

	if s.Type == "oneshot" {
		err := cmd.Run()
		if ttyFile != nil {
			ttyFile.Close()
		}
		if logFile != nil {
			logFile.Close()
		}
		return err
	}

	if err := cmd.Start(); err != nil {
		if ttyFile != nil {
			ttyFile.Close()
		}
		if logFile != nil {
			logFile.Close()
		}
		return err
	}

	if logFile != nil {
		logFile.Close()
	}

	s.Process = cmd.Process
	return nil
}

func serviceLogFileName(s *Service) string {
	name := strings.TrimSpace(s.Name)
	if name == "" {
		name = filepath.Base(strings.TrimSpace(s.Command))
	}
	name = strings.ToLower(name)
	if name == "" {
		return "unknown"
	}

	var b strings.Builder
	b.Grow(len(name))
	for _, r := range name {
		switch {
		case r >= 'a' && r <= 'z':
			b.WriteRune(r)
		case r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == '-' || r == '_' || r == '.':
			b.WriteRune(r)
		default:
			b.WriteByte('_')
		}
	}
	out := strings.Trim(b.String(), "._-")
	if out == "" {
		return "unknown"
	}
	return out
}
