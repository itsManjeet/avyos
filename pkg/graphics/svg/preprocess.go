package svg

import (
	"bytes"
	"encoding/xml"
	"io"
	"regexp"
	"strconv"
	"strings"
)

func preprocessSVG(data []byte) ([]byte, error) {
	roots, err := parseXMLTree(data)
	if err != nil {
		return nil, err
	}

	pre := svgPreprocessor{
		classColors:    map[string]string{},
		gradients:      map[string]*xmlNode{},
		gradientClones: map[string]string{},
		normalized:     map[string]bool{},
	}
	pre.collectMetadata(roots)
	pre.normalizeGradients()

	state := preprocessState{
		currentColor: "",
		opacity:      1.0,
		fill:         paintSpec{raw: "#000000", source: paintInherited},
		stroke:       paintSpec{raw: "none", source: paintInherited},
	}
	for i := range roots {
		pre.rewriteChild(&roots[i], state)
	}
	return encodeXMLTree(roots)
}

type xmlChild struct {
	elem      *xmlNode
	charData  []byte
	comment   []byte
	directive []byte
	procInst  *xml.ProcInst
}

type xmlNode struct {
	parent   *xmlNode
	start    xml.StartElement
	children []xmlChild
}

type paintSource int

const (
	paintInherited paintSource = iota
	paintAttr
	paintStyle
)

type paintSpec struct {
	raw    string
	source paintSource
}

type preprocessState struct {
	currentColor string
	opacity      float64
	fill         paintSpec
	stroke       paintSpec
}

type svgPreprocessor struct {
	classColors    map[string]string
	gradients      map[string]*xmlNode
	gradientClones map[string]string
	normalized     map[string]bool
	nextCloneID    int
}

func parseXMLTree(data []byte) ([]xmlChild, error) {
	dec := xml.NewDecoder(bytes.NewReader(data))
	var roots []xmlChild
	var stack []*xmlNode
	for {
		tok, err := dec.Token()
		if err != nil {
			if err == io.EOF {
				break
			}
			return nil, err
		}

		target := &roots
		if len(stack) != 0 {
			target = &stack[len(stack)-1].children
		}

		switch tok := tok.(type) {
		case xml.StartElement:
			node := &xmlNode{start: cloneStartElement(tok)}
			if len(stack) != 0 {
				node.parent = stack[len(stack)-1]
			}
			*target = append(*target, xmlChild{elem: node})
			stack = append(stack, node)
		case xml.EndElement:
			if len(stack) != 0 {
				stack = stack[:len(stack)-1]
			}
		case xml.CharData:
			*target = append(*target, xmlChild{charData: append([]byte(nil), tok...)})
		case xml.Comment:
			*target = append(*target, xmlChild{comment: append([]byte(nil), tok...)})
		case xml.Directive:
			*target = append(*target, xmlChild{directive: append([]byte(nil), tok...)})
		case xml.ProcInst:
			proc := tok
			proc.Inst = append([]byte(nil), tok.Inst...)
			*target = append(*target, xmlChild{procInst: &proc})
		}
	}
	return roots, nil
}

func encodeXMLTree(children []xmlChild) ([]byte, error) {
	var out bytes.Buffer
	enc := xml.NewEncoder(&out)
	for _, child := range children {
		if err := encodeXMLChild(enc, child); err != nil {
			return nil, err
		}
	}
	if err := enc.Flush(); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}

func encodeXMLChild(enc *xml.Encoder, child xmlChild) error {
	switch {
	case child.elem != nil:
		if err := enc.EncodeToken(child.elem.start); err != nil {
			return err
		}
		for _, grandChild := range child.elem.children {
			if err := encodeXMLChild(enc, grandChild); err != nil {
				return err
			}
		}
		return enc.EncodeToken(child.elem.start.End())
	case child.procInst != nil:
		return enc.EncodeToken(*child.procInst)
	case child.directive != nil:
		return enc.EncodeToken(xml.Directive(child.directive))
	case child.comment != nil:
		return enc.EncodeToken(xml.Comment(child.comment))
	default:
		return enc.EncodeToken(xml.CharData(child.charData))
	}
}

