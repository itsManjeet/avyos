package fs

import (
	"os"
	"syscall"
)

var (
	Stdin  = os.NewFile(uintptr(syscall.Stdin), Resolve("device:stdin"))
	Stdout = os.NewFile(uintptr(syscall.Stdout), Resolve("device:stdout"))
	Stderr = os.NewFile(uintptr(syscall.Stderr), Resolve("device:stderr"))
)

func NullDevice() *os.File {
	f, err := os.OpenFile(Resolve("device:null"), os.O_RDWR, 0666)
	if err != nil {
		panic("failed to open device:null : " + err.Error())
	}
	return f
}
