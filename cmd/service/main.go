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
	"sort"
	"strconv"

	"avyos.dev/api/service"
	"avyos.dev/pkg/format"
)

func init() {
	flag.Usage = func() {
		fmt.Fprintln(os.Stderr, "service - Manage init services")
		fmt.Fprintln(os.Stderr)
		fmt.Fprintln(os.Stderr, "Usage:")
		fmt.Fprintln(os.Stderr, "  service <subcommand> [args]")
		fmt.Fprintln(os.Stderr)
		fmt.Fprintln(os.Stderr, "Subcommands:")
		fmt.Fprintln(os.Stderr, "  list     List all services")
		fmt.Fprintln(os.Stderr, "  status   Show status for one service")
		fmt.Fprintln(os.Stderr, "  start    Start one or more services")
		fmt.Fprintln(os.Stderr, "  stop     Stop one or more services")
		fmt.Fprintln(os.Stderr, "  restart  Restart one or more services")
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
		"list":    cmdList,
		"status":  cmdStatus,
		"start":   cmdStart,
		"stop":    cmdStop,
		"restart": cmdRestart,
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

func cmdList(args []string) error {
	fs := flag.NewFlagSet("list", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	if err := fs.Parse(args); err != nil {
		return err
	}

	client, err := service.Connect()
	if err != nil {
		return fmt.Errorf("cannot connect to service manager: %w", err)
	}
	defer client.Close()

	statuses, err := client.List()
	if err != nil {
		return fmt.Errorf("failed to list services: %w", err)
	}

	sort.Slice(statuses, func(i, j int) bool {
		return statuses[i].Name < statuses[j].Name
	})

	table := format.NewTable("Name", "State", "PID", "Type", "Restart", "Description")
	for _, s := range statuses {
		pid := "-"
		if s.PID > 0 {
			pid = strconv.Itoa(int(s.PID))
		}
		table.AddRow(s.Name, stateOf(s), pid, s.Type, s.Restart, s.Description)
	}
	table.Print()
	return nil
}

func cmdStatus(args []string) error {
	fs := flag.NewFlagSet("status", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	if err := fs.Parse(args); err != nil {
		return err
	}
	args = fs.Args()

	if len(args) < 1 {
		return fmt.Errorf("usage: service status <name>")
	}

	client, err := service.Connect()
	if err != nil {
		return fmt.Errorf("cannot connect to service manager: %w", err)
	}
	defer client.Close()

	status, err := client.Status(args[0])
	if err != nil {
		return fmt.Errorf("failed to get status for %s: %w", args[0], err)
	}

	fmt.Printf("Name:        %s\n", status.Name)
	fmt.Printf("Description: %s\n", status.Description)
	fmt.Printf("State:       %s\n", stateOf(status))
	fmt.Printf("Type:        %s\n", status.Type)
	fmt.Printf("Restart:     %s\n", status.Restart)
	if status.PID > 0 {
		fmt.Printf("PID:         %d\n", status.PID)
	}
	return nil
}

func cmdStart(args []string) error {
	fs := flag.NewFlagSet("start", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	if err := fs.Parse(args); err != nil {
		return err
	}
	return runServiceAction(fs.Args(), "start", func(c *service.Client, name string) error { return c.Start(name) })
}

func cmdStop(args []string) error {
	fs := flag.NewFlagSet("stop", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	if err := fs.Parse(args); err != nil {
		return err
	}
	return runServiceAction(fs.Args(), "stop", func(c *service.Client, name string) error { return c.Stop(name) })
}

func cmdRestart(args []string) error {
	fs := flag.NewFlagSet("restart", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	if err := fs.Parse(args); err != nil {
		return err
	}
	return runServiceAction(fs.Args(), "restart", func(c *service.Client, name string) error { return c.Restart(name) })
}

func runServiceAction(args []string, verb string, fn func(*service.Client, string) error) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: service %s <name> [name...]", verb)
	}

	client, err := service.Connect()
	if err != nil {
		return fmt.Errorf("cannot connect to service manager: %w", err)
	}
	defer client.Close()

	for _, name := range args {
		if err := fn(client, name); err != nil {
			return fmt.Errorf("%s %s: %w", verb, name, err)
		}
		format.Success("%s %s", actionPastTense(verb), name)
	}
	return nil
}

func actionPastTense(verb string) string {
	switch verb {
	case "start":
		return "started"
	case "stop":
		return "stopped"
	case "restart":
		return "restarted"
	default:
		return verb + "ed"
	}
}

func stateOf(s service.ServiceStatus) string {
	switch {
	case s.Running != 0:
		return "running"
	case s.Failed != 0:
		return "failed"
	case s.Started != 0:
		return "started"
	default:
		return "stopped"
	}
}
