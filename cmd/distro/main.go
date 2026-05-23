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
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"avyos.dev/api/distro"
	"avyos.dev/pkg/format"
	"avyos.dev/pkg/term"
)

const defaultShell = "/bin/sh"

func init() {
	flag.Usage = func() {
		fmt.Fprintln(os.Stderr, "distro - Manage Linux distro environment")
		fmt.Fprintln(os.Stderr)
		fmt.Fprintln(os.Stderr, "Usage:")
		fmt.Fprintln(os.Stderr, "  distro <subcommand> [options] [args]")
		fmt.Fprintln(os.Stderr)
		fmt.Fprintln(os.Stderr, "Subcommands:")
		fmt.Fprintln(os.Stderr, "  status   Show distro installation status")
		fmt.Fprintln(os.Stderr, "  install  Install the distro rootfs")
		fmt.Fprintln(os.Stderr, "  run      Run command in distro")
		fmt.Fprintln(os.Stderr, "  shell    Open interactive shell in distro")
		fmt.Fprintln(os.Stderr, "  remove   Remove installed distro")
		fmt.Fprintln(os.Stderr)
		fmt.Fprintln(os.Stderr, "Exit Codes:")
		fmt.Fprintln(os.Stderr, "  0  Success")
		fmt.Fprintln(os.Stderr, "  1  Runtime/command error")
		fmt.Fprintln(os.Stderr, "  2  Invalid flags/usage")
	}
}

func main() {
	flag.Parse()
	args := flag.Args()

	commands := map[string]func(args []string) error{
		"status":  runStatus,
		"install": runInstall,
		"run":     runRun,
		"shell":   runShell,
		"remove":  runRemove,
	}

	if len(args) < 1 {
		flag.Usage()
		os.Exit(1)
	}

	cmd, ok := commands[args[0]]
	if !ok {
		format.Error("unknown subcommand: %s", args[0])
		os.Exit(1)
	}

	if err := cmd(args[1:]); err != nil {
		format.Error("%s", err)
		os.Exit(1)
	}
}

