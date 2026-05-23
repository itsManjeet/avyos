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
	"net"
	"os"
	"strconv"
	"sync"
	"sync/atomic"
	"syscall"

	"avyos.dev/api/uevent"
	"avyos.dev/lib/sutra"
)

// Server is the uevent daemon. It listens on a netlink socket for kernel
// device events, applies rules from uevent.conf, creates device nodes,
// and exposes a sutra service for clients.
type Server struct {
	nlFD     int
	rules    []Rule
	devices  map[string]uevent.DeviceInfo // devpath -> info
	deviceMu sync.RWMutex
	done     chan struct{}

	connMu  sync.RWMutex
	conns   map[uint32]*sutra.Conn
	nextCID atomic.Uint32
}

func NewServer() (*Server, error) {
	nlFD, err := openNetlinkSocket()
	if err != nil {
		return nil, fmt.Errorf("netlink socket: %w", err)
	}

	rules, err := loadRules()
	if err != nil {
		serviceLog.Warn("failed to load rules: %v", err)
		rules = []Rule{}
	}
	serviceLog.Info("loaded %d rules", len(rules))

	srv := &Server{
		nlFD:    nlFD,
		rules:   rules,
		devices: make(map[string]uevent.DeviceInfo),
		done:    make(chan struct{}),
		conns:   make(map[uint32]*sutra.Conn),
	}
	return srv, nil
}

func (s *Server) Listen() error {
	socketPath := "/run/dev.avyos.uevent"
	_ = os.Remove(socketPath)
	ln, err := net.Listen("unix", socketPath)
	if err != nil {
		return fmt.Errorf("listen: %w", err)
	}
	go func() {
		defer ln.Close()
		for {
			nc, err := ln.Accept()
			if err != nil {
				select {
				case <-s.done:
					return
				default:
				}
				serviceLog.Error("accept error: %v", err)
				continue
			}
			go s.serveConn(nc)
		}
	}()
	return nil
}

func (s *Server) registerConn(conn *sutra.Conn) uint32 {
	id := s.nextCID.Add(1)
	s.connMu.Lock()
	s.conns[id] = conn
	s.connMu.Unlock()
	return id
}

func (s *Server) unregisterConn(id uint32) {
	s.connMu.Lock()
	delete(s.conns, id)
	s.connMu.Unlock()
}

func (s *Server) serveConn(nc net.Conn) {
	conn := sutra.NewConn(nc)
	defer conn.Close()

	id := s.registerConn(conn)
	defer s.unregisterConn(id)

	for {
		tx, err := conn.Recv()
		if err != nil {
			return
		}
		tx.Object = id
		if err := uevent.Dispatch(uevent.Handlers{Uevent: s}, conn, tx); err != nil {
			return
		}
	}
}

// UeventHandler interface implementation

func (s *Server) GetDevice(_ uint32, in uevent.DeviceInfo) (uevent.DeviceInfo, error) {
	s.deviceMu.RLock()
	dev, ok := s.devices[in.DevPath]
	s.deviceMu.RUnlock()
	if !ok {
		return uevent.DeviceInfo{}, fmt.Errorf("device not found: %s", in.DevPath)
	}
	return dev, nil
}

func (s *Server) ListDevices(_ uint32) (uevent.DeviceList, error) {
	s.deviceMu.RLock()
	devices := make([]uevent.DeviceInfo, 0, len(s.devices))
	for _, dev := range s.devices {
		devices = append(devices, dev)
	}
	s.deviceMu.RUnlock()
	return uevent.DeviceList{Devices: devices}, nil
}

func (s *Server) Trigger(_ uint32, in uevent.TriggerRequest) error {
	go func() {
		subsystem := in.Subsystem
		if subsystem == "" || subsystem == "*" {
			serviceLog.Info("triggering all devices")
			if err := triggerAll(); err != nil {
				serviceLog.Error("trigger all: %v", err)
			}
		} else {
			serviceLog.Info("triggering subsystem: %s", subsystem)
			if err := triggerSubsystem(subsystem); err != nil {
				serviceLog.Error("trigger %s: %v", subsystem, err)
			}
		}
	}()
	return nil
}

func (s *Server) Run() error {
	// Coldplug: trigger existing devices
	go func() {
		serviceLog.Info("starting coldplug scan")
		if err := triggerAll(); err != nil {
			serviceLog.Warn("coldplug: %v", err)
		}
		serviceLog.Info("coldplug scan complete")
	}()

	// Main event loop: read from netlink
	for {
		ev, err := recvUEvent(s.nlFD)
		if err != nil {
			select {
			case <-s.done:
				return nil
			default:
			}
			serviceLog.Error("netlink recv: %v", err)
			continue
		}
		s.processEvent(ev)
	}
}

func (s *Server) processEvent(ev *UEvent) {
	serviceLog.Debug("%s %s [%s] dev=%s", ev.Action, ev.DevPath, ev.Subsystem, ev.DevName)

	s.updateDeviceRegistry(ev)

	matched := false
	for i := range s.rules {
		if matchRule(&s.rules[i], ev) {
			applyRule(&s.rules[i], ev)
			matched = true
			break
		}
	}
	if !matched {
		defaultRule := Rule{Mode: 0660}
		applyRule(&defaultRule, ev)
	}

	s.broadcastEvent(ev)
}

func (s *Server) updateDeviceRegistry(ev *UEvent) {
	major, _ := strconv.Atoi(ev.Major)
	minor, _ := strconv.Atoi(ev.Minor)

	switch ev.Action {
	case "add", "change":
		info := uevent.DeviceInfo{
			DevPath:   ev.DevPath,
			DevName:   ev.DevName,
			Subsystem: ev.Subsystem,
			DevType:   ev.DevType,
			Driver:    ev.Driver,
			Major:     int32(major),
			Minor:     int32(minor),
		}
		s.deviceMu.Lock()
		s.devices[ev.DevPath] = info
		s.deviceMu.Unlock()

	case "remove":
		s.deviceMu.Lock()
		delete(s.devices, ev.DevPath)
		s.deviceMu.Unlock()
	}
}

func (s *Server) broadcastEvent(ev *UEvent) {
	major, _ := strconv.Atoi(ev.Major)
	minor, _ := strconv.Atoi(ev.Minor)

	devEv := uevent.DeviceEvent{
		Action:    ev.Action,
		Subsystem: ev.Subsystem,
		DevPath:   ev.DevPath,
		DevName:   ev.DevName,
		DevType:   ev.DevType,
		Major:     int32(major),
		Minor:     int32(minor),
	}

	var sendFn func(*sutra.Conn, uint32, uevent.DeviceEvent) error
	switch ev.Action {
	case "add":
		sendFn = uevent.SendUeventDeviceAdded
	case "remove":
		sendFn = uevent.SendUeventDeviceRemoved
	case "change":
		sendFn = uevent.SendUeventDeviceChanged
	default:
		return
	}

	s.connMu.RLock()
	conns := make([]*sutra.Conn, 0, len(s.conns))
	for _, c := range s.conns {
		conns = append(conns, c)
	}
	s.connMu.RUnlock()

	for _, c := range conns {
		_ = sendFn(c, 0, devEv)
	}
}

func (s *Server) Close() error {
	close(s.done)
	syscall.Close(s.nlFD)
	return nil
}
