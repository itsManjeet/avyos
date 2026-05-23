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

package gpu

import (
	"fmt"
	"os"
	"unsafe"
)

// ---- QXL parameter IDs (DRM_QXL_GETPARAM) ----

const (
	QXLParamNumSurfaces   = 1
	QXLParamMaxRelocs     = 2
)

// ---- QXL update area types ----

const (
	QXLUpdateAreaType2D = 0
	QXLUpdateAreaType3D = 1
)

// ---- QXL ioctl structures (64-bit Linux ABI) ----

// qxlAlloc maps to struct drm_qxl_alloc.
type qxlAlloc struct {
	Size   uint32
	Handle uint32
}

// qxlMap maps to struct drm_qxl_map.
type qxlMap struct {
	Offset uint64
	Handle uint32
	_      [4]byte
}

// QXLReloc describes a single relocation entry for command submission.
// It maps to struct drm_qxl_reloc.
type QXLReloc struct {
	SrcHandle   uint32
	SrcType     uint32
	DstHandle   uint32
	DstType     uint32
	SrcOffset   uint64
	DstOffset   uint64
}

// QXLClipRect is a rectangular clip region for update-area commands.
// It maps to struct drm_clip_rect.
type QXLClipRect struct {
	X1, Y1, X2, Y2 uint16
}

// qxlExecBuffer maps to struct drm_qxl_execbuffer.
type qxlExecBuffer struct {
	Flags     uint32
	SurfID    uint32
	NumRelocs uint32
	NumClips  uint32
	RelocsPtr uint64
	ClipsPtr  uint64
}

// qxlUpdateArea maps to struct drm_qxl_update_area.
type qxlUpdateArea struct {
	Handle uint32
	Type   uint32
	Area   QXLClipRect
	_      [4]byte
}

// qxlGetParam maps to struct drm_qxl_getparam.
type qxlGetParam struct {
	Param uint64
	Value uint64
}

// qxlClientCap maps to struct drm_qxl_clientcap.
type qxlClientCap struct {
	Index uint32
	_     [4]byte
}

// QXLSurfaceFormat represents QXL surface pixel formats.
type QXLSurfaceFormat uint32

const (
	QXLSurfaceFormatRGBX8888 QXLSurfaceFormat = 32 // 32bpp packed RGBX
	QXLSurfaceFormatARGB8888 QXLSurfaceFormat = 33 // 32bpp packed ARGB
)

// qxlAllocSurf maps to struct drm_qxl_alloc_surf.
type qxlAllocSurf struct {
	Format QXLSurfaceFormat
	Width  uint32
	Height uint32
	Stride int32
	Handle uint32
	_      [4]byte
}

// ---- QXL ioctl request codes ----

var (
	ioctlQXLAlloc      = iowr('d', drmCommandBase+0x00, unsafe.Sizeof(qxlAlloc{}))
	ioctlQXLMap        = iowr('d', drmCommandBase+0x01, unsafe.Sizeof(qxlMap{}))
	ioctlQXLExecBuffer = iowr('d', drmCommandBase+0x02, unsafe.Sizeof(qxlExecBuffer{}))
	ioctlQXLUpdateArea = iowr('d', drmCommandBase+0x03, unsafe.Sizeof(qxlUpdateArea{}))
	ioctlQXLGetParam   = iowr('d', drmCommandBase+0x04, unsafe.Sizeof(qxlGetParam{}))
	ioctlQXLClientCap  = iow('d', drmCommandBase+0x05, unsafe.Sizeof(qxlClientCap{}))
	ioctlQXLAllocSurf  = iowr('d', drmCommandBase+0x06, unsafe.Sizeof(qxlAllocSurf{}))
	ioctlQXLGEMClose   = iowr('d', 0x09, unsafe.Sizeof(gemClose{}))
)

// ---- QXLDevice ----

// QXLDevice is a handle to a QXL virtual DRI render node.
//
// Obtain one via [Open] or [Probe] and type-assert:
//
//	qxl := dev.(*gpu.QXLDevice)
type QXLDevice struct {
	file *os.File
	path string
}

func (d *QXLDevice) Driver() Driver { return DriverQXL }
func (d *QXLDevice) Path() string   { return d.path }
func (d *QXLDevice) Close() error   { return d.file.Close() }

