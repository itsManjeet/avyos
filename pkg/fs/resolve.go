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

package fs

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func Resolve(format string, args ...any) string {
	path := fmt.Sprintf(format, args...)
	idx := strings.IndexByte(path, ':')
	if idx == -1 {
		return path
	}
	scheme := path[:idx]
	path = path[idx+1:]

	HOME := os.Getenv("HOME")

	switch scheme {
	case "app":
		return resolvePath(path, filepath.Join(HOME, "/Applications"), "/apps", "/avyos/apps")
	case "cmd":
		return resolvePath(path, filepath.Join(HOME, "/cmd"), "/cmd", "/avyos/cmd")
	case "config":
		return resolvePath(path, filepath.Join(HOME, "/config"), "/config", "/avyos/config")
	case "data":
		return resolvePath(path, filepath.Join(HOME, "/data"), "/data", "/avyos/data")
	case "cache":
		return filepath.Join("/cache", path)
	case "process":
		return filepath.Join("/cache/kernel/processes", path)
	case "device":
		return filepath.Join("/cache/kernel/devices", path)
	case "sysfs":
		return filepath.Join("/cache/kernel/sysfs", path)
	case "shared":
		return filepath.Join("/cache/kernel/shared", path)
	case "system", "service":
		return filepath.Join("/cache/runtime", path)
	case "user":
		return filepath.Join(fmt.Sprintf("/cache/runtime/user/%d", os.Getuid()), path)
	}
	return path
}

func resolvePath(file string, paths ...string) string {
	for _, path := range paths {
		path = filepath.Join(path, file)
		if Exists(path) {
			return path
		}
	}
	return file
}
