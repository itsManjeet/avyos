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
	"strings"
	"syscall"

	"avyos.dev/pkg/format"
	"avyos.dev/pkg/fs"
	"avyos.dev/pkg/identity"
)

func init() {
	flag.Usage = func() {
		fmt.Fprintln(os.Stderr, "identity - User identity and capability management")
		fmt.Fprintln(os.Stderr)
		fmt.Fprintln(os.Stderr, "Usage:")
		fmt.Fprintln(os.Stderr, "  identity <subcommand> [options] [args]")
		fmt.Fprintln(os.Stderr)
		fmt.Fprintln(os.Stderr, "Subcommands:")
		fmt.Fprintln(os.Stderr, "  info         Show identity information")
		fmt.Fprintln(os.Stderr, "  list         List identities")
		fmt.Fprintln(os.Stderr, "  switch       Switch to another identity")
		fmt.Fprintln(os.Stderr, "  capabilities Show identity capabilities")
		fmt.Fprintln(os.Stderr, "  add          Add a new identity")
		fmt.Fprintln(os.Stderr, "  password     Update identity password")
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
		"info":         cmdInfo,
		"list":         cmdList,
		"switch":       cmdSwitch,
		"capabilities": cmdCapabilities,
		"add":          cmdAdd,
		"password":     cmdPassword,
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

func cmdInfo(args []string) error {
	fs := flag.NewFlagSet("info", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	if err := fs.Parse(args); err != nil {
		return err
	}
	args = fs.Args()

	var id *identity.Identity
	var err error
	if len(args) > 0 {
		id, err = identity.LookupByName(args[0])
	} else {
		id, err = identity.LookupByID(os.Getuid())
	}
	if err != nil {
		return err
	}

	fmt.Printf("Id    : %d\n", id.ID)
	fmt.Printf("Name  : %s\n", id.Name)
	fmt.Printf("Home  : %s\n", id.Home)
	fmt.Printf("Shell : %s\n", id.Shell)
	fmt.Printf("Capabilities\n%s\n", strings.Join(id.Capabilities, ", "))
	return nil
}

func cmdList(args []string) error {
	fs := flag.NewFlagSet("list", flag.ContinueOnError)
	showSystem := fs.Bool("system", false, "Include system identities")
	fs.SetOutput(os.Stderr)
	if err := fs.Parse(args); err != nil {
		return err
	}

	identities, err := identity.LoadIdentityConfig()
	if err != nil {
		return err
	}

	table := format.NewTable("ID", "Name", "Home", "Shell")
	for _, u := range identities.Identities {
		if !*showSystem && u.ID < 1000 && u.ID != 0 {
			continue
		}
		table.AddRow(fmt.Sprintf("%d", u.ID), u.Name, u.Home, u.Shell)
	}
	table.Print()
	return nil
}

func cmdSwitch(args []string) error {
	flagSet := flag.NewFlagSet("switch", flag.ContinueOnError)
	flagSet.SetOutput(os.Stderr)
	if err := flagSet.Parse(args); err != nil {
		return err
	}
	args = flagSet.Args()

	if len(args) < 1 {
		return fmt.Errorf("usage: identity switch <username>")
	}

	username := args[0]
	id, err := identity.LookupByName(username)
	if err != nil {
		return err
	}

	if os.Getuid() != 0 {
		return fmt.Errorf("switching users requires root privileges")
	}

	if err := os.Chdir(id.Home); err != nil {
	}

	os.Setenv("HOME", id.Home)
	os.Setenv("USER", id.Name)
	os.Setenv("LOGNAME", id.Name)

	if err := syscall.Setgid(int(id.ID)); err != nil {
		return fmt.Errorf("failed to set GID: %w", err)
	}
	if err := syscall.Setuid(int(id.ID)); err != nil {
		return fmt.Errorf("failed to set UID: %w", err)
	}

	shell := id.Shell
	if shell == "" {
		shell = fs.Resolve("cmd:shell")
	}

	format.Success("Switched to user %s", username)
	return syscall.Exec(shell, []string{shell}, os.Environ())
}

func cmdCapabilities(args []string) error {
	fs := flag.NewFlagSet("capabilities", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	if err := fs.Parse(args); err != nil {
		return err
	}
	args = fs.Args()

	var id *identity.Identity
	var err error
	if len(args) > 0 {
		id, err = identity.LookupByName(args[0])
	} else {
		id, err = identity.LookupByID(os.Getuid())
	}
	if err != nil {
		return err
	}

	fmt.Printf("%s : %s\n", id.Name, strings.Join(id.Capabilities, " "))
	return nil
}

func cmdAdd(args []string) error {
	fs := flag.NewFlagSet("add", flag.ContinueOnError)
	kind := fs.String("kind", "user", "Kind of identity")
	_ = fs.Int("id", 0, "User ID")
	home := fs.String("home", "", "Home directory")
	shell := fs.String("shell", "/avyos/cmd/shell", "Default shell")
	fs.SetOutput(os.Stderr)
	if err := fs.Parse(args); err != nil {
		return err
	}
	args = fs.Args()

	if len(args) == 0 {
		return fmt.Errorf("no username provided")
	}

	id := identity.Identity{Name: args[0], Shell: *shell}
	if *home == "" {
		id.Home = "/users/" + id.Name
	} else {
		id.Home = *home
	}

	if err := identity.AddIdentity(id, *kind); err != nil {
		return err
	}
	return nil
}

func cmdPassword(args []string) error {
	fs := flag.NewFlagSet("password", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	if err := fs.Parse(args); err != nil {
		return err
	}
	args = fs.Args()

	if len(args) < 2 {
		return fmt.Errorf("usage: identity password <ID> <PASSWORD>")
	}
	if err := identity.UpdatePassword(args[0], "", args[1]); err != nil {
		return err
	}
	return nil
}
