// Copyright (c) 2026 Manjeet Singh <itsmanjeet1998@gmail.com>.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, version 3.
//
// This program is distributed in the hope that it will be useful, but
// WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the GNU
// General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with this program. If not, see <http://www.gnu.org/licenses/>.

package main

import (
	"encoding/json"
	"fmt"
	"image"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"syscall"

	"avyos.dev/lib/fs"
	"avyos.dev/lib/graphics/svg"
	"avyos.dev/lib/graphics/widget"
)

const launcherIconDecodeSize = 128

type launcherApp struct {
	ID       string
	Name     string
	Dir      string
	ExecPath string
	IconPath string
	Icon     image.Image
}

type appManifest struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Exec string `json:"exec"`
}

type appExitReport struct {
	App      launcherApp
	PID      int
	ExitCode int
	Signal   syscall.Signal
	Signaled bool
	CoreDump bool
	WaitErr  error
}

func (r appExitReport) crashed() bool {
	if r.CoreDump || r.Signaled {
		return true
	}
	return r.WaitErr != nil && r.ExitCode != 0
}

func discoverLauncherApps() []launcherApp {
	roots := launcherAppRoots()
	byID := make(map[string]launcherApp)
	order := make([]string, 0, 16)

	for _, root := range roots {
		entries, err := os.ReadDir(root)
		if err != nil {
			continue
		}
		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}
			appDir := filepath.Join(root, entry.Name())
			app, ok := loadLauncherApp(appDir)
			if !ok {
				continue
			}
			key := app.ID
			if key == "" {
				key = entry.Name()
			}
			if _, exists := byID[key]; !exists {
				order = append(order, key)
			}
			merged := byID[key]
			merged = mergeLauncherApp(merged, app)
			byID[key] = merged
		}
	}

	apps := make([]launcherApp, 0, len(order))
	for _, key := range order {
		apps = append(apps, byID[key])
	}
	slices.SortFunc(apps, func(a, b launcherApp) int {
		return strings.Compare(strings.ToLower(a.Name), strings.ToLower(b.Name))
	})
	return apps
}

func launcherAppRoots() []string {
	roots := []string{
		"/apps",
		"/usr/apps",
	}

	if HOME := os.Getenv("HOME"); HOME != "" {
		roots = append(roots, filepath.Join(HOME, "Applications"))
	}

	seen := make(map[string]struct{}, len(roots))
	out := make([]string, 0, len(roots))
	for _, root := range roots {
		if root == "" || !fs.IsDir(root) {
			continue
		}
		if _, ok := seen[root]; ok {
			continue
		}
		seen[root] = struct{}{}
		out = append(out, root)
	}
	return out
}

func loadLauncherApp(appDir string) (launcherApp, bool) {
	manifestPath := filepath.Join(appDir, "manifest.json")
	if !fs.IsFile(manifestPath) {
		return launcherApp{}, false
	}

	data, err := os.ReadFile(manifestPath)
	if err != nil {
		return launcherApp{}, false
	}

	var mf appManifest
	if err := json.Unmarshal(data, &mf); err != nil {
		return launcherApp{}, false
	}

	execPath := strings.TrimSpace(mf.Exec)
	if execPath == "" {
		execPath = filepath.Join(appDir, "exec")
	} else if !filepath.IsAbs(execPath) && strings.Contains(execPath, "/") {
		execPath = filepath.Join(appDir, execPath)
	}

	iconPath := filepath.Join(appDir, "icon.svg")
	app := launcherApp{
		ID:       mf.ID,
		Name:     strings.TrimSpace(mf.Name),
		Dir:      appDir,
		ExecPath: execPath,
		IconPath: iconPath,
	}
	if app.Name == "" {
		app.Name = filepath.Base(appDir)
	}
	if fs.IsFile(iconPath) {
		if icon, err := loadLauncherIcon(iconPath); err == nil {
			app.Icon = icon
		}
	}
	if app.ExecPath != "" && !strings.Contains(app.ExecPath, " ") && !filepath.IsAbs(app.ExecPath) && !strings.Contains(app.ExecPath, "/") {
		if resolved, err := exec.LookPath(app.ExecPath); err == nil {
			app.ExecPath = resolved
		}
	}
	return app, true
}

func loadLauncherIcon(path string) (image.Image, error) {
	if strings.EqualFold(filepath.Ext(path), ".svg") {
		return svg.DecodeSizedFile(path, launcherIconDecodeSize, launcherIconDecodeSize)
	}
	img, err := widget.NewImageFromFilePath(path)
	if err != nil {
		return nil, err
	}
	return img.Source, nil
}

func mergeLauncherApp(dst, src launcherApp) launcherApp {
	if dst.ID == "" {
		dst.ID = src.ID
	}
	if dst.Name == "" {
		dst.Name = src.Name
	}
	if dst.Dir == "" {
		dst.Dir = src.Dir
	}
	if dst.ExecPath == "" || !fs.IsFile(dst.ExecPath) {
		dst.ExecPath = src.ExecPath
	}
	if dst.IconPath == "" || dst.Icon == nil {
		dst.IconPath = src.IconPath
		dst.Icon = src.Icon
	}
	return dst
}

func launchApp(app launcherApp, onExit func(appExitReport)) error {
	cmd, err := newAppCommand(app)
	if err != nil {
		return err
	}

	if err := cmd.Start(); err != nil {
		return err
	}

	go func() {
		err := cmd.Wait()
		if onExit == nil {
			return
		}

		report := appExitReport{
			App:     app,
			WaitErr: err,
		}
		if cmd.Process != nil {
			report.PID = cmd.Process.Pid
		}
		if state := cmd.ProcessState; state != nil {
			if ws, ok := state.Sys().(syscall.WaitStatus); ok {
				report.ExitCode = ws.ExitStatus()
				report.Signaled = ws.Signaled()
				report.Signal = ws.Signal()
				report.CoreDump = ws.CoreDump()
			}
		}
		onExit(report)
	}()

	return nil
}

func newAppCommand(app launcherApp) (*exec.Cmd, error) {
	command := strings.Fields(strings.TrimSpace(app.ExecPath))
	if len(command) == 0 {
		return nil, fmt.Errorf("app %q has no executable", app.Name)
	}

	exe := command[0]
	args := command[1:]
	if strings.Contains(exe, "/") {
		if !filepath.IsAbs(exe) {
			exe = filepath.Join(app.Dir, exe)
		}
		if !fs.Exists(exe) {
			return nil, fmt.Errorf("app executable not found: %s", exe)
		}
	} else {
		resolved, err := exec.LookPath(exe)
		if err != nil {
			return nil, fmt.Errorf("lookup %s: %w", exe, err)
		}
		exe = resolved
	}

	cmd := exec.Command(exe, args...)
	cmd.Dir = app.Dir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	return cmd, nil
}
