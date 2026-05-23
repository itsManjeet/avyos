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
	"path/filepath"
	"strings"
	"syscall"
)

// UEvent represents a parsed kernel uevent.
type UEvent struct {
	Action    string
	DevPath   string
	Subsystem string
	DevName   string
	DevType   string
	Major     string
	Minor     string
	Driver    string
	Env       map[string]string
}

// openNetlinkSocket opens a NETLINK_KOBJECT_UEVENT socket for receiving
// kernel device events.
func openNetlinkSocket() (int, error) {
	fd, err := syscall.Socket(
		syscall.AF_NETLINK,
		syscall.SOCK_DGRAM,
		15, // NETLINK_KOBJECT_UEVENT
	)
	if err != nil {
		return -1, err
	}

	addr := &syscall.SockaddrNetlink{
		Family: syscall.AF_NETLINK,
		Pid:    uint32(syscall.Getpid()),
		Groups: 1, // UEVENT multicast group
	}

	if err := syscall.Bind(fd, addr); err != nil {
		syscall.Close(fd)
		return -1, err
	}

	// Set receive buffer size
	_ = syscall.SetsockoptInt(fd, syscall.SOL_SOCKET, syscall.SO_RCVBUF, 256*1024)

	return fd, nil
}

// recvUEvent reads and parses a single uevent from the netlink socket.
func recvUEvent(fd int) (*UEvent, error) {
	buf := make([]byte, 8192)
	n, _, err := syscall.Recvfrom(fd, buf, 0)
	if err != nil {
		return nil, err
	}
	return parseUEvent(buf[:n]), nil
}

// parseUEvent parses a raw netlink uevent message.
// Format: "action@devpath\0KEY=VALUE\0KEY=VALUE\0..."
func parseUEvent(data []byte) *UEvent {
	ev := &UEvent{
		Env: make(map[string]string),
	}

	parts := strings.SplitSeq(string(data), "\x00")
	for part := range parts {
		if part == "" {
			continue
		}

		// First line is "action@devpath"
		if ev.Action == "" && strings.Contains(part, "@") {
			before, after, _ := strings.Cut(part, "@")
			ev.Action = before
			ev.DevPath = after
			continue
		}

		// KEY=VALUE pairs
		if idx := strings.Index(part, "="); idx > 0 {
			key := part[:idx]
			value := part[idx+1:]
			ev.Env[key] = value

			switch key {
			case "ACTION":
				ev.Action = value
			case "DEVPATH":
				ev.DevPath = value
			case "SUBSYSTEM":
				ev.Subsystem = value
			case "DEVNAME":
				ev.DevName = value
			case "DEVTYPE":
				ev.DevType = value
			case "MAJOR":
				ev.Major = value
			case "MINOR":
				ev.Minor = value
			case "DRIVER":
				ev.Driver = value
			}
		}
	}

	return ev
}

// triggerSubsystem writes "add" to each device's uevent file under
// /sys to re-trigger coldplug events for a subsystem.
func triggerSubsystem(subsystem string) error {
	return triggerPath("/sys/class/" + subsystem)
}

// triggerAll triggers uevents for all devices.
func triggerAll() error {
	return triggerPath("/sys/devices")
}

func triggerPath(root string) error {
	return filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		if strings.HasSuffix(path, "/uevent") {
			fd, err := syscall.Open(path, syscall.O_WRONLY, 0)
			if err == nil {
				syscall.Write(fd, []byte("add"))
				syscall.Close(fd)
			}

		}
		return nil
	})
}
