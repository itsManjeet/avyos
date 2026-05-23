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

// Image renders a decoded image.Image into its allocated box.
//
//	widget.Image{Source: decoded, Fit: widget.ImageFitContain}
package widget

import (
	"image"
	"math"
	"os"
	"path/filepath"
	"strings"

	_ "image/jpeg"
	_ "image/png"

	"avyos.dev/pkg/graphics/geom"
	"avyos.dev/pkg/graphics/layout"
	"avyos.dev/pkg/graphics/paint"
	"avyos.dev/pkg/graphics/svg"
)

// ImageFit controls how an image is scaled within its box.
type ImageFit int

const (
	// ImageFitContain scales the image to fit within the box while preserving
	// its aspect ratio. The image is centred; empty space is left unfilled.
	ImageFitContain ImageFit = iota
	// ImageFitStretch fills the box exactly, ignoring the aspect ratio.
	ImageFitStretch
)

// Image is a [RenderBox] that renders a decoded image.Image.
// When Source is nil, the widget occupies no space.
type Image struct {
	Source image.Image
	Fit    ImageFit
}

func (im Image) Layout(c layout.BoxConstraints) geom.Size {
	if im.Source == nil {
		return c.Smallest()
	}
	b := im.Source.Bounds()
	return c.Constrain(geom.Sz(float64(b.Dx()), float64(b.Dy())))
}

func (im Image) Paint(ctx *paint.Context, offset geom.Point, size geom.Size) {
	if im.Source == nil {
		return
	}
	dst := geom.NewRect(offset.X, offset.Y, size.Width, size.Height)
	if im.Fit == ImageFitStretch {
		ctx.Canvas.DrawImage(im.Source, dst)
		return
	}
	b := im.Source.Bounds()
	srcW, srcH := float64(b.Dx()), float64(b.Dy())
	if srcW <= 0 || srcH <= 0 || size.Width <= 0 || size.Height <= 0 {
		return
	}
	scale := math.Min(size.Width/srcW, size.Height/srcH)
	drawW := srcW * scale
	drawH := srcH * scale
	ctx.Canvas.DrawImage(im.Source, geom.NewRect(
		offset.X+(size.Width-drawW)/2,
		offset.Y+(size.Height-drawH)/2,
		drawW, drawH,
	))
}

func (im Image) HitTest(pos, offset geom.Point, size geom.Size) bool {
	return geom.NewRect(offset.X, offset.Y, size.Width, size.Height).Contains(pos)
}

// NewImageFromFilePath decodes a JPEG or PNG file at path and returns an Image widget.
func NewImageFromFilePath(path string) (Image, error) {
	file, err := os.Open(path)
	if err != nil {
		return Image{}, err
	}
	defer file.Close()
	img, _, err := image.Decode(file)
	if err != nil {
		if !strings.EqualFold(filepath.Ext(path), ".svg") {
			return Image{}, err
		}
		if _, seekErr := file.Seek(0, 0); seekErr != nil {
			return Image{}, err
		}
		img, err = svg.DecodeFile(path)
		if err != nil {
			return Image{}, err
		}
	}
	return Image{Source: img}, nil
}
