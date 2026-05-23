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
	"strings"

	desktopapi "avyos.dev/api/desktop"
	"avyos.dev/pkg/format"
)

func init() {
	flag.Usage = func() {
		fmt.Fprintln(os.Stderr, "notify - Send a desktop notification")
		fmt.Fprintln(os.Stderr)
		fmt.Fprintln(os.Stderr, "Usage:")
		fmt.Fprintln(os.Stderr, "  notify [options] <title> [message]")
		fmt.Fprintln(os.Stderr)
		fmt.Fprintln(os.Stderr, "Options:")
		fmt.Fprintln(os.Stderr, "  -app-id string     Application identifier")
		fmt.Fprintln(os.Stderr, "  -app-name string   Application display name")
		fmt.Fprintln(os.Stderr, "  -icon string       Notification icon name")
		fmt.Fprintln(os.Stderr, "  -title string      Notification title")
		fmt.Fprintln(os.Stderr, "  -message string    Notification message")
		fmt.Fprintln(os.Stderr, "  -stdin             Read notification message from stdin")
		fmt.Fprintln(os.Stderr)
		fmt.Fprintln(os.Stderr, "Examples:")
		fmt.Fprintln(os.Stderr, "  notify \"Build finished\" \"All checks passed\"")
		fmt.Fprintln(os.Stderr, "  notify -app-name CI -icon emblem-ok -title \"Pipeline\" -message \"Deploy complete\"")
		fmt.Fprintln(os.Stderr, "  echo \"Long message\" | notify -title \"Notes\" -stdin")
		fmt.Fprintln(os.Stderr)
		fmt.Fprintln(os.Stderr, "Exit Codes:")
		fmt.Fprintln(os.Stderr, "  0  Success")
		fmt.Fprintln(os.Stderr, "  1  Runtime/command error")
		fmt.Fprintln(os.Stderr, "  2  Invalid flags/usage")
	}
}

func main() {
	appID := flag.String("app-id", "", "Application identifier")
	appName := flag.String("app-name", "", "Application display name")
	icon := flag.String("icon", "", "Notification icon name")
	title := flag.String("title", "", "Notification title")
	message := flag.String("message", "", "Notification message")
	readStdin := flag.Bool("stdin", false, "Read notification message from stdin")

	flag.Parse()

	if err := run(*appID, *appName, *icon, *title, *message, *readStdin, flag.Args()); err != nil {
		format.Error("%s", err)
		os.Exit(1)
	}
}

func run(appID, appName, icon, title, message string, readStdin bool, args []string) error {
	if strings.TrimSpace(title) == "" && len(args) > 0 {
		title = args[0]
		args = args[1:]
	}

	if strings.TrimSpace(message) == "" && len(args) > 0 {
		message = strings.Join(args, " ")
	}

	if readStdin {
		data, err := io.ReadAll(os.Stdin)
		if err != nil {
			return fmt.Errorf("read stdin: %w", err)
		}
		text := strings.TrimSpace(string(data))
		if text != "" {
			if strings.TrimSpace(message) == "" {
				message = text
			} else {
				message = message + "\n" + text
			}
		}
	}

	title = strings.TrimSpace(title)
	message = strings.TrimSpace(message)
	if title == "" {
		return fmt.Errorf("usage: notify [options] <title> [message]")
	}

	client, err := desktopapi.Connect()
	if err != nil {
		return fmt.Errorf("connect desktop service: %w", err)
	}
	defer client.Close()

	if err := client.Notify(desktopapi.NotificationRequest{
		AppId:   strings.TrimSpace(appID),
		AppName: strings.TrimSpace(appName),
		Title:   title,
		Message: message,
		Icon:    strings.TrimSpace(icon),
	}); err != nil {
		return fmt.Errorf("send notification: %w", err)
	}

	return nil
}
