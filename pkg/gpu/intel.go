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

// ---- Intel i915 parameter IDs (DRM_I915_GETPARAM) ----

const (
	I915ParamChipsetID          = 4
	I915ParamHasGemMmap         = 30
	I915ParamNumFences          = 13
	I915ParamHasExecSOB         = 42
	I915ParamSubsliceTotal      = 33
	I915ParamEUTotal            = 34
	I915ParamHasGPUResetStats   = 35
	I915ParamHasResourceStreamer = 36
	I915ParamHasExecSoftpin     = 37
	I915ParamHasExecAsync       = 40
	I915ParamHasExecFence       = 44
)

// ---- ExecBuffer flags ----

const (
	// Ring selection
	I915ExecDefault     = 0
	I915ExecRender      = 1
	I915ExecBSD         = 2
	I915ExecBLT         = 3
	I915ExecVEBOX       = 4

	// Misc flags
	I915ExecNoReloc           = 1 << 11
	I915ExecHandleCount        = 1 << 12
	I915ExecBatchFirst        = 1 << 18
	I915ExecFenceIn           = 1 << 16
	I915ExecFenceOut          = 1 << 17
	I915ExecFenceArray        = 1 << 19
	I915ExecFenceSubmit       = 1 << 20
	I915ExecUseExtensions     = 1 << 31
)

// ---- GEM domain flags ----

const (
	I915GEMDomainCPU         = 0x00000001
	I915GEMDomainGTT         = 0x00000002
	I915GEMDomainRender      = 0x00000004
	I915GEMDomainSampler     = 0x00000008
	I915GEMDomainCommand     = 0x00000010
	I915GEMDomainInstruction = 0x00000020
	I915GEMDomainVertexData  = 0x00000040
)

// ---- Tiling modes ----

const (
	I915TilingNone = 0
	I915TilingX    = 1
	I915TilingY    = 2
)

// ---- DRM command base ----

const drmCommandBase = uintptr(0x40)

// ---- i915 ioctl structures (64-bit Linux ABI) ----

type i915GemCreate struct {
	Size   uint64
	Handle uint32
	_      [4]byte
}

type i915GemPRead struct {
	Handle  uint32
	_       [4]byte
	Offset  uint64
	Size    uint64
	DataPtr uint64
}

type i915GemPWrite struct {
	Handle  uint32
	_       [4]byte
	Offset  uint64
	Size    uint64
	DataPtr uint64
}

type i915GemMmap struct {
	Handle  uint32
	_       [4]byte
	Offset  uint64
	Size    uint64
	AddrPtr uint64
	Flags   uint64
}

type i915GemSetDomain struct {
	Handle      uint32
	ReadDomains  uint32
	WriteDomain uint32
}

type i915GemBusy struct {
	Handle uint32
	Busy   uint32
}

type i915GemWait struct {
	BOHandle  uint32
	Flags     uint32
	TimeoutNS int64
}

type i915GemSetTiling struct {
	Handle    uint32
	TilingMode uint32
	Stride    uint32
	SwizzleMode uint32
}

type i915GemGetTiling struct {
	Handle      uint32
	TilingMode  uint32
	SwizzleMode uint32
	PhysSwizzle uint32
}

type i915GetParam struct {
	Param    int32
	_        [4]byte
	ValuePtr uint64 // *int32
}

type i915GemContextCreate struct {
	CtxID uint32
	Pad   uint32
}

type i915GemContextDestroy struct {
	CtxID uint32
	Pad   uint32
}

// ExecObject2 describes a GEM buffer object for submission to ExecBuffer.
// It maps to struct drm_i915_gem_exec_object2.
type ExecObject2 struct {
	Handle          uint32
	RelocationCount uint32
	RelocsPtr       uint64
	Alignment       uint64
	Offset          uint64
	Flags           uint64
	Rsvd1           uint64
	Rsvd2           uint64
}

