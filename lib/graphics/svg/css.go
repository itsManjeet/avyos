package svg

import (
	"sort"
	"strings"
	"unicode"
)

type cssRule struct {
	selector    string
	decls       []cssDeclaration
	specificity int
	order       int
}
type cssDeclaration struct {
	name, value string
	important   bool
}

type computedStyle map[string]string

var presentationProperties = map[string]bool{
	"alignment-baseline": true, "baseline-shift": true, "clip-path": true, "clip-rule": true, "color": true,
	"color-interpolation": true, "display": true, "dominant-baseline": true, "fill": true, "fill-opacity": true,
	"fill-rule": true, "filter": true, "font-family": true, "font-size": true, "font-style": true, "font-weight": true,
	"marker": true, "marker-end": true, "marker-mid": true, "marker-start": true, "mask": true, "opacity": true,
	"overflow": true, "paint-order": true, "shape-rendering": true, "stop-color": true, "stop-opacity": true,
	"stroke": true, "stroke-dasharray": true, "stroke-dashoffset": true, "stroke-linecap": true, "stroke-linejoin": true,
	"stroke-miterlimit": true, "stroke-opacity": true, "stroke-width": true, "text-anchor": true, "text-decoration": true,
	"transform": true, "vector-effect": true, "visibility": true, "flood-color": true, "flood-opacity": true,
}
var inheritedProperties = map[string]bool{
	"alignment-baseline": true, "clip-rule": true, "color": true, "color-interpolation": true, "dominant-baseline": true,
	"fill": true, "fill-opacity": true, "fill-rule": true, "font-family": true, "font-size": true, "font-style": true,
	"font-weight": true, "marker": true, "marker-end": true, "marker-mid": true, "marker-start": true, "paint-order": true,
	"shape-rendering": true, "stroke": true, "stroke-dasharray": true, "stroke-dashoffset": true, "stroke-linecap": true,
	"stroke-linejoin": true, "stroke-miterlimit": true, "stroke-opacity": true, "stroke-width": true, "text-anchor": true,
	"stop-color": true, "stop-opacity": true, "visibility": true,
}

func defaultStyle() computedStyle {
	return computedStyle{
		"color": "black", "display": "inline", "visibility": "visible", "fill": "black", "fill-opacity": "1", "fill-rule": "nonzero",
		"stroke": "none", "stroke-opacity": "1", "stroke-width": "1", "stroke-linecap": "butt", "stroke-linejoin": "miter",
		"stroke-miterlimit": "4", "opacity": "1", "font-family": "sans-serif", "font-size": "16", "font-weight": "normal",
		"font-style": "normal", "text-anchor": "start", "stop-color": "black", "stop-opacity": "1", "overflow": "hidden",
	}
}

func (d *Document) styleFor(n *xmlNode, parent computedStyle) computedStyle {
	s := defaultStyle()
	for k := range inheritedProperties {
		if v, ok := parent[k]; ok {
			s[k] = v
		}
	}
	type winner struct {
		spec, order int
		important   bool
		value       string
	}
	w := map[string]winner{}
	apply := func(name, value string, important bool, spec, order int) {
		name = strings.ToLower(strings.TrimSpace(name))
		value = strings.TrimSpace(value)
		if value == "" {
			return
		}
		old, ok := w[name]
		if ok && (old.important && !important || old.important == important && (old.spec > spec || old.spec == spec && old.order > order)) {
			return
		}
		w[name] = winner{spec, order, important, value}
	}
	for _, a := range n.start.Attr {
		name := strings.ToLower(localName(a.Name.Local))
		if presentationProperties[name] {
			apply(name, a.Value, false, 0, -1)
		}
	}
	for _, rule := range d.rules {
		if matchesSelector(n, rule.selector) {
			for _, x := range rule.decls {
				apply(x.name, x.value, x.important, rule.specificity, rule.order)
			}
		}
	}
	if inline, ok := attrValue(n.start.Attr, "style"); ok {
		for _, x := range parseDeclarations(inline) {
			apply(x.name, x.value, x.important, 1000, 1<<30)
		}
	}
	for k, x := range w {
		v := x.value
		if strings.EqualFold(v, "inherit") {
			if p, ok := parent[k]; ok {
				s[k] = p
			}
			continue
		}
		if strings.EqualFold(v, "initial") {
			s[k] = defaultStyle()[k]
			continue
		}
		if strings.EqualFold(v, "unset") {
			if inheritedProperties[k] {
				s[k] = parent[k]
			} else {
				s[k] = defaultStyle()[k]
			}
			continue
		}
		s[k] = v
	}
	return s
}

