//go:build !amd64 && !arm64

package pixbuf

// fillRow fills `pixels` pixels starting at pix[off] with the packed RGBA
// value c = r | g<<8 | b<<16 | a<<24.
func fillRow(pix []byte, off, pixels int, c uint32) {
	r := byte(c)
	g := byte(c >> 8)
	b := byte(c >> 16)
	a := byte(c >> 24)
	end := off + pixels*4
	for i := off; i < end; i += 4 {
		pix[i+0] = r
		pix[i+1] = g
		pix[i+2] = b
		pix[i+3] = a
	}
}