// GetParam queries a QXL driver parameter.
// param is one of the QXLParam* constants.
func (d *QXLDevice) GetParam(param uint64) (uint64, error) {
	p := qxlGetParam{Param: param}
	if err := ioctl(fd(d.file), ioctlQXLGetParam, unsafe.Pointer(&p)); err != nil {
		return 0, fmt.Errorf("qxl GetParam %d: %w", param, err)
	}
	return p.Value, nil
}

// Alloc allocates a QXL memory buffer of size bytes.
// Returns the kernel GEM handle.
func (d *QXLDevice) Alloc(size uint32) (handle uint32, err error) {
	p := qxlAlloc{Size: size}
	if err = ioctl(fd(d.file), ioctlQXLAlloc, unsafe.Pointer(&p)); err != nil {
		return 0, fmt.Errorf("qxl Alloc: %w", err)
	}
	return p.Handle, nil
}

// Free releases a QXL GEM buffer handle.
func (d *QXLDevice) Free(handle uint32) error {
	c := gemClose{Handle: handle}
	if err := ioctl(fd(d.file), ioctlQXLGEMClose, unsafe.Pointer(&c)); err != nil {
		return fmt.Errorf("qxl Free: %w", err)
	}
	return nil
}

// Map returns the mmap offset for a QXL GEM buffer.
// Use the offset with mmap(2) on the device file descriptor to obtain
// a CPU-accessible mapping.
func (d *QXLDevice) Map(handle uint32) (offset uint64, err error) {
	p := qxlMap{Handle: handle}
	if err = ioctl(fd(d.file), ioctlQXLMap, unsafe.Pointer(&p)); err != nil {
		return 0, fmt.Errorf("qxl Map: %w", err)
	}
	return p.Offset, nil
}

// AllocSurface allocates a typed display surface.
//
// format is one of the QXLSurfaceFormat* constants.
// stride is the row pitch in bytes; a negative stride indicates a
// bottom-up layout (as used by some QXL command encodings).
// Returns the GEM handle for the surface buffer.
func (d *QXLDevice) AllocSurface(format QXLSurfaceFormat, width, height uint32, stride int32) (handle uint32, err error) {
	p := qxlAllocSurf{
		Format: format,
		Width:  width,
		Height: height,
		Stride: stride,
	}
	if err = ioctl(fd(d.file), ioctlQXLAllocSurf, unsafe.Pointer(&p)); err != nil {
		return 0, fmt.Errorf("qxl AllocSurface: %w", err)
	}
	return p.Handle, nil
}

// UpdateArea asks the QXL driver to send a dirty-rect update for surfID
// to the QEMU display device.  area is the region to update; pass a
// zero-value QXLClipRect to update the full surface.
// updateType is one of QXLUpdateArea*.
func (d *QXLDevice) UpdateArea(handle, updateType uint32, area QXLClipRect) error {
	p := qxlUpdateArea{
		Handle: handle,
		Type:   updateType,
		Area:   area,
	}
	if err := ioctl(fd(d.file), ioctlQXLUpdateArea, unsafe.Pointer(&p)); err != nil {
		return fmt.Errorf("qxl UpdateArea: %w", err)
	}
	return nil
}

// ClientCap advertises a capability index to the QXL driver.
// The meaning of index values is defined by the QXL protocol.
func (d *QXLDevice) ClientCap(index uint32) error {
	p := qxlClientCap{Index: index}
	if err := ioctl(fd(d.file), ioctlQXLClientCap, unsafe.Pointer(&p)); err != nil {
		return fmt.Errorf("qxl ClientCap: %w", err)
	}
	return nil
}

// ExecBuffer submits a command buffer to the QXL engine.
//
// surfID identifies the target surface (0 for the primary surface).
// relocs is the list of buffer relocations; clips is the list of dirty
// clip rectangles.  flags is driver-defined.
func (d *QXLDevice) ExecBuffer(surfID, flags uint32, relocs []QXLReloc, clips []QXLClipRect) error {
	p := qxlExecBuffer{
		Flags:     flags,
		SurfID:    surfID,
		NumRelocs: uint32(len(relocs)),
		NumClips:  uint32(len(clips)),
	}
	if len(relocs) > 0 {
		p.RelocsPtr = uint64(uintptr(unsafe.Pointer(&relocs[0])))
	}
	if len(clips) > 0 {
		p.ClipsPtr = uint64(uintptr(unsafe.Pointer(&clips[0])))
	}
	if err := ioctl(fd(d.file), ioctlQXLExecBuffer, unsafe.Pointer(&p)); err != nil {
		return fmt.Errorf("qxl ExecBuffer: %w", err)
	}
	return nil
}
