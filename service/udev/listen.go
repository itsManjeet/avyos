package main

import (
	"flag"
	"log"
	"runtime"
	"syscall"

	"avyos.dev/pkg/job"
)

const (
	EventBufferSize = 1024 * 1024 * 8
	MaxJobQueue     = 256
	ObjectID        = 2
)

func listen(args []string) error {
	f := flag.NewFlagSet("listen", flag.ContinueOnError)

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

	syscall.SetsockoptInt(fd, syscall.SOL_SOCKET, syscall.SO_RCVBUF, 8*1024*1024)

	if err := syscall.Bind(fd, &syscall.SockaddrNetlink{
		Family: syscall.AF_NETLINK,
		Groups: 1,
		Pid:    uint32(syscall.Getpid()),
	}); err != nil {
		return err
	}

	buf := make([]byte, EventBufferSize)

	u := &udev{jobs: job.NewJobPool(runtime.NumCPU(), MaxJobQueue)}
	go StartService(u)

	for {
		n, _, err := syscall.Recvfrom(fd, buf, 0)
		if err != nil {
			log.Println("Error: failed to recieve uevent", err)
			continue
		}
		ev := Parse(buf[:n])

		u.mutex.Lock()
		u.jobs.Submit(ev)
		u.mutex.Unlock()
	}
}
