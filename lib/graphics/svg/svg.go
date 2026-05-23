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

// Package svg provides a pure-Go SVG decoder that integrates with image.Decode.
package svg

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	"image/draw"
	"io"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

func init() {
	image.RegisterFormat("svg", "<svg", Decode, DecodeConfig)
	image.RegisterFormat("svg", "<?xml", Decode, DecodeConfig)
}

// Decode parses an SVG document and rasterizes it into an RGBA image.
func Decode(r io.Reader) (image.Image, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}
	cfg, err := decodeConfig(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	doc, err := Parse(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	img := image.NewRGBA(image.Rect(0, 0, cfg.Width, cfg.Height))
	if err := doc.render(img, false); err != nil {
		return nil, err
	}
	return img, nil
}

// DecodeFile decodes an SVG file from disk.
// If the file is an icon-theme alias entry, it resolves the referenced file first.
func DecodeFile(path string) (image.Image, error) {
	resolved, data, err := loadFileData(path)
	if err != nil {
		return nil, err
	}
	_ = resolved
	return Decode(bytes.NewReader(data))
}

// DecodeSizedFile decodes an SVG file from disk and rasterizes it into the
// requested pixel size, preserving the source aspect ratio and centering it
// within the destination bounds.
func DecodeSizedFile(path string, width, height int) (image.Image, error) {
	if width <= 0 || height <= 0 {
		return DecodeFile(path)
	}
	resolved, data, err := loadFileData(path)
	if err != nil {
		return nil, err
	}
	_ = resolved

	cfg, err := decodeConfig(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}

	dst := image.NewRGBA(image.Rect(0, 0, width, height))
	drawW, drawH, offX, offY := fitRect(cfg.Width, cfg.Height, width, height)
	if drawW <= 0 || drawH <= 0 {
		return dst, nil
	}
	tmp := image.NewRGBA(image.Rect(0, 0, drawW, drawH))
	if err := Render(tmp, bytes.NewReader(data)); err != nil {
		return nil, err
	}
	draw.Draw(dst, image.Rect(offX, offY, offX+drawW, offY+drawH), tmp, image.Point{}, draw.Src)
	return dst, nil
}

// DecodeConfig parses the SVG header and returns the intrinsic image size.
func DecodeConfig(r io.Reader) (image.Config, error) {
	return decodeConfig(r)
}

// DecodeConfigFile parses an SVG file on disk and returns its intrinsic size.
func DecodeConfigFile(path string) (image.Config, error) {
	_, data, err := loadFileData(path)
	if err != nil {
		return image.Config{}, err
	}
	return decodeConfig(bytes.NewReader(data))
}

// Render rasterizes an SVG into the provided draw.Image.
// The document is scaled to the destination bounds when they differ from the
// SVG's intrinsic pixel size.
func Render(dst draw.Image, r io.Reader) error {
	data, err := io.ReadAll(r)
	if err != nil {
		return err
	}
	if _, err := decodeConfig(bytes.NewReader(data)); err != nil {
		return err
	}
	doc, err := Parse(bytes.NewReader(data))
	if err != nil {
		return err
	}
	return doc.render(dst, true)
}

func loadFileData(path string) (string, []byte, error) {
	resolved, err := resolveAliasPath(path)
	if err != nil {
		return "", nil, err
	}
	data, err := os.ReadFile(resolved)
	if err != nil {
		return "", nil, err
	}
	data = normalizeThemeFallback(path, resolved, data)
	return resolved, data, nil
}

func normalizeThemeFallback(requestedPath, resolvedPath string, data []byte) []byte {
	requestedTheme, ok := iconThemeName(requestedPath)
	if !ok {
		return data
	}
	resolvedTheme, ok := iconThemeName(resolvedPath)
	if !ok || requestedTheme == resolvedTheme {
		return data
	}

	replacements := themeFallbackColorMap(requestedTheme, resolvedTheme)
	if len(replacements) == 0 {
		return data
	}

	normalized := string(data)
	changed := false
	for from, to := range replacements {
		next := replaceHexColor(normalized, from, to)
		if next != normalized {
			changed = true
			normalized = next
		}
	}
	if !changed {
		return data
	}
	return []byte(normalized)
}

func iconThemeName(path string) (string, bool) {
	parts := strings.Split(filepath.Clean(path), string(filepath.Separator))
	for i := 0; i+2 < len(parts); i++ {
		if parts[i] == "data" && parts[i+1] == "icons" && parts[i+2] != "" {
			return parts[i+2], true
		}
	}
	return "", false
}

func themeFallbackColorMap(requestedTheme, resolvedTheme string) map[string]string {
	switch {
	case requestedTheme == "default" && resolvedTheme == "default-dark":
		return map[string]string{
			"#aaaaaa": "#505050",
			"#aaa":    "#505050",
		}
	case requestedTheme == "default-dark" && resolvedTheme == "default":
		return map[string]string{
			"#505050": "#aaaaaa",
			"#565656": "#aaaaaa",
			"#333333": "#aaaaaa",
		}
	default:
		return nil
	}
}

func replaceHexColor(s, from, to string) string {
	pattern := regexp.MustCompile(`(?i)` + regexp.QuoteMeta(from) + `\b`)
	return pattern.ReplaceAllString(s, to)
}

func resolveAliasPath(path string) (string, error) {
	seen := map[string]struct{}{}
	current := path
	for range 32 {
		clean := filepath.Clean(current)
		if _, ok := seen[clean]; ok {
			return "", fmt.Errorf("svg: alias cycle detected at %s", clean)
		}
		seen[clean] = struct{}{}

		data, err := os.ReadFile(clean)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				if alt, ok := resolveInSiblingThemes(clean); ok {
					current = alt
					continue
				}
			}
			return "", err
		}
		trim := strings.TrimSpace(string(data))
		if trim == "" || trim[0] == '<' {
			return clean, nil
		}
		if filepath.IsAbs(trim) {
			current = trim
		} else {
			current = filepath.Join(filepath.Dir(clean), trim)
		}
	}
	return "", fmt.Errorf("svg: alias depth exceeded for %s", path)
}

