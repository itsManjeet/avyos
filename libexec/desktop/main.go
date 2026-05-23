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
	"flag"
	"fmt"
	"os"
	"os/signal"
	"runtime"
	"syscall"

	"avyos.dev/lib/graphics/app"
	"avyos.dev/lib/graphics/backend/drmkms"
	"avyos.dev/lib/logger"
)

func init() {
	runtime.LockOSThread()

	log = logger.New("dev.avyos.desktop")
	_ = logger.SetupLog()
}

var (
	log                 *logger.Logger
	resolutionFlag      = flag.String("resolution", "", "DRM mode to use, e.g. 1920x1200 or 1920x1200@60")
	listResolutionsFlag = flag.Bool("list-resolutions", false, "List available DRM modes and exit")
)

func main() {
	flag.Parse()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP, syscall.SIGQUIT)
	defer signal.Stop(sigCh)
	go watchDesktopSignals(sigCh)

	drmBackend := drmkms.New("")

	if *listResolutionsFlag {
		if err := printAvailableResolutions(drmBackend); err != nil {
			fmt.Fprintf(os.Stderr, "desktop: list resolutions: %v\n", err)
			os.Exit(1)
		}
		return
	}

	app.Options.Title = "Desktop"
	app.Options.Width = 0
	app.Options.Height = 0
	app.Options.Scale = 1
	app.Options.Fullscreen = true
	app.Options.Resizable = true
	app.Options.Backend = drmBackend

	if *resolutionFlag != "" {
		drmBackend.SetMode(*resolutionFlag)
	}

	if err := app.Run(&DesktopApp{}); err != nil {
		fmt.Fprintf(os.Stderr, "desktop: %v\n", err)
		os.Exit(1)
	}
}

func watchDesktopSignals(sigCh <-chan os.Signal) {
	for sig := range sigCh {
		if shouldStopDesktopOnSignal(sig) {
			app.Stop()
			return
		}
	}
}

func shouldStopDesktopOnSignal(sig os.Signal) bool {
	switch sig {
	case syscall.SIGTERM, syscall.SIGHUP, syscall.SIGQUIT:
		return true
	default:
		return false
	}
}

func printAvailableResolutions(b *drmkms.Backend) error {
	if err := b.Init(); err != nil {
		return err
	}
	defer b.Shutdown()
	modes, err := b.Modes()
	if err != nil {
		return err
	}
	if len(modes) == 0 {
		return fmt.Errorf("no display modes available")
	}
	fmt.Fprintln(os.Stdout, "Available DRM modes:")
	for _, m := range modes {
		fmt.Fprintln(os.Stdout, m.String())
	}
	return nil
}
