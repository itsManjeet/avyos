//go:build linux

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

// Package gpu provides a pure-Go interface to DRI render nodes on Linux.
//
// It targets two GPU families:
//
//   - Intel (i915) — GEM buffer objects, execbuffer2, render contexts
//   - QXL — QEMU/KVM virtual GPU buffers and command submission
//
// Both families share the same [Device] interface. Callers probe the
// hardware with [Open], which detects the driver automatically:
//
//	dev, err := gpu.Open("")          // probe /dev/dri/renderD128–135
//	dev, err := gpu.Open("/dev/dri/renderD128")  // explicit path
//
// # Intel usage
//
//	intel := dev.(*gpu.IntelDevice)
//	buf, _ := intel.GEMCreate(4096)
//	intel.GEMWrite(buf, 0, data)
//	intel.ExecBuffer(ctx, objects, batchHandle, batchLen, flags)
//
// # QXL usage
//
//	qxl := dev.(*gpu.QXLDevice)
//	handle, _ := qxl.Alloc(4096)
//	offset, _ := qxl.Map(handle)
//	qxl.ExecBuffer(surfID, relocs, clips, flags)
package gpu

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"unsafe"
)

// Driver identifies the GPU driver family.
type Driver int

const (
	DriverUnknown Driver = iota
	DriverIntel
	DriverQXL
)

func (d Driver) String() string {
	switch d {
	case DriverIntel:
		return "i915"
	case DriverQXL:
		return "qxl"
	default:
		return "unknown"
	}
}

// Device is the common interface implemented by [IntelDevice] and [QXLDevice].
type Device interface {
	// Driver returns which GPU family this device belongs to.
	Driver() Driver
	// Path returns the /dev/dri/renderD* path that was opened.
	Path() string
	// Close releases the file descriptor.
	Close() error
}

// Open opens a DRI render node and returns the appropriate Device.
//
// If path is empty the first render node that reports a known driver
// (i915 or qxl) is returned.  An error is returned if no suitable
// device is found or the path is not a recognised GPU.
func Open(path string) (Device, error) {
	if path != "" {
		return openOne(path)
	}
	for i := 128; i <= 135; i++ {
		p := fmt.Sprintf("/dev/dri/renderD%d", i)
		if _, err := os.Stat(p); err != nil {
			continue
		}
		dev, err := openOne(p)
		if err != nil {
			continue
		}
		return dev, nil
	}
	return nil, errors.New("gpu: no supported render node found")
}

// Probe returns all recognised GPU render nodes on the system.
func Probe() ([]Device, error) {
	matches, err := filepath.Glob("/dev/dri/render*")
	if err != nil {
		return nil, err
	}
	var devs []Device
	for _, p := range matches {
		dev, err := openOne(p)
		if err != nil {
			continue
		}
		devs = append(devs, dev)
	}
	if len(devs) == 0 {
		return nil, errors.New("gpu: no supported render nodes found")
	}
	return devs, nil
}

// openOne opens one render node, queries the DRM driver name, and
// returns an IntelDevice or QXLDevice.
func openOne(path string) (Device, error) {
	f, err := os.OpenFile(path, os.O_RDWR, 0)
	if err != nil {
		return nil, fmt.Errorf("gpu: open %s: %w", path, err)
	}
	name, err := drmDriverName(f)
	if err != nil {
		f.Close()
		return nil, fmt.Errorf("gpu: query driver %s: %w", path, err)
	}
	switch {
	case strings.Contains(name, "i915"):
		return &IntelDevice{file: f, path: path}, nil
	case strings.Contains(name, "qxl"):
		return &QXLDevice{file: f, path: path}, nil
	default:
		f.Close()
		return nil, fmt.Errorf("gpu: unsupported driver %q on %s", name, path)
	}
}

// drmDriverName queries the DRM version and returns the driver name string.
func drmDriverName(f *os.File) (string, error) {
	var v drmVersion
	nameBuf := make([]byte, 64)
	v.NameLen = uint64(len(nameBuf))
	v.NamePtr = uint64(uintptr(unsafe.Pointer(&nameBuf[0])))
	if err := ioctl(f.Fd(), drmIoctlVersion, unsafe.Pointer(&v)); err != nil {
		return "", err
	}
	return strings.TrimRight(string(nameBuf[:v.NameLen]), "\x00"), nil
}

// fd returns the raw file descriptor for an os.File.
func fd(f *os.File) uintptr { return f.Fd() }

// ---- low-level ioctl ----

func ioc(dir, typ, nr, size uintptr) uintptr {
	return dir<<30 | typ<<8 | nr | size<<16
}
func ior(typ, nr, size uintptr) uintptr  { return ioc(2, typ, nr, size) }
func iow(typ, nr, size uintptr) uintptr  { return ioc(1, typ, nr, size) }
func iowr(typ, nr, size uintptr) uintptr { return ioc(3, typ, nr, size) }

func ioctl(fileFD uintptr, req uintptr, arg unsafe.Pointer) error {
	for {
		_, _, errno := syscall.Syscall(syscall.SYS_IOCTL, fileFD, req, uintptr(arg))
		if errno == 0 {
			return nil
		}
		if errno == syscall.EINTR || errno == syscall.EAGAIN {
			continue
		}
		return errno
	}
}

// ---- DRM base structures & ioctls ----

// drmVersion maps to struct drm_version (64-bit Linux ABI).
// Pointers are passed as uint64 to avoid Go pointer restrictions.
type drmVersion struct {
	Major    int32
	Minor    int32
	Patch    int32
	_        [4]byte // padding to align NameLen
	NameLen  uint64
	NamePtr  uint64
	DateLen  uint64
	DatePtr  uint64
	DescLen  uint64
	DescPtr  uint64
}

var drmIoctlVersion = iowr('d', 0x00, unsafe.Sizeof(drmVersion{}))
