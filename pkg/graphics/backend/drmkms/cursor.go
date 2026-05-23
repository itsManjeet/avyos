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

package drmkms

import (
	"errors"
	"image"
	stddraw "image/draw"
	"syscall"
	"unsafe"

	"avyos.dev/pkg/fs"
	"avyos.dev/pkg/graphics/event"
	"avyos.dev/pkg/graphics/svg"
)

var drmIoctlCursor = iowr('d', 0xA3, unsafe.Sizeof(drmModeCursor{}))
var drmIoctlCursor2 = iowr('d', 0xBB, unsafe.Sizeof(drmModeCursor2{}))

const (
	drmCursorBO   = 0x01 // set cursor buffer
	drmCursorMove = 0x02 // move cursor
)

type drmModeCursor struct {
	Flags  uint32
	CrtcID uint32
	X      int32
	Y      int32
	Width  uint32
	Height uint32
	Handle uint32
}

type drmModeCursor2 struct {
	Flags  uint32
	CrtcID uint32
	X      int32
	Y      int32
	Width  uint32
	Height uint32
	Handle uint32
	HotX   int32
	HotY   int32
}

const cursorSize = 64

type hwCursor struct {
	fd         uintptr
	crtcID     uint32
	handle     uint32
	data       []byte
	x          int32
	y          int32
	hotX       int32
	hotY       int32
	shape      event.CursorShape
	useCursor2 bool
	active     bool
}

func newHWCursor(fd uintptr, crtcID uint32, useCursor2 bool) *hwCursor {
	c := &hwCursor{fd: fd, crtcID: crtcID, useCursor2: useCursor2}

	create := drmModeCreateDumb{
		Width:  cursorSize,
		Height: cursorSize,
		Bpp:    32,
	}
	if err := ioctl(fd, drmIoctlCreateDumb, unsafe.Pointer(&create)); err != nil {
		return nil
	}
	c.handle = create.Handle

	mapReq := drmModeMapDumb{Handle: create.Handle}
	if err := ioctl(fd, drmIoctlMapDumb, unsafe.Pointer(&mapReq)); err != nil {
		d := drmModeDestroyDumb{Handle: create.Handle}
		_ = ioctl(fd, drmIoctlDestroyDumb, unsafe.Pointer(&d))
		return nil
	}

	data, err := syscall.Mmap(int(fd), int64(mapReq.Offset), int(create.Size),
		syscall.PROT_READ|syscall.PROT_WRITE, syscall.MAP_SHARED)
	if err != nil {
		d := drmModeDestroyDumb{Handle: create.Handle}
		_ = ioctl(fd, drmIoctlDestroyDumb, unsafe.Pointer(&d))
		return nil
	}
	c.data = data

	c.shape = event.CursorDefault
	c.hotX = 0
	c.hotY = 0
	drawCursorShape(data, int(create.Pitch), c.shape)

	if err := c.bind(); err != nil {
		log.Debug("failed to bind hardware cursor on CRTC %d: %v", crtcID, err)
		_ = syscall.Munmap(data)
		d := drmModeDestroyDumb{Handle: create.Handle}
		_ = ioctl(fd, drmIoctlDestroyDumb, unsafe.Pointer(&d))
		return nil
	}
	c.active = true
	return c
}

func (c *hwCursor) move(x, y int32) {
	if !c.active {
		return
	}
	c.x = x
	c.y = y
	if err := c.moveOnly(x, y); err != nil {
		log.Debug("failed to move hardware cursor on CRTC %d: %v", c.crtcID, err)
	}
}

func (c *hwCursor) reapply() error {
	if !c.active {
		return nil
	}
	if err := c.bind(); err != nil {
		return err
	}
	return c.moveOnly(c.x, c.y)
}

