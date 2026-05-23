package svg

import (
	"bytes"
	"encoding/xml"
	"io"
	"strings"
)

type xmlChild struct {
	elem     *xmlNode
	charData []byte
}

type xmlNode struct {
	parent   *xmlNode
	start    xml.StartElement
	children []xmlChild
}

func parseXMLTree(data []byte) ([]xmlChild, error) {
	dec := xml.NewDecoder(bytes.NewReader(data))
	dec.Strict = true
	var roots []xmlChild
	var stack []*xmlNode
	for {
		tok, err := dec.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		target := &roots
		if len(stack) != 0 {
			target = &stack[len(stack)-1].children
		}
		switch tok := tok.(type) {
		case xml.StartElement:
			n := &xmlNode{start: tok}
			if len(stack) != 0 {
				n.parent = stack[len(stack)-1]
			}
			*target = append(*target, xmlChild{elem: n})
			stack = append(stack, n)
		case xml.EndElement:
			if len(stack) != 0 {
				stack = stack[:len(stack)-1]
			}
		case xml.CharData:
			*target = append(*target, xmlChild{charData: append([]byte(nil), tok...)})
		}
	}
	return roots, nil
}

func nodeText(n *xmlNode) string {
	var b strings.Builder
	var appendText func(*xmlNode)
	appendText = func(node *xmlNode) {
		for _, child := range node.children {
			if child.elem != nil {
				appendText(child.elem)
			} else {
				b.Write(child.charData)
			}
		}
	}
	appendText(n)
	return b.String()
}

func attrValue(attrs []xml.Attr, key string) (string, bool) {
	for _, attr := range attrs {
		if localName(attr.Name.Local) == key {
			return attr.Value, true
		}
	}
	return "", false
}

func clamp01(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}
