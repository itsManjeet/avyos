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
	"errors"
	"flag"
	"fmt"
	"io"
	"net"
	"os"
	"strconv"
	"strings"

	"avyos.dev/api/dbg"
	"avyos.dev/pkg/logger"
	"avyos.dev/pkg/sutra"
)

var serviceLog = logger.New("dev.avyos.dbg")

var (
	listenAddr   = flag.String("listen", ":5037", "TCP listen address for dbg daemon")
	maxOutput    = flag.Int("max-output", defaultMaxOutputBytes, "Maximum stdout/stderr bytes captured per command stream")
	helperMode   = flag.String("dbg-helper", "", "Internal helper mode (read|write)")
	helperPath   = flag.String("dbg-path", "", "Internal helper file path")
	helperOffset = flag.Uint64("dbg-offset", 0, "Internal helper offset")
	helperSize   = flag.Uint("dbg-size", 0, "Internal helper read size")
	helperTrunc  = flag.Bool("dbg-truncate", false, "Internal helper truncate mode")
	helperPerm   = flag.Uint("dbg-mode", 0644, "Internal helper file mode")
)

func init() {
	flag.Usage = func() {
		fmt.Fprintln(os.Stderr, "dbgd - avyos remote debugging daemon")
		fmt.Fprintln(os.Stderr)
		fmt.Fprintln(os.Stderr, "Usage:")
		fmt.Fprintln(os.Stderr, "  dbgd [--listen=:5037] [--max-output=24576]")
		fmt.Fprintln(os.Stderr)
		fmt.Fprintln(os.Stderr, "Exit Codes:")
		fmt.Fprintln(os.Stderr, "  0  Success")
		fmt.Fprintln(os.Stderr, "  1  Runtime/service error")
		fmt.Fprintln(os.Stderr, "  2  Invalid flags/usage")
	}
}

func main() {
	flag.Parse()

	if strings.TrimSpace(*helperMode) != "" {
		if err := runHelper(strings.TrimSpace(*helperMode)); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		return
	}

	if err := logger.SetupLog(); err != nil {
		serviceLog.Error("failed to setup system log: %v", err)
	}

	ln, err := net.Listen("tcp", strings.TrimSpace(*listenAddr))
	if err != nil {
		serviceLog.Error("failed to start TCP listener: %v", err)
		os.Exit(1)
	}
	defer ln.Close()

	h := NewHandler(*maxOutput, nil)
	h.shells = newShellSessionManager(h)

	serviceLog.Info("dbgd listening on %s", strings.TrimSpace(*listenAddr))
	for {
		nc, err := ln.Accept()
		if err != nil {
			serviceLog.Error("accept error: %v", err)
			return
		}
		go serveConn(nc, h)
	}
}

func runHelper(mode string) error {
	mode = strings.ToLower(strings.TrimSpace(mode))
	path := strings.TrimSpace(*helperPath)
	if path == "" {
		return errors.New("helper path is required")
	}

	switch mode {
	case "read":
		size := int(*helperSize)
		if size <= 0 {
			size = defaultFileChunkBytes
		}
		if size > defaultFileChunkBytes {
			size = defaultFileChunkBytes
		}
		buf := make([]byte, size)
		f, err := os.Open(path)
		if err != nil {
			return err
		}
		defer f.Close()

		n, err := f.ReadAt(buf, int64(*helperOffset))
		if err != nil && !errors.Is(err, io.EOF) {
			return err
		}
		if n <= 0 {
			return nil
		}
		_, err = os.Stdout.Write(buf[:n])
		return err

	case "write":
		flags := os.O_CREATE | os.O_WRONLY
		if *helperTrunc {
			flags |= os.O_TRUNC
		}

		perm := os.FileMode(*helperPerm)
		if perm == 0 {
			perm = 0644
		}
		f, err := os.OpenFile(path, flags, perm)
		if err != nil {
			return err
		}
		defer f.Close()

		if !*helperTrunc || *helperOffset > 0 {
			if _, err := f.Seek(int64(*helperOffset), io.SeekStart); err != nil {
				return err
			}
		}

		n, err := io.Copy(f, os.Stdin)
		if err != nil {
			return err
		}
		_, err = io.WriteString(os.Stdout, strconv.FormatInt(n, 10))
		return err

	default:
		return fmt.Errorf("unknown helper mode: %s", mode)
	}
}

func serveConn(nc net.Conn, h *Handler) {
	conn := sutra.NewConn(nc)
	defer conn.Close()

	clientID := h.RegisterConn(conn)
	defer func() {
		h.DropClientSessions(clientID)
		h.UnregisterConn(clientID)
	}()

	for {
		tx, err := conn.Recv()
		if err != nil {
			return
		}
		tx.Object = clientID
		if err := dbg.Dispatch(dbg.Handlers{Dbg: h}, conn, tx); err != nil {
			return
		}
	}
}