func cloneStartElement(start xml.StartElement) xml.StartElement {
	return xml.StartElement{
		Name: cloneName(start.Name),
		Attr: cloneAttrs(start.Attr),
	}
}

func cloneName(name xml.Name) xml.Name {
	return xml.Name{Space: name.Space, Local: name.Local}
}

func cloneAttrs(attrs []xml.Attr) []xml.Attr {
	cloned := make([]xml.Attr, len(attrs))
	copy(cloned, attrs)
	return cloned
}

func cloneNode(node *xmlNode) *xmlNode {
	if node == nil {
		return nil
	}
	cloned := &xmlNode{
		start: cloneStartElement(node.start),
	}
	for _, child := range node.children {
		next := xmlChild{
			charData:  append([]byte(nil), child.charData...),
			comment:   append([]byte(nil), child.comment...),
			directive: append([]byte(nil), child.directive...),
		}
		if child.procInst != nil {
			proc := *child.procInst
			proc.Inst = append([]byte(nil), child.procInst.Inst...)
			next.procInst = &proc
		}
		if child.elem != nil {
			next.elem = cloneNode(child.elem)
			next.elem.parent = cloned
		}
		cloned.children = append(cloned.children, next)
	}
	return cloned
}

func (pre *svgPreprocessor) collectMetadata(children []xmlChild) {
	for _, child := range children {
		pre.collectChildMetadata(child)
	}
}

func (pre *svgPreprocessor) collectChildMetadata(child xmlChild) {
	if child.elem == nil {
		return
	}
	node := child.elem
	switch localName(node.start.Name.Local) {
	case "style":
		collectClassColors(nodeText(node), pre.classColors)
	case "linearGradient", "radialGradient":
		if id, ok := attrValue(node.start.Attr, "id"); ok && strings.TrimSpace(id) != "" {
			pre.gradients[id] = node
		}
	}
	for _, grandChild := range node.children {
		pre.collectChildMetadata(grandChild)
	}
}

func (pre *svgPreprocessor) normalizeGradients() {
	for id := range pre.gradients {
		pre.normalizeGradient(id, map[string]bool{})
	}
}

func (pre *svgPreprocessor) normalizeGradient(id string, visiting map[string]bool) {
	if pre.normalized[id] || visiting[id] {
		return
	}
	node := pre.gradients[id]
	if node == nil {
		return
	}
	visiting[id] = true
	if href, ok := gradientHref(node.start.Attr); ok && href != "" {
		pre.normalizeGradient(href, visiting)
		base := pre.gradients[href]
		if base != nil {
			copyMissingGradientAttrs(&node.start.Attr, base.start.Attr)
			if !hasStopChildren(node) {
				for _, child := range base.children {
					if child.elem != nil && localName(child.elem.start.Name.Local) == "stop" {
						cloned := cloneNode(child.elem)
						cloned.parent = node
						node.children = append(node.children, xmlChild{elem: cloned})
					}
				}
			}
		}
		removeHrefAttr(&node.start.Attr)
	}
	normalizeGradientStops(node)
	pre.normalized[id] = true
	delete(visiting, id)
}

func copyMissingGradientAttrs(dst *[]xml.Attr, src []xml.Attr) {
	for _, attr := range src {
		if isNamespaceAttr(attr) {
			continue
		}
		key := localName(attr.Name.Local)
		if key == "id" || key == "href" {
			continue
		}
		if _, ok := attrValue(*dst, key); ok {
			continue
		}
		*dst = append(*dst, xml.Attr{Name: cloneName(attr.Name), Value: attr.Value})
	}
}

func normalizeGradientStops(node *xmlNode) {
	for i := range node.children {
		stop := node.children[i].elem
		if stop == nil || localName(stop.start.Name.Local) != "stop" {
			continue
		}
		style, _ := attrValue(stop.start.Attr, "style")
		if color, ok := styleProperty(style, "stop-color"); ok {
			setAttr(&stop.start.Attr, "stop-color", color)
		}
		if opacity, ok := styleProperty(style, "stop-opacity"); ok {
			setAttr(&stop.start.Attr, "stop-opacity", opacity)
		}
	}
}

