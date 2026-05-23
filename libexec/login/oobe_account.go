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
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
)

type loginAccountSpec struct {
	Username string
	FullName string
	Groups   []string
	Home     string
	Shell    string
}

func addLoginAccount(spec loginAccountSpec) error {
	args := []string{
		"add",
		"--name", strings.TrimSpace(spec.FullName),
		"--groups", strings.Join(spec.Groups, ","),
		"--home", strings.TrimSpace(spec.Home),
		"--shell", shellOrDefault(spec.Shell),
		strings.TrimSpace(spec.Username),
	}
	return runIdentity(args, nil)
}

func updateLoginPassword(username, password string) error {
	return runIdentity([]string{"passwd", strings.TrimSpace(username), password}, []int{2})
}

func runIdentity(args []string, secretIndexes []int) error {
	cmd := exec.Command("/usr/sbin/identity", args...)
	out, err := cmd.CombinedOutput()
	if err == nil {
		return nil
	}
	msg := strings.TrimSpace(string(out))
	if msg == "" {
		msg = err.Error()
	}
	return fmt.Errorf("identity %s failed: %s", redactedArgs(args, secretIndexes), msg)
}

func redactedArgs(args []string, secretIndexes []int) string {
	redacted := append([]string(nil), args...)
	for _, idx := range secretIndexes {
		if idx >= 0 && idx < len(redacted) {
			redacted[idx] = "***"
		}
	}
	return strings.Join(redacted, " ")
}

func defaultHomeForUser(username string) string {
	return filepath.Join("/home", username)
}

func shellOrDefault(shell string) string {
	shell = strings.TrimSpace(shell)
	if shell == "" {
		return "/usr/bin/sh"
	}
	return shell
}