func resolveInSiblingThemes(path string) (string, bool) {
	clean := filepath.Clean(path)
	dir := filepath.Dir(clean)
	for {
		parent := filepath.Dir(dir)
		if filepath.Base(parent) == "icons" {
			rel, err := filepath.Rel(dir, clean)
			if err != nil {
				return "", false
			}
			entries, err := os.ReadDir(parent)
			if err != nil {
				return "", false
			}
			for _, entry := range entries {
				if !entry.IsDir() || entry.Name() == filepath.Base(dir) {
					continue
				}
				candidate := filepath.Join(parent, entry.Name(), rel)
				info, err := os.Stat(candidate)
				if err == nil && !info.IsDir() {
					return candidate, true
				}
			}
			return "", false
		}
		if parent == dir {
			return "", false
		}
		dir = parent
	}
}

func fitRect(srcW, srcH, dstW, dstH int) (drawW, drawH, offX, offY int) {
	if srcW <= 0 || srcH <= 0 || dstW <= 0 || dstH <= 0 {
		return 0, 0, 0, 0
	}
	scale := math.Min(float64(dstW)/float64(srcW), float64(dstH)/float64(srcH))
	if scale <= 0 {
		return 0, 0, 0, 0
	}
	drawW = int(math.Round(float64(srcW) * scale))
	drawH = int(math.Round(float64(srcH) * scale))
	if drawW < 1 {
		drawW = 1
	}
	if drawH < 1 {
		drawH = 1
	}
	offX = (dstW - drawW) / 2
	offY = (dstH - drawH) / 2
	return drawW, drawH, offX, offY
}