func hasStopChildren(node *xmlNode) bool {
	for _, child := range node.children {
		if child.elem != nil && localName(child.elem.start.Name.Local) == "stop" {
			return true
		}
	}
	return false
}

func gradientHref(attrs []xml.Attr) (string, bool) {
	for _, attr := range attrs {
		if localName(attr.Name.Local) != "href" {
			continue
		}
		return strings.TrimPrefix(strings.TrimSpace(attr.Value), "#"), true
	}
	return "", false
}

func removeHrefAttr(attrs *[]xml.Attr) {
	for i := 0; i < len(*attrs); i++ {
		if localName((*attrs)[i].Name.Local) == "href" {
			*attrs = append((*attrs)[:i], (*attrs)[i+1:]...)
			i--
		}
	}
}

func isNamespaceAttr(attr xml.Attr) bool {
	return attr.Name.Space == "xmlns" || attr.Name.Local == "xmlns" || strings.HasPrefix(attr.Name.Local, "xmlns:")
}

func (pre *svgPreprocessor) rewriteChild(child *xmlChild, state preprocessState) {
	if child.elem == nil {
		return
	}
	pre.rewriteNode(child.elem, state)
}

func (pre *svgPreprocessor) rewriteNode(node *xmlNode, inherited preprocessState) {
	name := localName(node.start.Name.Local)
	if name == "path" {
		if d, ok := attrValue(node.start.Attr, "d"); ok {
			setAttr(&node.start.Attr, "d", sanitizePathData(d))
		}
	}
	if name == "style" {
		normalizeStyleNode(node)
		return
	}

	currentColor := resolveElementColor(node.start.Attr, pre.classColors, inherited.currentColor)
	rewriteCurrentColorAttrs(node.start.Attr, currentColor)

	elementOpacity := resolveOpacity(node.start.Attr)
	removeAttr(&node.start.Attr, "opacity")
	updateStyle(&node.start.Attr, "opacity", "")
	effectiveOpacity := clamp01(inherited.opacity * elementOpacity)

	fill := resolvePaint(node.start.Attr, "fill", inherited.fill.raw)
	stroke := resolvePaint(node.start.Attr, "stroke", inherited.stroke.raw)
	if supportsPaintOpacity(name) && !approximatelyOne(effectiveOpacity) {
		pre.applyPaintOpacity(&node.start.Attr, "fill", fill, effectiveOpacity)
		pre.applyPaintOpacity(&node.start.Attr, "stroke", stroke, effectiveOpacity)
	}

	childState := preprocessState{
		currentColor: currentColor,
		opacity:      inherited.opacity,
		fill:         paintSpec{raw: fill.raw, source: paintInherited},
		stroke:       paintSpec{raw: stroke.raw, source: paintInherited},
	}
	if opacityInheritsToChildren(name) {
		childState.opacity = effectiveOpacity
	}
	for i := range node.children {
		pre.rewriteChild(&node.children[i], childState)
	}
}

func normalizeStyleNode(node *xmlNode) {
	if strings.TrimSpace(nodeText(node)) != "" {
		return
	}
	node.children = []xmlChild{{charData: []byte(" ")}}
}

func nodeText(node *xmlNode) string {
	var text strings.Builder
	for _, child := range node.children {
		if child.elem == nil && child.charData != nil {
			text.Write(child.charData)
		}
	}
	return text.String()
}

func supportsPaintOpacity(name string) bool {
	switch name {
	case "svg", "g", "defs", "style", "linearGradient", "radialGradient", "stop", "title", "desc", "symbol", "marker", "mask", "pattern", "clipPath", "metadata":
		return false
	default:
		return true
	}
}