func runStatus(args []string) error {
	fs := flag.NewFlagSet("status", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	if err := fs.Parse(args); err != nil {
		return err
	}

	client, err := distro.Connect()
	if err != nil {
		return fmt.Errorf("cannot connect to distro service: %w", err)
	}
	defer client.Close()

	status, err := client.GetStatus()
	if err != nil {
		return err
	}

	if status.Installed != 0 {
		fmt.Printf("Installed: yes\n")
		fmt.Printf("Path:      %s\n", status.Path)
		fmt.Printf("Size:      %s\n", format.Size(int64(status.Size)))
	} else {
		fmt.Printf("Installed: no\n")
		fmt.Printf("Path:      %s\n", status.Path)
		fmt.Println("Use 'distro install' to install the distro rootfs.")
	}
	return nil
}

func runInstall(args []string) error {
	fs := flag.NewFlagSet("install", flag.ContinueOnError)
	url := fs.String("url", "", "Custom URL to download from")
	fs.SetOutput(os.Stderr)
	if err := fs.Parse(args); err != nil {
		return err
	}

	client, err := distro.Connect()
	if err != nil {
		return fmt.Errorf("cannot connect to distro service: %w", err)
	}
	defer client.Close()

	if err := client.InstallDistro(*url); err != nil {
		return err
	}

	fmt.Println("Distro installed successfully.")
	return nil
}

func runRun(args []string) error {
	fs := flag.NewFlagSet("run", flag.ContinueOnError)
	bind := fs.String("bind", "", "Bind mount(s) (host:distro[,host:distro])")
	env := fs.String("env", "", "Set environment variable (KEY=VALUE)")
	workdir := fs.String("workdir", "/", "Set working directory")
	fs.SetOutput(os.Stderr)
	if err := fs.Parse(args); err != nil {
		return err
	}
	args = fs.Args()

	input, piped, err := readPipedStdin()
	if err != nil {
		return fmt.Errorf("failed to read stdin: %w", err)
	}

	command := args
	if len(command) == 0 {
		if !piped {
			return fmt.Errorf("interactive sessions require 'distro shell'; use a command or pipe input for 'distro run'")
		}
		command = []string{defaultShell}
	}

	client, err := distro.Connect()
	if err != nil {
		return fmt.Errorf("cannot connect to distro service: %w", err)
	}
	defer client.Close()

	result, err := client.RunDistro(distro.RunRequest{
		Command: distro.EncodeCommand(command),
		Workdir: *workdir,
		Bind:    *bind,
		Env:     *env,
		Input:   input,
	})
	if err != nil {
		return err
	}

	printRunOutput(result)
	if result.ExitCode != 0 {
		return fmt.Errorf("distro command exited with code %d", result.ExitCode)
	}
	return nil
}

func runShell(args []string) error {
	fs := flag.NewFlagSet("shell", flag.ContinueOnError)
	bind := fs.String("bind", "", "Bind mount(s) (host:distro[,host:distro])")
	env := fs.String("env", "", "Set environment variable (KEY=VALUE)")
	fs.SetOutput(os.Stderr)
	if err := fs.Parse(args); err != nil {
		return err
	}

	client, err := distro.Connect()
	if err != nil {
		return fmt.Errorf("cannot connect to distro service: %w", err)
	}
	defer client.Close()

	cols, rows := term.Size()
	sessionID, err := client.OpenShell(distro.ShellOpenRequest{
		Workdir: "/root",
		Bind:    *bind,
		Env:     *env,
		Rows:    int32(rows),
		Cols:    int32(cols),
	})
	if err != nil {
		return err
	}
	defer func() { _ = client.CloseShell(sessionID) }()

	outputCh := make(chan []byte, 128)
	exitCh := make(chan int32, 1)
	disconnectCh := make(chan struct{}, 1)
	inputErrCh := make(chan error, 1)

	client.OnShellOutputEvent(func(ev distro.ShellOutputEvent) {
		if ev.SessionID != sessionID || len(ev.Data) == 0 {
			return
		}
		data := make([]byte, len(ev.Data))
		copy(data, ev.Data)
		select {
		case outputCh <- data:
		default:
		}
	})
	client.OnShellExitEvent(func(ev distro.ShellExitEvent) {
		if ev.SessionID != sessionID {
			return
		}
		select {
		case exitCh <- ev.ExitCode:
		default:
		}
	})
	go func() {
		for {
			tx, err := client.Raw().Recv()
			if err != nil {
				select {
				case disconnectCh <- struct{}{}:
				default:
				}
				return
			}
			if !client.Raw().Route(tx) {
				_ = client.HandleEvent(tx)
			}
		}
	}()

	if term.IsTerminal(syscall.Stdin) {
		if err := term.EnableRawMode(); err == nil {
			defer term.DisableRawMode()
		}
	}

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGWINCH)
	defer signal.Stop(sigCh)
	go func() {
		for range sigCh {
			c, r := term.Size()
			_ = client.ResizeShell(sessionID, r, c)
		}
	}()
	_ = client.ResizeShell(sessionID, rows, cols)

	go func() {
		buf := make([]byte, 1024)
		for {
			n, err := os.Stdin.Read(buf)
			if n > 0 {
				if writeErr := client.SendShellInput(sessionID, buf[:n]); writeErr != nil {
					inputErrCh <- writeErr
					return
				}
			}
			if err != nil {
				if err == io.EOF {
					inputErrCh <- nil
				} else {
					inputErrCh <- err
				}
				return
			}
		}
	}()

	stdinClosed := false
	for {
		select {
		case data := <-outputCh:
			if len(data) > 0 {
				_, _ = os.Stdout.Write(data)
			}
		case code := <-exitCh:
			if code != 0 {
				return fmt.Errorf("distro shell exited with code %d", code)
			}
			return nil
		case <-disconnectCh:
			return fmt.Errorf("distro service disconnected")
		case inputErr := <-inputErrCh:
			if inputErr != nil {
				return fmt.Errorf("shell input failed: %w", inputErr)
			}
			if !stdinClosed {
				stdinClosed = true
				_ = client.CloseShell(sessionID)
			}
		}
	}
}

func runRemove(args []string) error {
	fs := flag.NewFlagSet("remove", flag.ContinueOnError)
	force := fs.Bool("force", false, "Force removal without confirmation")
	fs.SetOutput(os.Stderr)
	if err := fs.Parse(args); err != nil {
		return err
	}

	if !*force {
		fmt.Print("Remove the distro? This cannot be undone. [y/N]: ")
		var response string
		fmt.Scanln(&response)
		if strings.ToLower(response) != "y" {
			fmt.Println("Cancelled.")
			return nil
		}
	}

	client, err := distro.Connect()
	if err != nil {
		return fmt.Errorf("cannot connect to distro service: %w", err)
	}
	defer client.Close()

	if err := client.Uninstall(); err != nil {
		return err
	}

	fmt.Println("Distro removed.")
	return nil
}

func readPipedStdin() ([]byte, bool, error) {
	info, err := os.Stdin.Stat()
	if err != nil {
		return nil, false, err
	}
	if info.Mode()&os.ModeCharDevice != 0 {
		return nil, false, nil
	}

	input, err := io.ReadAll(os.Stdin)
	if err != nil {
		return nil, true, err
	}
	return input, true, nil
}

func printRunOutput(result distro.RunResult) {
	if len(result.Stdout) > 0 {
		_, _ = os.Stdout.Write(result.Stdout)
	}
	if len(result.Stderr) > 0 {
		_, _ = os.Stderr.Write(result.Stderr)
	}
}