func (c *hwCursor) setShape(shape event.CursorShape) error {
	if !c.active || c.data == nil {
		return nil
	}
	if !c.useCursor2 && shape != event.CursorDefault {
		shape = event.CursorDefault
	}
	if c.shape == shape {
		return nil
	}
	c.shape = shape
	c.hotX, c.hotY = drawCursorShape(c.data, cursorSize*4, shape)
	if err := c.bind(); err != nil {
		return err
	}
	return c.moveOnly(c.x, c.y)
}

func (c *hwCursor) destroy() {
	if !c.active {
		return
	}
	// Hide cursor by setting handle 0.
	if err := c.hide(); err != nil {
		log.Debug("failed to hide hardware cursor on CRTC %d: %v", c.crtcID, err)
	}

	if c.data != nil {
		_ = syscall.Munmap(c.data)
	}
	if c.handle != 0 {
		d := drmModeDestroyDumb{Handle: c.handle}
		_ = ioctl(c.fd, drmIoctlDestroyDumb, unsafe.Pointer(&d))
	}
	c.active = false
}

func (c *hwCursor) bind() error {
	if c.useCursor2 {
		cur := drmModeCursor2{
			Flags:  drmCursorBO,
			CrtcID: c.crtcID,
			Width:  cursorSize,
			Height: cursorSize,
			Handle: c.handle,
			HotX:   c.hotX,
			HotY:   c.hotY,
		}
		if err := ioctl(c.fd, drmIoctlCursor2, unsafe.Pointer(&cur)); err != nil {
			if errors.Is(err, syscall.ENOTTY) || errors.Is(err, syscall.EINVAL) || errors.Is(err, syscall.EOPNOTSUPP) {
				c.useCursor2 = false
			} else {
				return err
			}
		} else {
			return nil
		}
	}

	cur := drmModeCursor{
		Flags:  drmCursorBO,
		CrtcID: c.crtcID,
		Width:  cursorSize,
		Height: cursorSize,
		Handle: c.handle,
	}
	return ioctl(c.fd, drmIoctlCursor, unsafe.Pointer(&cur))
}

func (c *hwCursor) moveOnly(x, y int32) error {
	if c.useCursor2 {
		cur := drmModeCursor2{
			Flags:  drmCursorMove,
			CrtcID: c.crtcID,
			X:      x,
			Y:      y,
		}
		if err := ioctl(c.fd, drmIoctlCursor2, unsafe.Pointer(&cur)); err != nil {
			if !errors.Is(err, syscall.ENOTTY) && !errors.Is(err, syscall.EINVAL) && !errors.Is(err, syscall.EOPNOTSUPP) {
				return err
			}
			c.useCursor2 = false
		} else {
			return nil
		}
	}

	cur := drmModeCursor{
		Flags:  drmCursorMove,
		CrtcID: c.crtcID,
		X:      x,
		Y:      y,
	}
	return ioctl(c.fd, drmIoctlCursor, unsafe.Pointer(&cur))
}

func (c *hwCursor) hide() error {
	if c.useCursor2 {
		cur := drmModeCursor2{
			Flags:  drmCursorBO,
			CrtcID: c.crtcID,
		}
		if err := ioctl(c.fd, drmIoctlCursor2, unsafe.Pointer(&cur)); err != nil {
			if !errors.Is(err, syscall.ENOTTY) && !errors.Is(err, syscall.EINVAL) && !errors.Is(err, syscall.EOPNOTSUPP) {
				return err
			}
			c.useCursor2 = false
		} else {
			return nil
		}
	}

	cur := drmModeCursor{
		Flags:  drmCursorBO,
		CrtcID: c.crtcID,
	}
	return ioctl(c.fd, drmIoctlCursor, unsafe.Pointer(&cur))
}

