package main

import (
	"flag"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"syscall"

	"avyos.dev/pkg/job"
)

const (
	EventBufferSize = 64 * 1024
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

	buf := make([]byte, EventBufferSize)
	pool := job.NewJobPool(runtime.NumCPU(), 256)

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
		pool.Submit(ev)
	}
}
