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
	"os/user"
	"strconv"
	"strings"
	"syscall"

	"avyos.dev/lib/format"
)

func init() {
	flag.Usage = func() {
		fmt.Fprintln(os.Stderr, "identity - User identity and group management")
		fmt.Fprintln(os.Stderr)
		fmt.Fprintln(os.Stderr, "Usage:")
		fmt.Fprintln(os.Stderr, "  identity <subcommand> [options] [args]")
		fmt.Fprintln(os.Stderr)
		fmt.Fprintln(os.Stderr, "Subcommands:")
		fmt.Fprintln(os.Stderr, "  info    Show user information")
		fmt.Fprintln(os.Stderr, "  list    List users from /etc/passwd")
		fmt.Fprintln(os.Stderr, "  switch  Switch to another user")
		fmt.Fprintln(os.Stderr, "  groups  Show user groups")
		fmt.Fprintln(os.Stderr, "  add     Add a new user")
		fmt.Fprintln(os.Stderr, "  passwd  Update user password")
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
		"groups":       cmdGroups,
		"capabilities": cmdGroups,
		"add":          cmdAdd,
		"passwd":       cmdPassword,
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

	var u *user.User
	var err error
	if len(args) > 0 {
		u, err = user.Lookup(args[0])
	} else {
		u, err = user.LookupId(strconv.Itoa(os.Getuid()))
	}
	if err != nil {
		return err
	}

	fmt.Printf("Uid   : %s\n", u.Uid)
	fmt.Printf("Gid   : %s\n", u.Gid)
	fmt.Printf("Name  : %s\n", u.Username)
	fmt.Printf("Full  : %s\n", u.Name)
	fmt.Printf("Home  : %s\n", u.HomeDir)
	fmt.Printf("Shell : %s\n", defaultLoginShell())
	groups, _ := groupNames(u)
	fmt.Printf("Groups\n%s\n", strings.Join(groups, ", "))
	return nil
}

func cmdList(args []string) error {
	fs := flag.NewFlagSet("list", flag.ContinueOnError)
	showSystem := fs.Bool("system", false, "Include system users")
	fs.SetOutput(os.Stderr)
	if err := fs.Parse(args); err != nil {
		return err
	}

	records, err := readPasswdRecords()
	if err != nil {
		return err
	}

	table := format.NewTable("UID", "Name", "Home", "Shell")
	for _, r := range records {
		if !*showSystem && r.UID < 1000 && r.UID != 0 {
			continue
		}
		table.AddRow(fmt.Sprintf("%d", r.UID), r.Name, r.Home, shellOrDefault(r.Shell))
	}
	table.Print()
	return nil
}

func cmdSwitch(args []string) error {
	fs := flag.NewFlagSet("switch", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	if err := fs.Parse(args); err != nil {
		return err
	}
	args = fs.Args()

	if len(args) < 1 {
		return fmt.Errorf("usage: identity switch <username>")
	}
	if os.Getuid() != 0 {
		return fmt.Errorf("switching users requires root privileges")
	}

	u, err := user.Lookup(args[0])
	if err != nil {
		return err
	}
	uid, err := strconv.Atoi(u.Uid)
	if err != nil {
		return err
	}
	gid, err := strconv.Atoi(u.Gid)
	if err != nil {
		return err
	}
	groups, err := groupIDs(u)
	if err == nil {
		_ = syscall.Setgroups(groups)
	}

	home := u.HomeDir
	if home == "" {
		home = "/"
	}
	_ = os.Chdir(home)
	os.Setenv("HOME", home)
	os.Setenv("USER", u.Username)
	os.Setenv("LOGNAME", u.Username)

	if err := syscall.Setgid(gid); err != nil {
		return fmt.Errorf("failed to set GID: %w", err)
	}
	if err := syscall.Setuid(uid); err != nil {
		return fmt.Errorf("failed to set UID: %w", err)
	}

	shell := defaultLoginShell()
	format.Success("Switched to user %s", u.Username)
	return syscall.Exec(shell, []string{shell}, os.Environ())
}

func cmdGroups(args []string) error {
	fs := flag.NewFlagSet("groups", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	if err := fs.Parse(args); err != nil {
		return err
	}
	args = fs.Args()

	var u *user.User
	var err error
	if len(args) > 0 {
		u, err = user.Lookup(args[0])
	} else {
		u, err = user.LookupId(strconv.Itoa(os.Getuid()))
	}
	if err != nil {
		return err
	}

	groups, err := groupNames(u)
	if err != nil {
		return err
	}
	fmt.Printf("%s : %s\n", u.Username, strings.Join(groups, " "))
	return nil
}

func cmdAdd(args []string) error {
	fs := flag.NewFlagSet("add", flag.ContinueOnError)
	kind := fs.String("kind", "user", "Kind of user; currently user-compatible IDs are used")
	home := fs.String("home", "", "Home directory")
	shell := fs.String("shell", defaultLoginShell(), "Default shell")
	fullName := fs.String("name", "", "Full name/GECOS")
	groups := fs.String("groups", "users", "Comma-separated supplementary groups")
	fs.SetOutput(os.Stderr)
	if err := fs.Parse(args); err != nil {
		return err
	}
	args = fs.Args()
	_ = kind

	if len(args) == 0 {
		return fmt.Errorf("no username provided")
	}

	username := args[0]
	userHome := *home
	if userHome == "" {
		userHome = "/home/" + username
	}
	return addLoginAccount(loginAccountSpec{
		Username: username,
		FullName: *fullName,
		Groups:   splitList(*groups),
		Home:     userHome,
		Shell:    *shell,
	})
}

func cmdPassword(args []string) error {
	fs := flag.NewFlagSet("passwd", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	if err := fs.Parse(args); err != nil {
		return err
	}
	args = fs.Args()

	if len(args) < 2 {
		return fmt.Errorf("usage: identity passwd <USER> <PASSWORD>")
	}
	return updateLoginPassword(args[0], args[1])
}

func groupNames(u *user.User) ([]string, error) {
	raw, err := u.GroupIds()
	if err != nil {
		return nil, err
	}
	names := make([]string, 0, len(raw))
	seen := map[string]struct{}{}
	for _, gid := range raw {
		group, err := user.LookupGroupId(gid)
		name := gid
		if err == nil && group.Name != "" {
			name = group.Name
		}
		if _, ok := seen[name]; ok {
			continue
		}
		seen[name] = struct{}{}
		names = append(names, name)
	}
	return names, nil
}

func groupIDs(u *user.User) ([]int, error) {
	raw, err := u.GroupIds()
	if err != nil {
		return nil, err
	}
	ids := make([]int, 0, len(raw))
	for _, value := range raw {
		id, err := strconv.Atoi(value)
		if err == nil {
			ids = append(ids, id)
		}
	}
	return ids, nil
}
