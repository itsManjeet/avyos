package svg

import (
	"encoding/xml"
	"fmt"
	"image"
	stdcolor "image/color"
	"io"
	"math"
	"strconv"
	"strings"
)

func decodeConfig(r io.Reader) (image.Config, error) {
	root, err := parseRoot(r)
	if err != nil {
		return image.Config{}, err
	}
	width, height, err := root.viewportSize()
	if err != nil {
		return image.Config{}, err
	}
	return image.Config{
		ColorModel: stdcolor.RGBAModel,
		Width:      width,
		Height:     height,
	}, nil
}

type rootNode struct {
	attrs map[string]string
}

func parseRoot(r io.Reader) (*rootNode, error) {
	dec := xml.NewDecoder(r)
	for {
		tok, err := dec.Token()
		if err == io.EOF {
			return nil, fmt.Errorf("svg: missing root <svg> element")
		}
		if err != nil {
			return nil, err
		}
		start, ok := tok.(xml.StartElement)
		if !ok {
			continue
		}
		if localName(start.Name.Local) != "svg" {
			return nil, fmt.Errorf("svg: missing root <svg> element")
		}
		attrs := make(map[string]string, len(start.Attr))
		for _, attr := range start.Attr {
			attrs[localName(attr.Name.Local)] = attr.Value
		}
		return &rootNode{attrs: attrs}, nil
	}
}

func (n *rootNode) viewportSize() (int, int, error) {
	vb, hasVB := parseViewBox(n.attrs["viewBox"])
	width, hasWidth := parseLength(n.attrs["width"], 0)
	height, hasHeight := parseLength(n.attrs["height"], 0)
	switch {
	case hasWidth && hasHeight:
	case hasVB:
		width = vb.width
		height = vb.height
	case hasWidth:
		height = width
	case hasHeight:
		width = height
	default:
		return 0, 0, fmt.Errorf("svg: width/height or viewBox required")
	}
	if width <= 0 || height <= 0 {
		return 0, 0, fmt.Errorf("svg: invalid width/height")
	}
	return int(math.Ceil(width)), int(math.Ceil(height)), nil
}

type viewBox struct {
	width  float64
	height float64
}

func parseViewBox(raw string) (viewBox, bool) {
	parts := splitFields(raw)
	if len(parts) != 4 {
		return viewBox{}, false
	}
	width, ok := parseFloat(parts[2])
	if !ok {
		return viewBox{}, false
	}
	height, ok := parseFloat(parts[3])
	if !ok {
		return viewBox{}, false
	}
	if width <= 0 || height <= 0 {
		return viewBox{}, false
	}
	return viewBox{width: width, height: height}, true
}

func parseLength(raw string, percentBase float64) (float64, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0, false
	}
	if strings.HasSuffix(raw, "%") {
		if percentBase == 0 {
			return 0, false
		}
		v, err := strconv.ParseFloat(strings.TrimSuffix(raw, "%"), 64)
		if err != nil {
			return 0, false
		}
		return percentBase * v / 100, true
	}
	unit := ""
	for _, suffix := range []string{"px", "pt", "pc", "mm", "cm", "in", "q"} {
		if strings.HasSuffix(raw, suffix) {
			unit = suffix
			raw = strings.TrimSpace(strings.TrimSuffix(raw, suffix))
			break
		}
	}
	v, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		return 0, false
	}
	switch unit {
	case "", "px":
		return v, true
	case "pt":
		return v * 96 / 72, true
	case "pc":
		return v * 16, true
	case "mm":
		return v * 96 / 25.4, true
	case "cm":
		return v * 96 / 2.54, true
	case "in":
		return v * 96, true
	case "q":
		return v * 96 / (25.4 * 4), true
	default:
		return 0, false
	}
}

func parseFloat(raw string) (float64, bool) {
	v, err := strconv.ParseFloat(strings.TrimSpace(raw), 64)
	return v, err == nil
}

func localName(name string) string {
	if i := strings.IndexByte(name, ':'); i >= 0 {
		return name[i+1:]
	}
	return name
}

func splitFields(raw string) []string {
	return strings.Fields(strings.NewReplacer(",", " ", "\n", " ", "\t", " ", "\r", " ").Replace(strings.TrimSpace(raw)))
}