func opacityInheritsToChildren(name string) bool {
	switch name {
	case "svg", "g", "symbol", "marker", "mask", "pattern", "clipPath", "a":
		return true
	default:
		return false
	}
}

func resolvePaint(attrs []xml.Attr, key, inherited string) paintSpec {
	paint := paintSpec{raw: inherited, source: paintInherited}
	if value, ok := attrValue(attrs, key); ok {
		paint.raw = strings.TrimSpace(value)
		paint.source = paintAttr
	}
	if style, ok := attrValue(attrs, "style"); ok {
		if value, ok := styleProperty(style, key); ok {
			paint.raw = strings.TrimSpace(value)
			paint.source = paintStyle
		}
	}
	return paint
}

func resolveOpacity(attrs []xml.Attr) float64 {
	opacity := 1.0
	if value, ok := attrValue(attrs, "opacity"); ok {
		if parsed, ok := parseUnitInterval(value); ok {
			opacity = parsed
		}
	}
	if style, ok := attrValue(attrs, "style"); ok {
		if value, ok := styleProperty(style, "opacity"); ok {
			if parsed, ok := parseUnitInterval(value); ok {
				opacity = parsed
			}
		}
	}
	return opacity
}

func (pre *svgPreprocessor) applyPaintOpacity(attrs *[]xml.Attr, key string, paint paintSpec, opacity float64) {
	raw := strings.TrimSpace(paint.raw)
	if raw == "" || strings.EqualFold(raw, "none") {
		return
	}
	if id := urlPaintID(raw); id != "" {
		if cloneID := pre.ensureGradientOpacity(id, opacity); cloneID != "" {
			updatePaintValue(attrs, key, paint.source, "url(#"+cloneID+")")
			return
		}
	}
	multiplyPaintOpacity(attrs, key, opacity)
}

func urlPaintID(raw string) string {
	raw = strings.TrimSpace(raw)
	if !strings.HasPrefix(raw, "url(") || !strings.HasSuffix(raw, ")") {
		return ""
	}
	inner := strings.TrimSpace(raw[4 : len(raw)-1])
	inner = strings.Trim(inner, `"'`)
	if !strings.HasPrefix(inner, "#") || len(inner) == 1 {
		return ""
	}
	return inner[1:]
}

func (pre *svgPreprocessor) ensureGradientOpacity(id string, opacity float64) string {
	if approximatelyOne(opacity) {
		return id
	}
	key := id + "|" + formatFloat(opacity)
	if cloneID, ok := pre.gradientClones[key]; ok {
		return cloneID
	}
	base := pre.gradients[id]
	if base == nil {
		return id
	}

	clone := cloneNode(base)
	pre.nextCloneID++
	cloneID := id + "__opacity_" + strconv.Itoa(pre.nextCloneID)
	setAttr(&clone.start.Attr, "id", cloneID)
	pre.applyGradientStopOpacity(clone, opacity)

	if base.parent != nil {
		clone.parent = base.parent
		base.parent.children = append(base.parent.children, xmlChild{elem: clone})
	}
	pre.gradients[cloneID] = clone
	pre.gradientClones[key] = cloneID
	return cloneID
}

func (pre *svgPreprocessor) applyGradientStopOpacity(node *xmlNode, opacity float64) {
	for i := range node.children {
		stop := node.children[i].elem
		if stop == nil || localName(stop.start.Name.Local) != "stop" {
			continue
		}
		current := 1.0
		if value, ok := attrValue(stop.start.Attr, "stop-opacity"); ok {
			if parsed, ok := parseUnitInterval(value); ok {
				current = parsed
			}
		}
		setAttr(&stop.start.Attr, "stop-opacity", formatFloat(clamp01(current*opacity)))
		updateStyle(&stop.start.Attr, "stop-opacity", "")
	}
}

func updatePaintValue(attrs *[]xml.Attr, key string, source paintSource, value string) {
	if source == paintStyle {
		updateStyle(attrs, key, value)
		return
	}
	setAttr(attrs, key, value)
}

