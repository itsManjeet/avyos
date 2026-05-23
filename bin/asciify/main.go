/*
 * Copyright (c) 2026 Manjeet Singh <itsmanjeet1998@gmail.com>.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU General Public License as published by
 * the Free Software Foundation, version 3.
 *
 * This program is distributed in the hope that it will be useful, but
 * WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the GNU
 * General Public License for more details.
 *
 * You should have received a copy of the GNU General Public License
 * along with this program. If not, see <http://www.gnu.org/licenses/>.
 *
 */

package main

import (
	"fmt"
	"image"
	"image/color"
	_ "image/jpeg"
	_ "image/png"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"avyos.dev/lib/graphics/svg"
)

// Dark → light characters
const asciiRamp = "@%#*+=-:. "

func main() {
	if len(os.Args) < 2 {
		fmt.Println("usage: img2ascii <image> [width]")
		os.Exit(1)
	}

	path := os.Args[1]
	width := 80
	if len(os.Args) >= 3 {
		if w, err := strconv.Atoi(os.Args[2]); err == nil && w > 0 {
			width = w
		}
	}

	img, err := loadImage(path)
	if err != nil {
		panic(err)
	}

	ascii := imageToASCII(img, width)
	fmt.Print(ascii)
}

func loadImage(path string) (image.Image, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	img, _, err := image.Decode(f)
	if err == nil {
		return img, nil
	}
	if !strings.EqualFold(filepath.Ext(path), ".svg") {
		return nil, err
	}
	if _, seekErr := f.Seek(0, 0); seekErr != nil {
		return nil, err
	}
	return svg.DecodeFile(path)
}

func imageToASCII(img image.Image, outWidth int) string {
	bounds := img.Bounds()
	w := bounds.Dx()
	h := bounds.Dy()

	// Terminal characters are taller than wide
	aspect := float64(h) / float64(w)
	outHeight := int(float64(outWidth) * aspect * 0.55)

	var result strings.Builder
	for y := range outHeight {
		for x := range outWidth {
			srcX := x * w / outWidth
			srcY := y * h / outHeight

			gray := color.GrayModel.Convert(img.At(srcX, srcY)).(color.Gray)
			result.WriteString(grayToChar(gray.Y))
		}
		result.WriteString("\n")
	}
	return result.String()
}

func grayToChar(v uint8) string {
	rampLen := len(asciiRamp)
	index := int(v) * (rampLen - 1) / 255
	return string(asciiRamp[index])
}