func parseStylesheet(css string) []cssRule {
	for {
		a := strings.Index(css, "/*")
		if a < 0 {
			break
		}
		b := strings.Index(css[a+2:], "*/")
		if b < 0 {
			css = css[:a]
			break
		}
		css = css[:a] + css[a+2+b+2:]
	}
	var rules []cssRule
	order := 0
	for len(css) > 0 {
		i := strings.IndexByte(css, '{')
		if i < 0 {
			break
		}
		selectors := strings.TrimSpace(css[:i])
		css = css[i+1:]
		depth := 1
		j := 0
		for j < len(css) && depth > 0 {
			if css[j] == '{' {
				depth++
			}
			if css[j] == '}' {
				depth--
			}
			j++
		}
		if depth != 0 {
			break
		}
		body := css[:j-1]
		css = css[j:]
		if strings.HasPrefix(strings.TrimSpace(selectors), "@") {
			continue
		}
		decls := parseDeclarations(body)
		for _, sel := range splitOutside(selectors, ',') {
			sel = strings.TrimSpace(sel)
			if sel != "" {
				rules = append(rules, cssRule{sel, decls, selectorSpecificity(sel), order})
				order++
			}
		}
	}
	return rules
}

func parseDeclarations(s string) []cssDeclaration {
	var out []cssDeclaration
	for _, part := range splitOutside(s, ';') {
		i := strings.IndexByte(part, ':')
		if i < 0 {
			continue
		}
		name := strings.TrimSpace(part[:i])
		value := strings.TrimSpace(part[i+1:])
		important := false
		lower := strings.ToLower(value)
		if j := strings.LastIndex(lower, "!important"); j >= 0 && strings.TrimSpace(lower[j:]) == "!important" {
			value = strings.TrimSpace(value[:j])
			important = true
		}
		if name != "" && value != "" {
			out = append(out, cssDeclaration{name, value, important})
		}
	}
	return out
}

func splitOutside(s string, sep byte) []string {
	var out []string
	start, depth := 0, 0
	quote := byte(0)
	for i := 0; i < len(s); i++ {
		c := s[i]
		if quote != 0 {
			if c == quote {
				quote = 0
			}
			continue
		}
		if c == '\'' || c == '"' {
			quote = c
			continue
		}
		if c == '(' || c == '[' {
			depth++
		}
		if c == ')' || c == ']' {
			depth--
		}
		if c == sep && depth == 0 {
			out = append(out, s[start:i])
			start = i + 1
		}
	}
	return append(out, s[start:])
}

type selectorPart struct {
	compound   string
	combinator byte
}

func selectorParts(sel string) []selectorPart {
	var out []selectorPart
	for i := 0; i < len(sel); {
		for i < len(sel) && unicode.IsSpace(rune(sel[i])) {
			i++
		}
		if i >= len(sel) {
			break
		}
		start, depth, quote := i, 0, byte(0)
		for i < len(sel) {
			c := sel[i]
			if quote != 0 {
				if c == quote {
					quote = 0
				}
				i++
				continue
			}
			if c == '\'' || c == '"' {
				quote = c
				i++
				continue
			}
			if c == '[' || c == '(' {
				depth++
			}
			if c == ']' || c == ')' {
				depth--
			}
			if depth == 0 && (c == '>' || unicode.IsSpace(rune(c))) {
				break
			}
			i++
		}
		compound := strings.TrimSpace(sel[start:i])
		hadSpace := false
		for i < len(sel) && unicode.IsSpace(rune(sel[i])) {
			hadSpace = true
			i++
		}
		comb := byte(0)
		if i < len(sel) && sel[i] == '>' {
			comb = '>'
			i++
			for i < len(sel) && unicode.IsSpace(rune(sel[i])) {
				i++
			}
		} else if hadSpace && i < len(sel) {
			comb = ' '
		}
		if compound != "" {
			out = append(out, selectorPart{compound, comb})
		}
	}
	return out
}