type i915GemExecBuffer2 struct {
	BuffersPtr       uint64
	BufferCount      uint32
	BatchStartOffset uint32
	BatchLen         uint32
	DR1              uint32
	DR4              uint32
	NumClipRects     uint32
	ClipRectsPtr     uint64
	Flags            uint64
	Rsvd1            uint32
	Rsvd2            uint32
}

type i915GemMmapGTT struct {
	Handle uint32
	Pad    uint32
	Offset uint64
}

// ---- i915 ioctl request codes ----

var (
	ioctlI915GetParam         = iowr('d', drmCommandBase+0x06, unsafe.Sizeof(i915GetParam{}))
	ioctlI915GEMCreate        = iowr('d', drmCommandBase+0x1b, unsafe.Sizeof(i915GemCreate{}))
	ioctlI915GEMPRead         = iowr('d', drmCommandBase+0x1c, unsafe.Sizeof(i915GemPRead{}))
	ioctlI915GEMPWrite        = iowr('d', drmCommandBase+0x1d, unsafe.Sizeof(i915GemPWrite{}))
	ioctlI915GEMMmap          = iowr('d', drmCommandBase+0x1e, unsafe.Sizeof(i915GemMmap{}))
	ioctlI915GEMSetDomain     = iowr('d', drmCommandBase+0x1f, unsafe.Sizeof(i915GemSetDomain{}))
	ioctlI915GEMBusy          = iowr('d', drmCommandBase+0x17, unsafe.Sizeof(i915GemBusy{}))
	ioctlI915GEMWait          = iowr('d', drmCommandBase+0x2c, unsafe.Sizeof(i915GemWait{}))
	ioctlI915GEMSetTiling     = iowr('d', drmCommandBase+0x21, unsafe.Sizeof(i915GemSetTiling{}))
	ioctlI915GEMGetTiling     = iowr('d', drmCommandBase+0x22, unsafe.Sizeof(i915GemGetTiling{}))
	ioctlI915GEMContextCreate = iowr('d', drmCommandBase+0x2d, unsafe.Sizeof(i915GemContextCreate{}))
	ioctlI915GEMContextDestroy = iow('d', drmCommandBase+0x2e, unsafe.Sizeof(i915GemContextDestroy{}))
	ioctlI915GEMExecBuffer2   = iowr('d', drmCommandBase+0x29, unsafe.Sizeof(i915GemExecBuffer2{}))
	ioctlI915GEMMmapGTT       = iowr('d', drmCommandBase+0x24, unsafe.Sizeof(i915GemMmapGTT{}))
	ioctlI915GEMClose         = iowr('d', 0x09, unsafe.Sizeof(gemClose{}))
)

// gemClose maps to struct drm_gem_close (shared by Intel and QXL).
type gemClose struct {
	Handle uint32
	Pad    uint32
}

// ---- IntelDevice ----

// IntelDevice is a handle to an Intel i915 DRI render node.
//
// Obtain one via [Open] or [Probe] and type-assert:
//
//	intel := dev.(*gpu.IntelDevice)
type IntelDevice struct {
	file *os.File
	path string
}

func (d *IntelDevice) Driver() Driver { return DriverIntel }
func (d *IntelDevice) Path() string   { return d.path }
func (d *IntelDevice) Close() error   { return d.file.Close() }

// GetParam queries a driver parameter.  param is one of the I915Param*
// constants.  Returns the int32 value reported by the kernel.
func (d *IntelDevice) GetParam(param int32) (int32, error) {
	var value int32
	p := i915GetParam{
		Param:    param,
		ValuePtr: uint64(uintptr(unsafe.Pointer(&value))),
	}
	if err := ioctl(fd(d.file), ioctlI915GetParam, unsafe.Pointer(&p)); err != nil {
		return 0, fmt.Errorf("i915 GetParam %d: %w", param, err)
	}
	return value, nil
}

// GEMCreate allocates a new GEM buffer of at least size bytes.
// Returns the kernel handle and the actual allocated size.
func (d *IntelDevice) GEMCreate(size uint64) (handle uint32, actualSize uint64, err error) {
	c := i915GemCreate{Size: size}
	if err = ioctl(fd(d.file), ioctlI915GEMCreate, unsafe.Pointer(&c)); err != nil {
		return 0, 0, fmt.Errorf("i915 GEMCreate: %w", err)
	}
	return c.Handle, c.Size, nil
}

