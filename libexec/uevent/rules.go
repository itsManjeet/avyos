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
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"

	"avyos.dev/lib/fs"
	"avyos.dev/lib/identity"
	"avyos.dev/lib/ini"
)

// Rule represents a uevent processing rule loaded from uevent.conf.
// Each INI section (except defaults) defines a rule.
//
// Config format (INI):
//
//	[input]
//	subsystem = input
//	devname = input/*
//	group = input
//	mode = 0660
//
//	[tty]
//	subsystem = tty
//	group = tty
//	mode = 0660
//
//	[block]
//	subsystem = block
//	group = disk
//	mode = 0660
//	run = /avyos/cmd/driver load $DRIVER
//
//	[net]
//	subsystem = net
//	service = network:link-up
type Rule struct {
	Name       string
	Subsystem  string // match: subsystem name or glob
	DevName    string // match: device name or glob
	DevType    string // match: device type
	Driver     string // match: driver name
	Action     string // match: action (add, remove, change)
	Owner      int    // set: uid for device node (-1 = unset)
	Group      int    // set: gid for device node (-1 = unset)
	Mode       uint32 // set: permission mode for device node
	Symlink    string // set: create symlink at this path
	Run        string // set: execute command
	Service    string // set: notify service (service-name:event)
	LoadDriver bool   // set: auto-load matching driver
}

// loadRules reads and parses rules from config:uevent.conf.
func loadRules() ([]Rule, error) {
	configPath := "/etc/defaults/uevent.conf"
	cfg, err := ini.ParseFile(configPath)
	if err != nil {
		return nil, err
	}

	var rules []Rule
	for name, section := range cfg.Sections {
		if name == "" {
			continue // skip default section
		}

		rule := Rule{
			Name:  name,
			Owner: -1,
			Group: -1,
			Mode:  0660,
		}

		for _, entry := range section.Entries {
			if entry.Type != ini.EntryKeyValue {
				continue
			}
			switch entry.Key {
			case "subsystem":
				rule.Subsystem = entry.Value
			case "devname":
				rule.DevName = entry.Value
			case "devtype":
				rule.DevType = entry.Value
			case "driver":
				rule.Driver = entry.Value
			case "action":
				rule.Action = entry.Value
			case "owner":
				if uid, err := strconv.Atoi(entry.Value); err == nil {
					rule.Owner = uid
				} else {
					cap, err := identity.LookupByName(entry.Value)
					if err == nil {
						rule.Owner = cap.ID
					}
				}
			case "group":
				if gid, err := strconv.Atoi(entry.Value); err == nil {
					rule.Group = gid
				} else {
					cap, err := identity.LookupCapabilityByName(entry.Value)
					if err == nil {
						rule.Group = cap.ID
					}
				}
			case "mode":
				m, err := strconv.ParseUint(entry.Value, 8, 32)
				if err == nil {
					rule.Mode = uint32(m)
				}
			case "symlink":
				rule.Symlink = entry.Value
			case "run":
				rule.Run = entry.Value
			case "service":
				rule.Service = entry.Value
			case "load_driver":
				rule.LoadDriver = entry.Value == "true" || entry.Value == "1"
			}
		}

		rules = append(rules, rule)
	}

	return rules, nil
}

// matchRule checks if a uevent matches this rule.
func matchRule(rule *Rule, ev *UEvent) bool {
	if rule.Subsystem != "" && !matchGlob(rule.Subsystem, ev.Subsystem) {
		return false
	}
	if rule.DevName != "" && !matchGlob(rule.DevName, ev.DevName) {
		return false
	}
	if rule.DevType != "" && rule.DevType != ev.DevType {
		return false
	}
	if rule.Driver != "" && !matchGlob(rule.Driver, ev.Driver) {
		return false
	}
	if rule.Action != "" && rule.Action != ev.Action {
		return false
	}
	return true
}

// applyRule applies a matched rule to the uevent.
func applyRule(rule *Rule, ev *UEvent) {
	switch ev.Action {
	case "add", "change":
		applyAdd(rule, ev)
	case "remove":
		applyRemove(rule, ev)
	}
}

func applyAdd(rule *Rule, ev *UEvent) {
	if ev.DevName == "" {
		return
	}

	devPath := fmt.Sprintf("/dev/%s", ev.DevName)

	// Create parent directories
	dir := filepath.Dir(devPath)
	if !fs.Exists(dir) {
		_ = os.MkdirAll(dir, 0755)
	}

	// Create device node if major/minor present
	if ev.Major != "" && ev.Minor != "" {
		major, _ := strconv.Atoi(ev.Major)
		minor, _ := strconv.Atoi(ev.Minor)
		if major > 0 || minor > 0 {
			devNum := makedev(major, minor)
			mode := rule.Mode
			if ev.Subsystem == "block" {
				mode |= syscall.S_IFBLK
			} else {
				mode |= syscall.S_IFCHR
			}

			// Remove stale node if it exists
			_ = syscall.Unlink(devPath)

			if err := syscall.Mknod(devPath, mode, int(devNum)); err != nil {
				serviceLog.Error("mknod %s: %v", devPath, err)
				return
			}

			serviceLog.Debug("created device node %s (%d:%d)", devPath, major, minor)
		}
	}

	// Set ownership
	uid := max(rule.Owner, 0)
	gid := max(rule.Group, 0)
	_ = syscall.Chown(devPath, uid, gid)

	// Set permissions
	_ = syscall.Chmod(devPath, rule.Mode)

	// Create symlink
	if rule.Symlink != "" {
		link := expandVars(rule.Symlink, ev)
		linkPath := fmt.Sprintf("/dev/%s", link)
		_ = os.MkdirAll(filepath.Dir(linkPath), 0755)
		_ = os.Symlink(devPath, linkPath)
	}

	// Execute command
	if rule.Run != "" {
		runCommand(rule.Run, ev)
	}

	// Load driver
	if rule.LoadDriver && ev.Driver != "" {
		runCommand("/avyos/cmd/driver load "+ev.Driver, ev)
	}

	// Notify service
	if rule.Service != "" {
		notifyService(rule.Service, ev)
	}
}

