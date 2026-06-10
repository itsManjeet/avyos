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
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"avyos.dev/api/desktop"
)

type crashReport struct {
	Timestamp string `json:"timestamp"`
	Hostname  string `json:"hostname"`
	Exec      string `json:"exec"`
	PID       int    `json:"pid"`
	UID       int    `json:"uid"`
	GID       int    `json:"gid"`
	Signal    string `json:"signal"`
	CoreBytes int64  `json:"core_bytes"`
}

func main() {
	var (
		pid      = flag.Int("pid", 0, "Process ID")
		uid      = flag.Int("uid", -1, "User ID")
		gid      = flag.Int("gid", -1, "Group ID")
		signal   = flag.String("signal", "", "Fatal signal")
		execName = flag.String("exec", "", "Executable name")
		hostname = flag.String("hostname", "", "Hostname")
		epoch    = flag.Int64("time", 0, "Crash time (unix epoch)")
	)
	flag.Parse()

	crashTime := time.Now().UTC()
	if *epoch > 0 {
		crashTime = time.Unix(*epoch, 0).UTC()
	}

	n, _ := io.Copy(io.Discard, os.Stdin)

	report := crashReport{
		Timestamp: crashTime.Format(time.RFC3339),
		Hostname:  strings.TrimSpace(*hostname),
		Exec:      sanitizeExecName(*execName),
		PID:       *pid,
		UID:       *uid,
		GID:       *gid,
		Signal:    strings.TrimSpace(*signal),
		CoreBytes: n,
	}
	if report.Exec == "" {
		report.Exec = "application"
	}
	if report.Hostname == "" {
		report.Hostname = "avyos"
	}
	if report.Signal == "" {
		report.Signal = "unknown"
	}

	_ = persistCrashReport(report)
	_ = sendCrashNotification(report)
}

func persistCrashReport(report crashReport) error {
	dir := "/var/log/crash.log"
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	name := fmt.Sprintf("%s-%d.json",
		strings.ReplaceAll(strings.ReplaceAll(report.Timestamp, ":", "-"), "T", "_"),
		report.PID,
	)
	path := filepath.Join(dir, name)
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0644)
}

func sendCrashNotification(report crashReport) error {
	client, err := desktop.Connect()
	if err != nil {
		return err
	}
	defer client.Close()

	req := desktop.NotificationRequest{
		AppId:   "dev.avyos.coredump",
		AppName: "Crash Reporter",
		Title:   fmt.Sprintf("%s crashed", report.Exec),
		Message: formatCrashMessage(report),
		Icon:    "dialog-error",
	}
	return client.Notify(req)
}

func formatCrashMessage(report crashReport) string {
	parts := []string{}
	if report.PID > 0 {
		parts = append(parts, "pid "+strconv.Itoa(report.PID))
	}
	if report.Signal != "" {
		parts = append(parts, "signal "+report.Signal)
	}
	if report.CoreBytes > 0 {
		parts = append(parts, fmt.Sprintf("core %d bytes", report.CoreBytes))
	}
	if report.Hostname != "" {
		parts = append(parts, "host "+report.Hostname)
	}
	return strings.Join(parts, " • ")
}

func sanitizeExecName(name string) string {
	name = strings.TrimSpace(name)
	name = filepath.Base(name)
	name = strings.Trim(name, " .")
	return name
}