// GEMClose releases a GEM buffer handle.
func (d *IntelDevice) GEMClose(handle uint32) error {
	c := gemClose{Handle: handle}
	if err := ioctl(fd(d.file), ioctlI915GEMClose, unsafe.Pointer(&c)); err != nil {
		return fmt.Errorf("i915 GEMClose: %w", err)
	}
	return nil
}

// GEMRead reads size bytes from the GEM buffer starting at offset into dst.
func (d *IntelDevice) GEMRead(handle uint32, offset, size uint64, dst []byte) error {
	if uint64(len(dst)) < size {
		return fmt.Errorf("i915 GEMRead: dst buffer too small")
	}
	p := i915GemPRead{
		Handle:  handle,
		Offset:  offset,
		Size:    size,
		DataPtr: uint64(uintptr(unsafe.Pointer(&dst[0]))),
	}
	if err := ioctl(fd(d.file), ioctlI915GEMPRead, unsafe.Pointer(&p)); err != nil {
		return fmt.Errorf("i915 GEMRead: %w", err)
	}
	return nil
}

// GEMWrite writes src into the GEM buffer starting at offset.
func (d *IntelDevice) GEMWrite(handle uint32, offset uint64, src []byte) error {
	if len(src) == 0 {
		return nil
	}
	p := i915GemPWrite{
		Handle:  handle,
		Offset:  offset,
		Size:    uint64(len(src)),
		DataPtr: uint64(uintptr(unsafe.Pointer(&src[0]))),
	}
	if err := ioctl(fd(d.file), ioctlI915GEMPWrite, unsafe.Pointer(&p)); err != nil {
		return fmt.Errorf("i915 GEMWrite: %w", err)
	}
	return nil
}

// GEMMmapGTT returns the fake GTT offset for use with mmap(2).
// Map the returned offset into the process address space via syscall.Mmap
// on the device file descriptor.
func (d *IntelDevice) GEMMmapGTT(handle uint32) (offset uint64, err error) {
	p := i915GemMmapGTT{Handle: handle}
	if err = ioctl(fd(d.file), ioctlI915GEMMmapGTT, unsafe.Pointer(&p)); err != nil {
		return 0, fmt.Errorf("i915 GEMMmapGTT: %w", err)
	}
	return p.Offset, nil
}

// GEMMmap directly maps a GEM buffer into the process address space
// using the kernel's MMAP ioctl (no GTT indirection).
// Returns the virtual address as a uintptr.
func (d *IntelDevice) GEMMmap(handle uint32, offset, size uint64) (uintptr, error) {
	p := i915GemMmap{
		Handle: handle,
		Offset: offset,
		Size:   size,
	}
	if err := ioctl(fd(d.file), ioctlI915GEMMmap, unsafe.Pointer(&p)); err != nil {
		return 0, fmt.Errorf("i915 GEMMmap: %w", err)
	}
	return uintptr(p.AddrPtr), nil
}

// GEMSetDomain transitions a GEM buffer into the given read/write domains.
// readDomains and writeDomain are combinations of the I915GEMDomain* flags.
func (d *IntelDevice) GEMSetDomain(handle, readDomains, writeDomain uint32) error {
	p := i915GemSetDomain{
		Handle:      handle,
		ReadDomains:  readDomains,
		WriteDomain: writeDomain,
	}
	if err := ioctl(fd(d.file), ioctlI915GEMSetDomain, unsafe.Pointer(&p)); err != nil {
		return fmt.Errorf("i915 GEMSetDomain: %w", err)
	}
	return nil
}

// GEMBusy reports whether the GPU is still rendering to the buffer.
func (d *IntelDevice) GEMBusy(handle uint32) (bool, error) {
	p := i915GemBusy{Handle: handle}
	if err := ioctl(fd(d.file), ioctlI915GEMBusy, unsafe.Pointer(&p)); err != nil {
		return false, fmt.Errorf("i915 GEMBusy: %w", err)
	}
	return p.Busy != 0, nil
}