func applyRemove(rule *Rule, ev *UEvent) {
	if ev.DevName == "" {
		return
	}

	devPath := fmt.Sprintf("/dev/%s", ev.DevName)
	_ = syscall.Unlink(devPath)

	// Remove symlink
	if rule.Symlink != "" {
		link := expandVars(rule.Symlink, ev)
		linkPath := fmt.Sprintf("/dev/%s", link)
		_ = syscall.Unlink(linkPath)
	}

	// Execute command
	if rule.Run != "" {
		runCommand(rule.Run, ev)
	}

	// Notify service
	if rule.Service != "" {
		notifyService(rule.Service, ev)
	}
}

func makedev(major, minor int) uint64 {
	return uint64(major)<<8 | uint64(minor)
}

func expandVars(s string, ev *UEvent) string {
	s = strings.ReplaceAll(s, "$DEVNAME", ev.DevName)
	s = strings.ReplaceAll(s, "$DEVPATH", ev.DevPath)
	s = strings.ReplaceAll(s, "$SUBSYSTEM", ev.Subsystem)
	s = strings.ReplaceAll(s, "$DEVTYPE", ev.DevType)
	s = strings.ReplaceAll(s, "$DRIVER", ev.Driver)
	s = strings.ReplaceAll(s, "$MAJOR", ev.Major)
	s = strings.ReplaceAll(s, "$MINOR", ev.Minor)
	s = strings.ReplaceAll(s, "$ACTION", ev.Action)
	for k, v := range ev.Env {
		s = strings.ReplaceAll(s, "$"+k, v)
	}
	return s
}

func runCommand(cmdLine string, ev *UEvent) {
	cmdLine = expandVars(cmdLine, ev)
	parts := strings.Fields(cmdLine)
	if len(parts) == 0 {
		return
	}

	cmd := exec.Command(parts[0], parts[1:]...)
	cmd.Env = os.Environ()
	for k, v := range ev.Env {
		cmd.Env = append(cmd.Env, k+"="+v)
	}

	if err := cmd.Start(); err != nil {
		serviceLog.Error("run %s: %v", parts[0], err)
		return
	}

	// Don't block on command completion
	go func() {
		if err := cmd.Wait(); err != nil {
			serviceLog.Warn("command %s exited: %v", parts[0], err)
		}
	}()
}

func notifyService(target string, ev *UEvent) {
	// Format: "service-name:event-string"
	parts := strings.SplitN(target, ":", 2)
	if len(parts) != 2 {
		serviceLog.Warn("invalid service target: %s", target)
		return
	}

	serviceName := parts[0]
	eventName := parts[1]

	socketPath := "/run/" + serviceName

	go func() {
		client, err := connectService(socketPath)
		if err != nil {
			serviceLog.Debug("cannot notify %s: %v", serviceName, err)
			return
		}
		defer client.Close()

		payload := expandVars(eventName, ev)
		_ = client.Send(1, 0x0100, []byte(payload))
	}()
}

func connectService(socketPath string) (serviceClient, error) {
	conn, err := syscall.Socket(syscall.AF_UNIX, syscall.SOCK_STREAM, 0)
	if err != nil {
		return serviceClient{}, err
	}
	sa := &syscall.SockaddrUnix{Name: socketPath}
	if err := syscall.Connect(conn, sa); err != nil {
		syscall.Close(conn)
		return serviceClient{}, err
	}
	return serviceClient{fd: conn}, nil
}

type serviceClient struct {
	fd int
}

func (c *serviceClient) Send(dest uint32, event uint16, payload []byte) error {
	// Build a minimal sutra transaction
	buf := make([]byte, 12+len(payload))
	buf[0] = 0x00
	buf[1] = 0x00
	buf[2] = 0x00
	buf[3] = 0x80 // client ID range
	buf[4] = byte(dest)
	buf[5] = byte(dest >> 8)
	buf[6] = byte(dest >> 16)
	buf[7] = byte(dest >> 24)
	buf[8] = byte(event)
	buf[9] = byte(event >> 8)
	buf[10] = byte(len(payload))
	buf[11] = byte(len(payload) >> 8)
	copy(buf[12:], payload)

	_, err := syscall.Write(c.fd, buf)
	return err
}

func (c *serviceClient) Close() error {
	return syscall.Close(c.fd)
}

// matchGlob performs simple glob matching (supports * only).
func matchGlob(pattern, value string) bool {
	if pattern == "*" {
		return true
	}
	if !strings.Contains(pattern, "*") {
		return pattern == value
	}
	matched, _ := filepath.Match(pattern, value)
	return matched
}
