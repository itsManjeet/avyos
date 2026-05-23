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
	"os"
	"syscall"

	"avyos.dev/api/login"
	"avyos.dev/api/service"
	"avyos.dev/pkg/format"
)

func init() {
	flag.Usage = func() {
		fmt.Fprintln(os.Stderr, "power - Power and session actions")
		fmt.Fprintln(os.Stderr)
		fmt.Fprintln(os.Stderr, "Usage:")
		fmt.Fprintln(os.Stderr, "  power <subcommand> [options]")
		fmt.Fprintln(os.Stderr)
		fmt.Fprintln(os.Stderr, "Subcommands:")
		fmt.Fprintln(os.Stderr, "  off     Shutdown the system")
		fmt.Fprintln(os.Stderr, "  reboot  Reboot the system")
		fmt.Fprintln(os.Stderr, "  logout  Logout active graphical session")
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
		"off":    cmdOff,
		"reboot": cmdReboot,
		"logout": cmdLogout,
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

func cmdOff(args []string) error {
	fs := flag.NewFlagSet("off", flag.ContinueOnError)
	force := fs.Bool("force", false, "Force immediate shutdown")
	fs.SetOutput(os.Stderr)
	if err := fs.Parse(args); err != nil {
		return err
	}

	if *force {
		return syscall.Reboot(syscall.LINUX_REBOOT_CMD_POWER_OFF)
	}

	fmt.Println("Shutting down...")
	client, err := service.Connect()
	if err != nil {
		return fmt.Errorf("cannot connect to service manager: %w (use --force only for emergency)", err)
	}
	defer client.Close()

	if err := client.Poweroff(); err != nil {
		return fmt.Errorf("poweroff request failed: %w (use --force only for emergency)", err)
	}
	return nil
}

func cmdReboot(args []string) error {
	fs := flag.NewFlagSet("reboot", flag.ContinueOnError)
	force := fs.Bool("force", false, "Force immediate reboot")
	fs.SetOutput(os.Stderr)
	if err := fs.Parse(args); err != nil {
		return err
	}

	if *force {
		return syscall.Reboot(syscall.LINUX_REBOOT_CMD_RESTART)
	}

	fmt.Println("Rebooting...")
	client, err := service.Connect()
	if err != nil {
		return fmt.Errorf("cannot connect to service manager: %w (use --force only for emergency)", err)
	}
	defer client.Close()

	if err := client.Reboot(); err != nil {
		return fmt.Errorf("reboot request failed: %w (use --force only for emergency)", err)
	}
	return nil
}

func cmdLogout(args []string) error {
	fs := flag.NewFlagSet("logout", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	if err := fs.Parse(args); err != nil {
		return err
	}

	fmt.Println("Logging out...")
	client, err := login.Connect()
	if err != nil {
		return fmt.Errorf("cannot connect to login service: %w", err)
	}
	defer client.Close()

	if err := client.Logout(); err != nil {
		return fmt.Errorf("logout failed: %w", err)
	}
	return nil
}
