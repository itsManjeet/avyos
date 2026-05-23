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
	"log"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"
)

type child struct {
	name string
	cmd  *exec.Cmd
}

func main() {
	log.SetPrefix("session: ")
	log.SetFlags(0)

	// Session components in start order.
	// Daemons run in the background; the last two (background, dock) are
	// the critical desktop processes — if either exits the session ends.
	type component struct {
		name     string
		args     []string
		daemon   bool // daemon = don't wait on exit
		optional bool // optional = warn on failure, continue
	}

	components := []component{
		{name: "settings", args: []string{"--daemon"}, daemon: true, optional: true},
		{name: "waylayer", args: []string{"--daemon"}, daemon: true, optional: true},
		{name: "background", args: []string{"--mode", "layer"}},
		{name: "dock"},
	}

	// Set deterministic Wayland runtime/socket defaults for the session.
	runtimeDir := fmt.Sprintf("/run/user/%d", os.Getuid())
	_ = os.Setenv("XDG_RUNTIME_DIR", runtimeDir)
	_ = os.Setenv("WAYLAND_DISPLAY", "waylayer")
	_ = os.Setenv("avyos_SESSION_MODE", "1")

	var children []*child
	var critical []*child

	for _, comp := range components {
		path := filepath.Join("/usr/apps", comp.name, "exec")
		cmd := exec.Command(path, comp.args...)
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		cmd.Dir = os.Getenv("HOME")
		cmd.Env = os.Environ()
		// Start each component in its own process group so terminal SIGINT
		// (Ctrl+C) on the parent shell does not tear down the full desktop.
		cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

		if err := cmd.Start(); err != nil {
			if comp.optional {
				log.Printf("warning: failed to start %s: %v", comp.name, err)
				continue
			}
			// Critical component failed — tear down everything started so far
			log.Printf("failed to start %s: %v", comp.name, err)
			stopAll(children)
			os.Exit(1)
		}

		log.Printf("started %s pid=%d", comp.name, cmd.Process.Pid)
		c := &child{name: comp.name, cmd: cmd}
		children = append(children, c)

		if !comp.daemon {
			critical = append(critical, c)
		}
	}

	if len(critical) == 0 {
		log.Printf("no critical components running, exiting")
		stopAll(children)
		os.Exit(1)
	}

	// Wait for any critical component to exit, or a signal
	exits := make(chan *child, len(critical))
	for _, c := range critical {
		go func() {
			c.cmd.Wait()
			exits <- c
		}()
	}

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP, syscall.SIGQUIT)

	for {
		select {
		case sig := <-sigCh:
			// Ignore interactive shell/tty signals for session robustness.
			if sig == syscall.SIGINT || sig == syscall.SIGHUP || sig == syscall.SIGQUIT {
				log.Printf("ignoring %v", sig)
				continue
			}
			log.Printf("received %v", sig)
			signal.Stop(sigCh)
			stopAll(children)
			return
		case c := <-exits:
			log.Printf("%s exited", c.name)
			signal.Stop(sigCh)
			stopAll(children)
			return
		}
	}
}

func stopAll(children []*child) {
	// Stop in reverse order
	for i := len(children) - 1; i >= 0; i-- {
		stopChild(children[i])
	}
}

func stopChild(c *child) {
	if c.cmd == nil || c.cmd.Process == nil {
		return
	}
	_ = c.cmd.Process.Signal(syscall.SIGTERM)

	done := make(chan struct{})
	go func() {
		c.cmd.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		log.Printf("forcing stop for %s pid=%d", c.name, c.cmd.Process.Pid)
		_ = c.cmd.Process.Signal(syscall.SIGKILL)
	}
}