func multiplyPaintOpacity(attrs *[]xml.Attr, key string, factor float64) {
	prop := key + "-opacity"
	style, hasStyle := attrValue(*attrs, "style")
	if value, ok := styleProperty(style, prop); ok {
		current := 1.0
		if parsed, ok := parseUnitInterval(value); ok {
			current = parsed
		}
		updateStyle(attrs, prop, formatFloat(clamp01(current*factor)))
		return
	}
	if value, ok := attrValue(*attrs, prop); ok {
		current := 1.0
		if parsed, ok := parseUnitInterval(value); ok {
			current = parsed
		}
		setAttr(attrs, prop, formatFloat(clamp01(current*factor)))
		return
	}
	if hasStyle && strings.TrimSpace(style) == "" {
		removeAttr(attrs, "style")
	}
	setAttr(attrs, prop, formatFloat(clamp01(factor)))
}

func parseUnitInterval(raw string) (float64, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0, false
	}
	if strings.HasSuffix(raw, "%") {
		v, err := strconv.ParseFloat(strings.TrimSpace(strings.TrimSuffix(raw, "%")), 64)
		if err != nil {
			return 0, false
		}
		return clamp01(v / 100.0), true
	}
	v, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		return 0, false
	}
	return clamp01(v), true
}

func clamp01(v float64) float64 {
	switch {
	case v < 0:
		return 0
	case v > 1:
		return 1
	default:
		return v
	}
}

func approximatelyOne(v float64) bool {
	return 0.999999 <= v
}

func formatFloat(v float64) string {
	return strconv.FormatFloat(v, 'f', -1, 64)
}

func attrValue(attrs []xml.Attr, key string) (string, bool) {
	for _, attr := range attrs {
		if localName(attr.Name.Local) == key {
			return attr.Value, true
		}
	}
	return "", false
}

func setAttr(attrs *[]xml.Attr, key, value string) {
	for i := range *attrs {
		if localName((*attrs)[i].Name.Local) == key {
			(*attrs)[i].Value = value
			return
		}
	}
	*attrs = append(*attrs, xml.Attr{Name: xml.Name{Local: key}, Value: value})
}

func removeAttr(attrs *[]xml.Attr, key string) {
	for i := 0; i < len(*attrs); i++ {
		if localName((*attrs)[i].Name.Local) == key {
			*attrs = append((*attrs)[:i], (*attrs)[i+1:]...)
			i--
		}
	}
}

type styleDecl struct {
	key   string
	value string
}

func parseStyleDecls(style string) []styleDecl {
	if style == "" {
		return nil
	}
	parts := strings.Split(style, ";")
	decls := make([]styleDecl, 0, len(parts))
	for _, part := range parts {
		key, value, ok := strings.Cut(part, ":")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		if key == "" {
			continue
		}
		decls = append(decls, styleDecl{
			key:   strings.ToLower(key),
			value: strings.TrimSpace(value),
		})
	}
	return decls
}

func joinStyleDecls(decls []styleDecl) string {
	if len(decls) == 0 {
		return ""
	}
	parts := make([]string, 0, len(decls))
	for _, decl := range decls {
		if decl.key == "" {
			continue
		}
		parts = append(parts, decl.key+":"+decl.value)
	}
	return strings.Join(parts, ";")
}

func styleProperty(style, key string) (string, bool) {
	for _, decl := range parseStyleDecls(style) {
		if decl.key == strings.ToLower(strings.TrimSpace(key)) {
			return decl.value, true
		}
	}
	return "", false
}

func updateStyle(attrs *[]xml.Attr, key, value string) {
	style, _ := attrValue(*attrs, "style")
	decls := parseStyleDecls(style)
	lowerKey := strings.ToLower(strings.TrimSpace(key))
	found := false
	out := decls[:0]
	for _, decl := range decls {
		if decl.key != lowerKey {
			out = append(out, decl)
			continue
		}
		found = true
		if value != "" {
			out = append(out, styleDecl{key: lowerKey, value: value})
		}
	}
	if !found && value != "" {
		out = append(out, styleDecl{key: lowerKey, value: value})
	}
	if joined := joinStyleDecls(out); joined != "" {
		setAttr(attrs, "style", joined)
	} else {
		removeAttr(attrs, "style")
	}
}

