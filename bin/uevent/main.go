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
	"os/signal"
	"syscall"

	"avyos.dev/api/uevent"
	"avyos.dev/lib/format"
)

func init() {
	flag.Usage = func() {
		fmt.Fprintln(os.Stderr, "uevent - Device event and metadata management")
		fmt.Fprintln(os.Stderr)
		fmt.Fprintln(os.Stderr, "Usage:")
		fmt.Fprintln(os.Stderr, "  uevent <subcommand> [options] [args]")
		fmt.Fprintln(os.Stderr)
		fmt.Fprintln(os.Stderr, "Subcommands:")
		fmt.Fprintln(os.Stderr, "  list     List known devices")
		fmt.Fprintln(os.Stderr, "  info     Show details for a device path")
		fmt.Fprintln(os.Stderr, "  monitor  Stream add/remove/change events")
		fmt.Fprintln(os.Stderr, "  trigger  Request device rescan")
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
		"info":    cmdInfo,
		"monitor": cmdMonitor,
		"trigger": cmdTrigger,
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
	subsystem := fs.String("subsystem", "", "Filter by subsystem")
	fs.SetOutput(os.Stderr)
	if err := fs.Parse(args); err != nil {
		return err
	}

	client, err := uevent.Connect()
	if err != nil {
		return fmt.Errorf("cannot connect to uevent service: %w", err)
	}
	defer client.Close()

	list, err := client.Uevent.ListDevices()
	if err != nil {
		return fmt.Errorf("failed to list devices: %w", err)
	}

	table := format.NewTable("Device", "Name", "Subsystem", "Type", "Driver", "Major", "Minor")
	for _, dev := range list.Devices {
		if *subsystem != "" && dev.Subsystem != *subsystem {
			continue
		}
		table.AddRow(
			dev.DevPath,
			dev.DevName,
			dev.Subsystem,
			dev.DevType,
			dev.Driver,
			fmt.Sprintf("%d", dev.Major),
			fmt.Sprintf("%d", dev.Minor),
		)
	}
	table.Print()

	return nil
}

func cmdInfo(args []string) error {
	fs := flag.NewFlagSet("info", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	if err := fs.Parse(args); err != nil {
		return err
	}
	args = fs.Args()

	if len(args) < 1 {
		return fmt.Errorf("usage: uevent info <devpath>")
	}

	client, err := uevent.Connect()
	if err != nil {
		return fmt.Errorf("cannot connect to uevent service: %w", err)
	}
	defer client.Close()

	dev, err := client.Uevent.GetDevice(uevent.DeviceInfo{DevPath: args[0]})
	if err != nil {
		return fmt.Errorf("device not found: %w", err)
	}

	fmt.Printf("Device:      %s\n", dev.DevPath)
	fmt.Printf("Name:        %s\n", dev.DevName)
	fmt.Printf("Subsystem:   %s\n", dev.Subsystem)
	if dev.DevType != "" {
		fmt.Printf("Type:        %s\n", dev.DevType)
	}
	if dev.Driver != "" {
		fmt.Printf("Driver:      %s\n", dev.Driver)
	}
	fmt.Printf("Major:       %d\n", dev.Major)
	fmt.Printf("Minor:       %d\n", dev.Minor)
	return nil
}

func cmdMonitor(args []string) error {
	fs := flag.NewFlagSet("monitor", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	if err := fs.Parse(args); err != nil {
		return err
	}

	client, err := uevent.Connect()
	if err != nil {
		return fmt.Errorf("cannot connect to uevent service: %w", err)
	}
	defer client.Close()

	fmt.Println("Monitoring device events (press Ctrl+C to stop)...")

	client.Uevent.OnDeviceAdded(func(ev uevent.DeviceEvent) {
		fmt.Printf("%s  %s  [%s]  %s\n", format.Color(format.Green, "ADD"), ev.DevPath, ev.Subsystem, ev.DevName)
	})
	client.Uevent.OnDeviceRemoved(func(ev uevent.DeviceEvent) {
		fmt.Printf("%s  %s  [%s]  %s\n", format.Color(format.Red, "REMOVE"), ev.DevPath, ev.Subsystem, ev.DevName)
	})
	client.Uevent.OnDeviceChanged(func(ev uevent.DeviceEvent) {
		fmt.Printf("%s  %s  [%s]  %s\n", format.Color(format.Yellow, "CHANGE"), ev.DevPath, ev.Subsystem, ev.DevName)
	})
	go func() {
		for {
			tx, err := client.Uevent.Conn.Recv()
			if err != nil {
				return
			}
			if !client.Uevent.Conn.Route(tx) {
				_ = client.HandleEvent(tx)
			}
		}
	}()

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig
	fmt.Println()
	return nil
}

func cmdTrigger(args []string) error {
	fs := flag.NewFlagSet("trigger", flag.ContinueOnError)
	subsystem := fs.String("subsystem", "", "Trigger specific subsystem only")
	fs.SetOutput(os.Stderr)
	if err := fs.Parse(args); err != nil {
		return err
	}

	client, err := uevent.Connect()
	if err != nil {
		return fmt.Errorf("cannot connect to uevent service: %w", err)
	}
	defer client.Close()

	if err := client.Uevent.Trigger(uevent.TriggerRequest{Subsystem: *subsystem}); err != nil {
		return fmt.Errorf("trigger failed: %w", err)
	}

	if *subsystem != "" {
		format.Success("Triggered re-scan for subsystem: %s", *subsystem)
	} else {
		format.Success("Triggered full device re-scan")
	}
	return nil
}
