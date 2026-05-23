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
	"bufio"
	"os"
	"path/filepath"
	"strings"
)

func Parse(path string) (*Service, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	svc := &Service{
		Environment: make(map[string]string),
		Type:        "daemon",
		Restart:     "on-failure",
	}

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		before, after, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}

		key := strings.TrimSpace(before)
		value := strings.TrimSpace(after)

		switch strings.ToLower(key) {
		case "name":
			svc.Name = value
		case "description":
			svc.Description = value
		case "command":
			parts := strings.Fields(value)
			if len(parts) > 0 {
				svc.Command = parts[0]
				svc.Args = parts[1:]
			}
		case "after":
			svc.After = strings.Split(value, ",")
			for i := range svc.After {
				svc.After[i] = strings.TrimSpace(svc.After[i])
			}
		case "type":
			svc.Type = value
		case "restart":
			svc.Restart = value
		case "tty":
			svc.TTY = value
		case "environment":
			envParts := strings.SplitN(value, "=", 2)
			if len(envParts) == 2 {
				svc.Environment[envParts[0]] = envParts[1]
			}
		}
	}

	if svc.Name == "" {
		// Use filename without .service extension
		base := filepath.Base(path)
		svc.Name = strings.TrimSuffix(base, ".service")
	}

	return svc, scanner.Err()
}
