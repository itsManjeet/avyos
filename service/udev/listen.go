package main

import (
	"flag"
	"fmt"
	"io/fs"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"syscall"

	"golang.org/x/sys/unix"
)

func listen(args []string) error {
	f := flag.NewFlagSet("listen", flag.ContinueOnError)
	trigger := f.Bool("trigger", false, "trigger kernel events")

	if err := f.Parse(args); err != nil {
		return err
	}
	fd, err := syscall.Socket(
		syscall.AF_NETLINK,
		syscall.SOCK_DGRAM,
		syscall.NETLINK_KOBJECT_UEVENT,
	)
	if err != nil {
		return err
	}
	defer syscall.Close(fd)

	if err := syscall.Bind(fd, &syscall.SockaddrNetlink{
		Family: syscall.AF_NETLINK,
		Groups: 1,
		Pid:    uint32(syscall.Getpid()),
	}); err != nil {
		return err
	}

	buf := make([]byte, 64*1024)

	if *trigger {
		go func() {
			filepath.Walk("/sys", func(path string, info fs.FileInfo, err error) error {
				if err != nil || info.IsDir() || filepath.Base(path) != "uevent" {
					return err
				}
				_ = os.WriteFile(path, []byte("add\n"), 0644)
				return nil
			})
		}()
	}

	for {
		n, _, err := syscall.Recvfrom(fd, buf, 0)
		if err != nil {
			log.Println("Error: failed to recieve uevent", err)
			continue
		}
		ev := Parse(buf[:n])
		handleEvent(ev)
	}

}

func handleEvent(ev *Event) {
	fmt.Printf("UDEV: ACTION=%s SUBSYSTEM=%s DEVICE=%s ENVIRON=%v\n",
		ev.Action, ev.Subsystem, ev.Device, ev.Environ)

	var err error
	switch ev.Action {
	case "add":
		tryLoadModule(ev)
		err = createDevice(ev)
	case "remove":
		err = removeDevice(ev)
	}
	if err != nil {
		fmt.Printf("Error: %v\n", err)
	}
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
