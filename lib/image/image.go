package main

/*
#include <stdint.h>
#include <stdlib.h>
#include <string.h>
typedef struct {
	uint8_t *pixels;
	int width;
	int height;
} Image;
*/
import "C"
import (
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"os"
	"unsafe"

	"golang.org/x/image/draw"
)

//export image_open
func image_open(c_path *C.char) *C.Image {
	path := C.GoString(c_path)

	file, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer file.Close()

	img, _, err := image.Decode(file)
	if err != nil {
		return nil
	}

	bounds := img.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()

	rgba := image.NewRGBA(bounds)

	// TODO: better way for rgba format
	draw.Draw(rgba, bounds, img, bounds.Min, draw.Src)

	size := width * height * 4
	pixels := C.malloc(C.size_t(size))
	if pixels == nil {
		return nil
	}

	C.memcpy(pixels, unsafe.Pointer(&rgba.Pix[0]), C.size_t(size))

	c_img := (*C.Image)(C.malloc(C.size_t(unsafe.Sizeof(C.Image{}))))
	c_img.pixels = (*C.uint8_t)(pixels)
	c_img.width = C.int(width)
	c_img.height = C.int(height)

	return c_img
}

//export image_free
func image_free(self *C.Image) {
	if self == nil {
		return
	}
	if self.pixels != nil {
		C.free(unsafe.Pointer(self.pixels))
	}
	C.free(unsafe.Pointer(self))
}

//export image_resize
func image_resize(src *C.Image, width C.int, height C.int) *C.Image {
	if src == nil || src.pixels == nil {
		return nil
	}

	w := int(src.width)
	h := int(src.height)

	// Wrap C memory as Go image (no copy)
	pix := unsafe.Slice((*byte)(unsafe.Pointer(src.pixels)), w*h*4)

	srcImg := &image.RGBA{
		Pix:    pix,
		Stride: w * 4,
		Rect:   image.Rect(0, 0, w, h),
	}

	dstRect := image.Rect(0, 0, int(width), int(height))
	dstImg := image.NewRGBA(dstRect)

	// High quality resize
	draw.CatmullRom.Scale(
		dstImg,
		dstRect,
		srcImg,
		srcImg.Rect,
		draw.Over,
		nil,
	)

	size := int(width) * int(height) * 4
	dstPixels := C.malloc(C.size_t(size))
	if dstPixels == nil {
		return nil
	}

	C.memcpy(dstPixels, unsafe.Pointer(&dstImg.Pix[0]), C.size_t(size))

	out := (*C.Image)(C.malloc(C.size_t(unsafe.Sizeof(C.Image{}))))
	out.pixels = (*C.uint8_t)(dstPixels)
	out.width = width
	out.height = height

	return out
}

func main() {

}
