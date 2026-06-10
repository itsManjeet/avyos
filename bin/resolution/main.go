package main

import (
	"flag"
	"fmt"
	"os"
	"sort"
	"strings"

	"avyos.dev/api/service"
	"avyos.dev/lib/format"
	"avyos.dev/lib/graphics/backend/drmkms"
)

const desktopServiceName = "desktop"

func init() {
	flag.Usage = func() {
		fmt.Fprintln(os.Stderr, "resolution - Manage the system display resolution")
		fmt.Fprintln(os.Stderr)
		fmt.Fprintln(os.Stderr, "Usage:")
		fmt.Fprintln(os.Stderr, "  resolution <subcommand> [args]")
		fmt.Fprintln(os.Stderr)
		fmt.Fprintln(os.Stderr, "Subcommands:")
		fmt.Fprintln(os.Stderr, "  list          List available DRM modes")
		fmt.Fprintln(os.Stderr, "  show          Show the configured display mode")
		fmt.Fprintln(os.Stderr, "  set <mode>    Persist a DRM mode and restart desktop")
		fmt.Fprintln(os.Stderr, "  clear         Remove the persisted mode and restart desktop")
		fmt.Fprintln(os.Stderr)
		fmt.Fprintln(os.Stderr, "Examples:")
		fmt.Fprintln(os.Stderr, "  resolution list")
		fmt.Fprintln(os.Stderr, "  resolution set 1920x1200")
		fmt.Fprintln(os.Stderr, "  resolution set 1920x1200@60")
	}
}

func main() {
	flag.Parse()
	args := flag.Args()

	commands := map[string]func([]string) error{
		"list":  cmdList,
		"show":  cmdShow,
		"set":   cmdSet,
		"clear": cmdClear,
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

	backend := drmkms.New("")
	if err := backend.Init(); err != nil {
		return fmt.Errorf("cannot initialize DRM/KMS backend: %w", err)
	}
	defer backend.Shutdown()

	modes, err := backend.Modes()
	if err != nil {
		return fmt.Errorf("cannot query display modes: %w", err)
	}
	if len(modes) == 0 {
		return fmt.Errorf("no display modes available")
	}

	sort.SliceStable(modes, func(i, j int) bool {
		li := modes[i].Width * modes[i].Height
		lj := modes[j].Width * modes[j].Height
		if li != lj {
			return li > lj
		}
		if modes[i].Width != modes[j].Width {
			return modes[i].Width > modes[j].Width
		}
		if modes[i].Height != modes[j].Height {
			return modes[i].Height > modes[j].Height
		}
		return modes[i].Refresh > modes[j].Refresh
	})

	configured, err := drmkms.LoadConfiguredMode()
	if err != nil {
		return fmt.Errorf("cannot read configured display mode: %w", err)
	}
	if configured == "" {
		fmt.Println("Configured mode: automatic")
	} else {
		fmt.Printf("Configured mode: %s\n", configured)
	}
	fmt.Println("Available DRM modes:")
	for _, mode := range modes {
		fmt.Printf("  %s\n", mode.String())
	}
	return nil
}

func cmdShow(args []string) error {
	fs := flag.NewFlagSet("show", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	if err := fs.Parse(args); err != nil {
		return err
	}

	configured, err := drmkms.LoadConfiguredMode()
	if err != nil {
		return fmt.Errorf("cannot read configured display mode: %w", err)
	}
	if configured == "" {
		fmt.Println("automatic")
		return nil
	}
	fmt.Println(configured)
	return nil
}

func cmdSet(args []string) error {
	fs := flag.NewFlagSet("set", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	if err := fs.Parse(args); err != nil {
		return err
	}
	args = fs.Args()
	if len(args) != 1 {
		return fmt.Errorf("usage: resolution set <WxH[@Hz]>")
	}

	spec := strings.TrimSpace(args[0])
	if spec == "" {
		return fmt.Errorf("resolution is required")
	}

	backend := drmkms.New("")
	if err := backend.Init(); err != nil {
		return fmt.Errorf("cannot initialize DRM/KMS backend: %w", err)
	}
	modes, err := backend.Modes()
	backend.Shutdown()
	if err != nil {
		return fmt.Errorf("cannot query display modes: %w", err)
	}

	selected, err := selectConfiguredMode(modes, spec)
	if err != nil {
		return err
	}
	if err := drmkms.SaveConfiguredMode(selected.Spec()); err != nil {
		return fmt.Errorf("cannot save display mode: %w", err)
	}
	if err := restartDesktopService(); err != nil {
		return err
	}

	format.Success("set display mode to %s and restarted %s", selected.Spec(), desktopServiceName)
	return nil
}

func cmdClear(args []string) error {
	fs := flag.NewFlagSet("clear", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	if err := fs.Parse(args); err != nil {
		return err
	}

	if err := drmkms.ClearConfiguredMode(); err != nil {
		return fmt.Errorf("cannot clear display mode: %w", err)
	}
	if err := restartDesktopService(); err != nil {
		return err
	}

	format.Success("cleared display mode override and restarted %s", desktopServiceName)
	return nil
}

func restartDesktopService() error {
	client, err := service.Connect()
	if err != nil {
		return fmt.Errorf("cannot connect to service manager: %w", err)
	}
	defer client.Close()

	if err := client.Restart(desktopServiceName); err != nil {
		return fmt.Errorf("restart %s: %w", desktopServiceName, err)
	}
	return nil
}

func selectConfiguredMode(modes []drmkms.Mode, spec string) (drmkms.Mode, error) {
	spec = strings.TrimSpace(spec)
	if spec == "" {
		return drmkms.Mode{}, fmt.Errorf("resolution is required")
	}
	sizePart, refreshPart, hasRefresh := strings.Cut(spec, "@")
	var reqWidth, reqHeight int
	if _, err := fmt.Sscanf(sizePart, "%dx%d", &reqWidth, &reqHeight); err != nil {
		if _, errAlt := fmt.Sscanf(sizePart, "%dX%d", &reqWidth, &reqHeight); errAlt != nil {
			return drmkms.Mode{}, fmt.Errorf("invalid resolution %q", spec)
		}
	}
	if reqWidth <= 0 || reqHeight <= 0 {
		return drmkms.Mode{}, fmt.Errorf("invalid resolution %q", spec)
	}

	var reqRefresh uint32
	if hasRefresh {
		if _, err := fmt.Sscanf(refreshPart, "%d", &reqRefresh); err != nil || reqRefresh == 0 {
			return drmkms.Mode{}, fmt.Errorf("invalid resolution %q", spec)
		}
	}

	var matches []drmkms.Mode
	for _, mode := range modes {
		if mode.Width != reqWidth || mode.Height != reqHeight {
			continue
		}
		if hasRefresh && mode.Refresh != reqRefresh {
			continue
		}
		matches = append(matches, mode)
	}
	if len(matches) == 0 {
		return drmkms.Mode{}, fmt.Errorf("resolution %q is not available", spec)
	}

	best := matches[0]
	if !hasRefresh {
		for _, mode := range matches[1:] {
			if mode.Refresh > best.Refresh {
				best = mode
			}
		}
	}
	return best, nil
}