// GEMWait waits for the GPU to finish with the buffer.
// timeoutNS is a relative timeout in nanoseconds; -1 means wait forever.
func (d *IntelDevice) GEMWait(handle uint32, timeoutNS int64) error {
	p := i915GemWait{BOHandle: handle, TimeoutNS: timeoutNS}
	if err := ioctl(fd(d.file), ioctlI915GEMWait, unsafe.Pointer(&p)); err != nil {
		return fmt.Errorf("i915 GEMWait: %w", err)
	}
	return nil
}

// GEMSetTiling sets the tiling mode for a GEM buffer.
// Returns the actual tiling mode and swizzle mode selected by the kernel.
func (d *IntelDevice) GEMSetTiling(handle, tilingMode, stride uint32) (actualTiling, swizzle uint32, err error) {
	p := i915GemSetTiling{Handle: handle, TilingMode: tilingMode, Stride: stride}
	if err = ioctl(fd(d.file), ioctlI915GEMSetTiling, unsafe.Pointer(&p)); err != nil {
		return 0, 0, fmt.Errorf("i915 GEMSetTiling: %w", err)
	}
	return p.TilingMode, p.SwizzleMode, nil
}

// GEMGetTiling returns the tiling and swizzle modes for a GEM buffer.
func (d *IntelDevice) GEMGetTiling(handle uint32) (tilingMode, swizzleMode uint32, err error) {
	p := i915GemGetTiling{Handle: handle}
	if err = ioctl(fd(d.file), ioctlI915GEMGetTiling, unsafe.Pointer(&p)); err != nil {
		return 0, 0, fmt.Errorf("i915 GEMGetTiling: %w", err)
	}
	return p.TilingMode, p.SwizzleMode, nil
}

// ContextCreate creates a new GPU render context.
// Returns the context ID to pass to ExecBuffer.
func (d *IntelDevice) ContextCreate() (ctxID uint32, err error) {
	p := i915GemContextCreate{}
	if err = ioctl(fd(d.file), ioctlI915GEMContextCreate, unsafe.Pointer(&p)); err != nil {
		return 0, fmt.Errorf("i915 ContextCreate: %w", err)
	}
	return p.CtxID, nil
}

// ContextDestroy destroys a GPU render context.
func (d *IntelDevice) ContextDestroy(ctxID uint32) error {
	p := i915GemContextDestroy{CtxID: ctxID}
	if err := ioctl(fd(d.file), ioctlI915GEMContextDestroy, unsafe.Pointer(&p)); err != nil {
		return fmt.Errorf("i915 ContextDestroy: %w", err)
	}
	return nil
}

// ExecBuffer submits a batch buffer for GPU execution.
//
// objects is the list of GEM buffers referenced by the batch; the last
// entry must be the batch buffer itself (or batchHandle must equal the
// handle in the last object).
//
// batchOffset is the byte offset within the batch buffer where execution
// starts.  batchLen is the length of the batch in bytes.
//
// flags is a combination of the I915Exec* constants that select the
// ring/engine and submission options.  ctxID should be 0 for the
// default context or the value returned by ContextCreate.
func (d *IntelDevice) ExecBuffer(ctxID uint32, objects []ExecObject2,
	batchOffset, batchLen uint32, flags uint64) error {

	if len(objects) == 0 {
		return fmt.Errorf("i915 ExecBuffer: objects list must not be empty")
	}
	p := i915GemExecBuffer2{
		BuffersPtr:       uint64(uintptr(unsafe.Pointer(&objects[0]))),
		BufferCount:      uint32(len(objects)),
		BatchStartOffset: batchOffset,
		BatchLen:         batchLen,
		Flags:            flags,
		Rsvd1:            ctxID,
	}
	if err := ioctl(fd(d.file), ioctlI915GEMExecBuffer2, unsafe.Pointer(&p)); err != nil {
		return fmt.Errorf("i915 ExecBuffer: %w", err)
	}
	return nil
}