func sanitizePathData(d string) string {
	if d == "" {
		return "M0 0"
	}
	d = strings.ReplaceAll(d, "NaN", "0")
	d = strings.ReplaceAll(d, "nan", "0")
	d = strings.TrimSpace(d)
	for len(d) > 0 {
		last := d[len(d)-1]
		if !isPathCommand(last) {
			break
		}
		d = strings.TrimRight(d[:len(d)-1], " \t\r\n,")
	}
	if d == "" {
		return "M0 0"
	}
	return d
}

func isPathCommand(b byte) bool {
	switch b {
	case 'M', 'm', 'Z', 'z', 'L', 'l', 'H', 'h', 'V', 'v', 'C', 'c', 'S', 's', 'Q', 'q', 'T', 't', 'A', 'a':
		return true
	default:
		return false
	}
}

var classSelectorRe = regexp.MustCompile(`\.([A-Za-z0-9_-]+)`)

func collectClassColors(css string, classColors map[string]string) {
	for _, block := range strings.Split(css, "}") {
		selectors, decls, ok := strings.Cut(block, "{")
		if !ok {
			continue
		}
		color := extractDeclColor(decls)
		if color == "" {
			continue
		}
		matches := classSelectorRe.FindAllStringSubmatch(selectors, -1)
		for _, match := range matches {
			if len(match) == 2 {
				classColors[match[1]] = color
			}
		}
	}
}

func resolveElementColor(attrs []xml.Attr, classColors map[string]string, inherited string) string {
	color := inherited
	classList := ""
	styleValue := ""
	for _, attr := range attrs {
		switch localName(attr.Name.Local) {
		case "class":
			classList = attr.Value
		case "style":
			styleValue = attr.Value
		case "color":
			if normalized := normalizeColor(attr.Value); normalized != "" {
				color = normalized
			}
		}
	}
	for _, className := range strings.Fields(classList) {
		if classColor, ok := classColors[className]; ok {
			color = classColor
		}
	}
	if styleColor := extractDeclColor(styleValue); styleColor != "" {
		color = styleColor
	}
	return color
}

func rewriteCurrentColorAttrs(attrs []xml.Attr, resolved string) {
	if resolved == "" {
		return
	}
	for i := range attrs {
		switch localName(attrs[i].Name.Local) {
		case "fill", "stroke", "stop-color", "flood-color", "lighting-color":
			if strings.EqualFold(strings.TrimSpace(attrs[i].Value), "currentColor") {
				attrs[i].Value = resolved
			}
		case "style":
			attrs[i].Value = rewriteStyleCurrentColor(attrs[i].Value, resolved)
		}
	}
}

func rewriteStyleCurrentColor(style, resolved string) string {
	if !strings.Contains(style, "currentColor") {
		return style
	}
	parts := strings.Split(style, ";")
	for i, part := range parts {
		key, value, ok := strings.Cut(part, ":")
		if !ok {
			continue
		}
		if strings.EqualFold(strings.TrimSpace(value), "currentColor") {
			parts[i] = key + ":" + resolved
			continue
		}
		if strings.Contains(value, "currentColor") {
			parts[i] = key + ":" + strings.ReplaceAll(value, "currentColor", resolved)
		}
	}
	return strings.Join(parts, ";")
}

func extractDeclColor(style string) string {
	for _, part := range strings.Split(style, ";") {
		key, value, ok := strings.Cut(part, ":")
		if !ok || strings.TrimSpace(strings.ToLower(key)) != "color" {
			continue
		}
		if normalized := normalizeColor(value); normalized != "" {
			return normalized
		}
	}
	return ""
}

func normalizeColor(value string) string {
	value = strings.TrimSpace(value)
	if value == "" || strings.EqualFold(value, "currentColor") {
		return ""
	}
	return value
}
