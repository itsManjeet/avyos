package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"golang.org/x/sys/unix"
)

type Event struct {
	Action    string
	Device    string
	Subsystem string
	Environ   map[string]string
}

func Parse(data []byte) *Event {
	parts := strings.Split(string(data), "\x00")

	ev := &Event{
		Environ: make(map[string]string),
	}

	if len(parts) > 0 {
		if i := strings.Index(parts[0], "@"); i >= 0 {
			ev.Action = parts[0][:i]
			ev.Device = parts[0][i+1:]
		}
	}

	for _, p := range parts[1:] {
		if p == "" {
			continue
		}
		if i := strings.Index(p, "="); i >= 0 {
			key := p[:i]
			val := p[i+1:]
			ev.Environ[key] = val
		}
	}

	ev.Subsystem = ev.Environ["SUBSYSTEM"]
	return ev
}

func (e *Event) Run() error {
	fmt.Printf("UDEV: ACTION=%s SUBSYSTEM=%s DEVICE=%s ENVIRON=%v\n",
		e.Action, e.Subsystem, e.Device, e.Environ)

	var err error
	switch e.Action {
	case "add":
		tryLoadModule(e)
		err = createDevice(e)
	case "remove":
		err = removeDevice(e)
	}
	return err
}

func tryLoadModule(ev *Event) {
	alias, ok := ev.Environ["MODALIAS"]
	if !ok || alias == "" {
		return
	}

	exec.Command("busybox", "modprobe", "-q", alias).Run()
}

func createDevice(ev *Event) error {
	devname, ok := ev.Environ["DEVNAME"]
	if !ok {
		return nil
	}

	major, minor, err := parseDevNums(ev)
	if err != nil {
		return err
	}

	fullPath := filepath.Join("/dev", devname)

	if err := ensureParent(fullPath); err != nil {
		return err
	}

	_ = os.Remove(fullPath)

	mode := deviceMode(ev) | 0660
	dev := unix.Mkdev(major, minor)

	if err := unix.Mknod(fullPath, mode, int(dev)); err != nil {
		return fmt.Errorf("mknod %s: %w", fullPath, err)
	}

	return nil
}

func removeDevice(ev *Event) error {
	devname, ok := ev.Environ["DEVNAME"]
	if !ok {
		return nil
	}
	path := filepath.Join("/dev", devname)
	return os.Remove(path)
}

func parseDevNums(ev *Event) (uint32, uint32, error) {
	majStr, ok1 := ev.Environ["MAJOR"]
	minStr, ok2 := ev.Environ["MINOR"]
	if !ok1 || !ok2 {
		return 0, 0, fmt.Errorf("missing MAJOR/MINOR")
	}

	maj, err := strconv.Atoi(majStr)
	if err != nil {
		return 0, 0, err
	}
	min, err := strconv.Atoi(minStr)
	if err != nil {
		return 0, 0, err
	}

	return uint32(maj), uint32(min), nil
}

func ensureParent(path string) error {
	dir := filepath.Dir(path)
	if dir == "." || dir == "/" {
		return nil
	}
	return os.MkdirAll(dir, 0755)
}

func deviceMode(ev *Event) uint32 {
	if ev.Subsystem == "block" {
		return unix.S_IFBLK
	}
	return unix.S_IFCHR
}
