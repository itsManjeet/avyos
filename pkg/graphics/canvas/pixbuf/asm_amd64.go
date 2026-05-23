//go:build amd64

package pixbuf

import "unsafe"

//go:noescape
func fillSolidRGBAAsm(dst *byte, n uintptr, color uint32)

// fillRow fills `pixels` pixels starting at pix[off] with the packed RGBA
// color `c` using SSE2. pix must have at least off+pixels*4 bytes.
func fillRow(pix []byte, off, pixels int, c uint32) {
	if pixels <= 0 {
		return
	}
	fillSolidRGBAAsm((*byte)(unsafe.Pointer(&pix[off])), uintptr(pixels*4), c)
}