func matchesSelector(n *xmlNode, sel string) bool {
	parts := selectorParts(sel)
	if len(parts) == 0 {
		return false
	}
	i := len(parts) - 1
	cur := n
	if !matchesCompound(cur, parts[i].compound) {
		return false
	}
	for i > 0 {
		comb := parts[i-1].combinator
		i--
		if comb == '>' {
			cur = cur.parent
			if cur == nil || !matchesCompound(cur, parts[i].compound) {
				return false
			}
		} else {
			found := false
			for cur = cur.parent; cur != nil; cur = cur.parent {
				if matchesCompound(cur, parts[i].compound) {
					found = true
					break
				}
			}
			if !found {
				return false
			}
		}
	}
	return true
}

func matchesCompound(n *xmlNode, s string) bool {
	if n == nil {
		return false
	}
	name := localName(n.start.Name.Local)
	i := 0
	if i < len(s) && (unicode.IsLetter(rune(s[i])) || s[i] == '*') {
		j := i + 1
		for j < len(s) && (unicode.IsLetter(rune(s[j])) || unicode.IsDigit(rune(s[j])) || s[j] == '-' || s[j] == '_') {
			j++
		}
		if s[i:j] != "*" && !strings.EqualFold(s[i:j], name) {
			return false
		}
		i = j
	}
	attrs := attrMap(n)
	classes := " " + strings.Join(strings.Fields(attrs["class"]), " ") + " "
	for i < len(s) {
		switch s[i] {
		case '#', '.':
			kind := s[i]
			i++
			j := i
			for j < len(s) && (unicode.IsLetter(rune(s[j])) || unicode.IsDigit(rune(s[j])) || s[j] == '-' || s[j] == '_') {
				j++
			}
			v := s[i:j]
			if kind == '#' && attrs["id"] != v {
				return false
			}
			if kind == '.' && !strings.Contains(classes, " "+v+" ") {
				return false
			}
			i = j
		case '[':
			j := strings.IndexByte(s[i:], ']')
			if j < 0 {
				return false
			}
			q := strings.TrimSpace(s[i+1 : i+j])
			i += j + 1
			op := ""
			k := -1
			for _, x := range []string{"~=", "|=", "^=", "$=", "*=", "="} {
				if p := strings.Index(q, x); p >= 0 {
					k = p
					op = x
					break
				}
			}
			if k < 0 {
				if _, ok := attrs[q]; !ok {
					return false
				}
				continue
			}
			key := strings.TrimSpace(q[:k])
			want := strings.Trim(strings.TrimSpace(q[k+len(op):]), "\"'")
			got, ok := attrs[key]
			if !ok {
				return false
			}
			switch op {
			case "=":
				if got != want {
					return false
				}
			case "~=":
				if !strings.Contains(" "+strings.Join(strings.Fields(got), " ")+" ", " "+want+" ") {
					return false
				}
			case "|=":
				if got != want && !strings.HasPrefix(got, want+"-") {
					return false
				}
			case "^=":
				if !strings.HasPrefix(got, want) {
					return false
				}
			case "$=":
				if !strings.HasSuffix(got, want) {
					return false
				}
			case "*=":
				if !strings.Contains(got, want) {
					return false
				}
			}
		case ':':
			i++
			j := i
			for j < len(s) && (unicode.IsLetter(rune(s[j])) || s[j] == '-') {
				j++
			}
			pseudo := s[i:j]
			if pseudo == "root" && n.parent != nil {
				return false
			}
			if pseudo == "first-child" && !firstElementChild(n) {
				return false
			}
			if pseudo != "root" && pseudo != "first-child" {
				return false
			}
			i = j
		default:
			return false
		}
	}
	return true
}

func firstElementChild(n *xmlNode) bool {
	if n.parent == nil {
		return false
	}
	for _, c := range n.parent.children {
		if c.elem != nil {
			return c.elem == n
		}
	}
	return false
}

func selectorSpecificity(s string) int {
	id, class, elem := 0, 0, 0
	for i, c := range s {
		switch c {
		case '#':
			id++
		case '.', '[', ':':
			class++
		default:
			if (i == 0 || unicode.IsSpace(rune(s[i-1])) || s[i-1] == '>') && unicode.IsLetter(c) {
				elem++
			}
		}
	}
	return id*100 + class*10 + elem
}

func sortedKeys[V any](m map[string]V) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
