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
	"net"
	"os"
	"syscall"

	"avyos.dev/api/distro"
	"avyos.dev/lib/logger"
	"avyos.dev/lib/sutra"
)

var serviceLog = logger.New("distro")

func main() {
	if len(os.Args) > 1 && os.Args[1] == "init" {
		if err := runInit(); err != nil {
			serviceLog.Error("distro init failed: %v", err)
			os.Exit(1)
		}
		return
	}

	if err := logger.SetupLog(); err != nil {
		serviceLog.Error("failed to setup system log: %v", err)
	}

	socketPath := "/run/dev.avyos.distro"
	_ = os.Remove(socketPath)
	ln, err := net.Listen("unix", socketPath)
	if err != nil {
		serviceLog.Error("failed to listen: %v", err)
		os.Exit(1)
	}
	defer ln.Close()

	h := newHandler()
	serviceLog.Info("distro service ready")

	for {
		nc, err := ln.Accept()
		if err != nil {
			serviceLog.Error("accept error: %v", err)
			return
		}
		go serveDistroConn(nc, h)
	}
}

func serveDistroConn(nc net.Conn, h *Handler) {
	conn := sutra.NewConn(nc)
	defer conn.Close()

	uid := peerUID(nc)
	objectID := h.RegisterConn(conn, uid)
	defer func() {
		h.shells.CloseByOwner(objectID)
		h.UnregisterConn(objectID)
	}()

	for {
		tx, err := conn.Recv()
		if err != nil {
			return
		}
		tx.Object = objectID
		if err := distro.Dispatch(distro.Handlers{Distro: h}, conn, tx); err != nil {
			return
		}
	}
}

func peerUID(nc net.Conn) uint32 {
	uc, ok := nc.(*net.UnixConn)
	if !ok {
		return 0
	}
	raw, err := uc.SyscallConn()
	if err != nil {
		return 0
	}
	var uid uint32
	_ = raw.Control(func(fd uintptr) {
		if cred, err := syscall.GetsockoptUcred(int(fd), syscall.SOL_SOCKET, syscall.SO_PEERCRED); err == nil {
			uid = cred.Uid
		}
	})
	return uid
}