func cursorSVGInfo(shape event.CursorShape) (string, int32, int32) {
	switch shape {
	case event.CursorText:
		return "text.svg", cursorSize / 2, cursorSize / 2
	case event.CursorMove:
		return "fleur.svg", cursorSize / 2, cursorSize / 2
	case event.CursorResizeNS:
		return "size_ver.svg", cursorSize / 2, cursorSize / 2
	case event.CursorResizeEW:
		return "size_hor.svg", cursorSize / 2, cursorSize / 2
	case event.CursorResizeNWSE:
		return "size_fdiag.svg", cursorSize / 2, cursorSize / 2
	case event.CursorResizeNESW:
		return "size_bdiag.svg", cursorSize / 2, cursorSize / 2
	default:
		return "default.svg", 0, 0
	}
}

func renderCursorFromSVG(data []byte, pitch int, shape event.CursorShape) (int32, int32, bool) {
	name, hotX, hotY := cursorSVGInfo(shape)
	path := fs.Resolve("data:icons/default/cursor/%s", name)
	img, err := svg.DecodeSizedFile(path, cursorSize, cursorSize)
	if err != nil {
		return 0, 0, false
	}
	rgba, ok := img.(*image.RGBA)
	if !ok {
		rgba = image.NewRGBA(img.Bounds())
		stddraw.Draw(rgba, rgba.Bounds(), img, image.Point{}, stddraw.Src)
	}
	for y := 0; y < cursorSize; y++ {
		for x := 0; x < cursorSize; x++ {
			src := y*rgba.Stride + x*4
			dst := y*pitch + x*4
			if src+3 >= len(rgba.Pix) || dst+3 >= len(data) {
				continue
			}
			data[dst+0] = rgba.Pix[src+2] // B
			data[dst+1] = rgba.Pix[src+1] // G
			data[dst+2] = rgba.Pix[src+0] // R
			data[dst+3] = rgba.Pix[src+3] // A
		}
	}
	return hotX, hotY, true
}

func drawCursorShape(data []byte, pitch int, shape event.CursorShape) (int32, int32) {
	clearCursor(data)
	if hx, hy, ok := renderCursorFromSVG(data, pitch, shape); ok {
		return hx, hy
	}
	drawArrowCursor(data, pitch)
	return 0, 0
}

func clearCursor(data []byte) {
	// Clear to fully transparent.
	for i := range data {
		data[i] = 0
	}
}

// drawArrowCursor renders a standard arrow pointer into an ARGB8888 buffer.
//
// Bitmap key:  'X' = black outline, 'o' = white fill, ' ' = transparent.
func drawArrowCursor(data []byte, pitch int) {
	rows := []string{
		"XoX,            ",
		"XooX,           ",
		"XoooX,          ",
		"XooooX,         ",
		"XoooooX,        ",
		"XooooooX,       ",
		"XoooooooX,      ",
		"XooooooooX,     ",
		"XoooooooooX,    ",
		"XooooooXXXX,    ",
		"XooX            ",
		"XoX.            ",
		"XX.             ",
	}
	for y, row := range rows {
		for x, ch := range row {
			switch ch {
			case 'X':
				setARGB(data, pitch, x, y, 0xFF, 0x00, 0x00, 0x00)
			case 'o':
				setARGB(data, pitch, x, y, 0xFF, 0xFF, 0xFF, 0xFF)
			case '.':
				setARGB(data, pitch, x, y, 0x60, 0x00, 0x00, 0x00)
			case ',':
				setARGB(data, pitch, x, y, 0x30, 0x00, 0x00, 0x00)
			}
		}
	}
}

func drawCursorPixel(data []byte, pitch, x, y int, outline bool) {
	if outline {
		setARGB(data, pitch, x, y, 0xFF, 0x00, 0x00, 0x00)
		return
	}
	setARGB(data, pitch, x, y, 0xFF, 0xFF, 0xFF, 0xFF)
}

// setARGB writes a single ARGB8888 pixel (LE byte order: B, G, R, A).
func setARGB(data []byte, pitch, x, y int, a, r, g, b byte) {
	off := y*pitch + x*4
	if off+3 >= len(data) {
		return
	}
	data[off+0] = b
	data[off+1] = g
	data[off+2] = r
	data[off+3] = a
}
