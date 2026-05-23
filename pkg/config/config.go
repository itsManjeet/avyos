// Package config provides a parser for a simple configuration format with
// support for nested blocks, lists, strings, numbers, and booleans.
package config

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"unicode"
)

// Config holds the parsed configuration data.
type Config struct {
	data map[string]any
}

// Get retrieves a value by dot-separated path.
// Returns nil if the path does not exist and no default is provided.
// Returns the first default value if the path does not exist.
func (c *Config) Get(path string, defaults ...any) any {
	parts := strings.Split(path, ".")
	var current any = c.data
	for _, part := range parts {
		m, ok := current.(map[string]any)
		if !ok {
			if len(defaults) > 0 {
				return defaults[0]
			}
			return nil
		}
		val, exists := m[part]
		if !exists {
			if len(defaults) > 0 {
				return defaults[0]
			}
			return nil
		}
		current = val
	}
	return current
}

// Data returns the raw underlying map of the config.
func (c *Config) Data() map[string]any {
	return c.data
}

// Parse parses a config string and returns a Config object.
// Relative paths in .import directives are resolved from the current working directory.
func Parse(input string) (*Config, error) {
	p := newParser(input, "")
	data, err := p.parseBlock(false)
	if err != nil {
		return nil, err
	}
	return &Config{data: data}, nil
}

// ParseFile reads a file and parses it as a Config object.
// Relative paths in .import directives are resolved relative to the file's directory.
func ParseFile(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	p := newParser(string(data), filepath.Dir(path))
	result, err := p.parseBlock(false)
	if err != nil {
		return nil, err
	}
	return &Config{data: result}, nil
}

// Unmarshal parses config bytes and populates v (must be a pointer to a struct or map).
func Unmarshal(data []byte, v any) error {
	cfg, err := Parse(string(data))
	if err != nil {
		return err
	}
	return populateValue(cfg.data, reflect.ValueOf(v))
}

// Marshal converts v (a struct or map) to config format bytes.
func Marshal(v any) ([]byte, error) {
	var buf bytes.Buffer
	if err := marshalValue(&buf, reflect.ValueOf(v), 0); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// --- Parser ---

type parser struct {
	input   []rune
	pos     int
	baseDir string
}

func newParser(input, baseDir string) *parser {
	return &parser{input: []rune(input), baseDir: baseDir}
}

func (p *parser) peek() (rune, bool) {
	if p.pos >= len(p.input) {
		return 0, false
	}
	return p.input[p.pos], true
}

func (p *parser) skipHorizontalWhitespace() {
	for p.pos < len(p.input) && (p.input[p.pos] == ' ' || p.input[p.pos] == '\t') {
		p.pos++
	}
}

func (p *parser) skipWhitespaceAndNewlines() {
	for p.pos < len(p.input) {
		ch := p.input[p.pos]
		if ch == ' ' || ch == '\t' || ch == '\n' || ch == '\r' {
			p.pos++
		} else {
			break
		}
	}
}

// skipLineRemainder skips optional horizontal whitespace and an inline comment,
// leaving the position at the newline (or EOF).
func (p *parser) skipLineRemainder() {
	p.skipHorizontalWhitespace()
	if p.pos < len(p.input) && p.input[p.pos] == '#' {
		for p.pos < len(p.input) && p.input[p.pos] != '\n' {
			p.pos++
		}
	}
}

func (p *parser) parseKey() (string, error) {
	start := p.pos
	for p.pos < len(p.input) {
		ch := p.input[p.pos]
		if unicode.IsLetter(ch) || unicode.IsDigit(ch) || ch == '_' {
			p.pos++
		} else {
			break
		}
	}
	if p.pos == start {
		return "", fmt.Errorf("expected identifier at position %d, got %q", p.pos, string(p.input[p.pos:min(p.pos+10, len(p.input))]))
	}
	return string(p.input[start:p.pos]), nil
}

// parseBlock parses a sequence of key=value and key{block} entries.
// If nested is true, it stops on '}' without consuming it.
func (p *parser) parseBlock(nested bool) (map[string]any, error) {
	result := make(map[string]any)

	for {
		p.skipWhitespaceAndNewlines()

		ch, ok := p.peek()
		if !ok {
			break
		}

		if ch == '}' {
			if nested {
				break
			}
			return nil, fmt.Errorf("unexpected '}' at position %d", p.pos)
		}

		if ch == '#' {
			for p.pos < len(p.input) && p.input[p.pos] != '\n' {
				p.pos++
			}
			continue
		}

		if ch == '.' {
			p.pos++ // consume '.'
			directive, err := p.parseKey()
			if err != nil {
				return nil, fmt.Errorf("invalid directive at position %d: %w", p.pos, err)
			}
			switch directive {
			case "import":
				p.skipHorizontalWhitespace()
				importPath, err := p.parseQuotedString()
				if err != nil {
					return nil, fmt.Errorf(".import: %w", err)
				}
				if p.baseDir != "" && !filepath.IsAbs(importPath) {
					importPath = filepath.Join(p.baseDir, importPath)
				}
				raw, err := os.ReadFile(importPath)
				if err != nil {
					return nil, fmt.Errorf(".import %q: %w", importPath, err)
				}
				sub := newParser(string(raw), filepath.Dir(importPath))
				imported, err := sub.parseBlock(false)
				if err != nil {
					return nil, fmt.Errorf(".import %q: %w", importPath, err)
				}
				for k, v := range imported {
					result[k] = v
				}
				p.skipLineRemainder()
			default:
				return nil, fmt.Errorf("unknown directive .%s at position %d", directive, p.pos)
			}
			continue
		}

		key, err := p.parseKey()
		if err != nil {
			return nil, err
		}

		p.skipHorizontalWhitespace()

		ch, ok = p.peek()
		if !ok {
			return nil, fmt.Errorf("unexpected end of input after key %q", key)
		}

		switch ch {
		case '=':
			p.pos++ // consume '='
			p.skipHorizontalWhitespace()
			val, err := p.parseValue()
			if err != nil {
				return nil, fmt.Errorf("error parsing value for key %q: %w", key, err)
			}
			p.skipLineRemainder()
			result[key] = val

		case '{':
			p.pos++ // consume '{'
			p.skipLineRemainder()
			sub, err := p.parseBlock(true)
			if err != nil {
				return nil, fmt.Errorf("error parsing block for key %q: %w", key, err)
			}
			p.skipHorizontalWhitespace()
			if p.pos >= len(p.input) || p.input[p.pos] != '}' {
				return nil, fmt.Errorf("expected '}' to close block for key %q", key)
			}
			p.pos++ // consume '}'
			p.skipLineRemainder()
			result[key] = sub

		default:
			return nil, fmt.Errorf("expected '=' or '{' after key %q, got %q", key, string(ch))
		}
	}

	return result, nil
}

func (p *parser) parseValue() (any, error) {
	ch, ok := p.peek()
	if !ok {
		return "", nil
	}
	switch ch {
	case '"':
		return p.parseQuotedString()
	case '[':
		return p.parseList()
	default:
		return p.parseSimpleValue()
	}
}

func (p *parser) parseQuotedString() (string, error) {
	p.pos++ // consume opening '"'
	var sb strings.Builder

	for p.pos < len(p.input) {
		ch := p.input[p.pos]
		if ch == '"' {
			p.pos++ // consume closing '"'
			return sb.String(), nil
		}
		if ch == '\\' && p.pos+1 < len(p.input) {
			p.pos++
			switch p.input[p.pos] {
			case 'n':
				sb.WriteRune('\n')
			case 't':
				sb.WriteRune('\t')
			case '"':
				sb.WriteRune('"')
			case '\\':
				sb.WriteRune('\\')
			default:
				sb.WriteRune('\\')
				sb.WriteRune(p.input[p.pos])
			}
			p.pos++
			continue
		}
		if ch == '\n' {
			sb.WriteRune('\n')
			p.pos++
			// Strip leading whitespace from continuation lines
			for p.pos < len(p.input) && (p.input[p.pos] == ' ' || p.input[p.pos] == '\t') {
				p.pos++
			}
			continue
		}
		sb.WriteRune(ch)
		p.pos++
	}

	return "", fmt.Errorf("unterminated string literal")
}

func (p *parser) parseList() ([]any, error) {
	p.pos++ // consume '['
	var list []any

	for {
		// Skip whitespace, newlines, and comments between list items
		for {
			p.skipWhitespaceAndNewlines()
			ch, ok := p.peek()
			if !ok {
				return nil, fmt.Errorf("unterminated list")
			}
			if ch != '#' {
				break
			}
			for p.pos < len(p.input) && p.input[p.pos] != '\n' {
				p.pos++
			}
		}

		ch, ok := p.peek()
		if !ok {
			return nil, fmt.Errorf("unterminated list")
		}
		if ch == ']' {
			p.pos++ // consume ']'
			break
		}

		val, err := p.parseValue()
		if err != nil {
			return nil, err
		}
		list = append(list, val)

		p.skipHorizontalWhitespace()
		ch, ok = p.peek()
		if !ok {
			return nil, fmt.Errorf("unterminated list")
		}
		if ch == ',' {
			p.pos++ // consume ','
		} else if ch != ']' && ch != '\n' && ch != '\r' && ch != '#' {
			return nil, fmt.Errorf("expected ',' or ']' in list, got %q", string(ch))
		}
	}

	return list, nil
}

func (p *parser) parseSimpleValue() (any, error) {
	start := p.pos
	for p.pos < len(p.input) {
		ch := p.input[p.pos]
		if ch == '\n' || ch == '\r' || ch == '#' || ch == ',' || ch == ']' || ch == '}' {
			break
		}
		p.pos++
	}
	raw := strings.TrimRight(string(p.input[start:p.pos]), " \t")
	return inferType(raw), nil
}

func inferType(s string) any {
	switch strings.ToLower(s) {
	case "true":
		return true
	case "false":
		return false
	}
	cleaned := strings.ReplaceAll(s, "_", "")
	if i, err := strconv.ParseInt(cleaned, 10, 64); err == nil {
		return i
	}
	if f, err := strconv.ParseFloat(cleaned, 64); err == nil {
		return f
	}
	return s
}

// --- Marshal ---

func marshalValue(buf *bytes.Buffer, v reflect.Value, depth int) error {
	for v.Kind() == reflect.Pointer {
		if v.IsNil() {
			return nil
		}
		v = v.Elem()
	}
	switch v.Kind() {
	case reflect.Struct:
		t := v.Type()
		for i := 0; i < t.NumField(); i++ {
			field := t.Field(i)
			if !field.IsExported() {
				continue
			}
			key := fieldKey(field)
			if err := marshalField(buf, key, v.Field(i), depth); err != nil {
				return err
			}
		}
	case reflect.Map:
		for _, k := range v.MapKeys() {
			key := fmt.Sprintf("%v", k.Interface())
			if err := marshalField(buf, key, v.MapIndex(k), depth); err != nil {
				return err
			}
		}
	default:
		return fmt.Errorf("marshal requires a struct or map at top level, got %s", v.Kind())
	}
	return nil
}

func marshalField(buf *bytes.Buffer, key string, v reflect.Value, depth int) error {
	indent := strings.Repeat("    ", depth)

	for v.Kind() == reflect.Pointer {
		if v.IsNil() {
			return nil
		}
		v = v.Elem()
	}

	switch v.Kind() {
	case reflect.Struct:
		fmt.Fprintf(buf, "%s%s {\n", indent, key)
		if err := marshalValue(buf, v, depth+1); err != nil {
			return err
		}
		fmt.Fprintf(buf, "%s}\n", indent)
	case reflect.Map:
		fmt.Fprintf(buf, "%s%s {\n", indent, key)
		if err := marshalValue(buf, v, depth+1); err != nil {
			return err
		}
		fmt.Fprintf(buf, "%s}\n", indent)
	case reflect.Slice:
		fmt.Fprintf(buf, "%s%s = [\n", indent, key)
		for i := 0; i < v.Len(); i++ {
			fmt.Fprintf(buf, "%s    %s,\n", indent, marshalScalar(v.Index(i)))
		}
		fmt.Fprintf(buf, "%s]\n", indent)
	default:
		fmt.Fprintf(buf, "%s%s = %s\n", indent, key, marshalScalar(v))
	}
	return nil
}

func marshalScalar(v reflect.Value) string {
	switch v.Kind() {
	case reflect.String:
		s := v.String()
		escaped := strings.ReplaceAll(s, "\\", "\\\\")
		escaped = strings.ReplaceAll(escaped, "\"", "\\\"")
		return "\"" + escaped + "\""
	case reflect.Bool:
		if v.Bool() {
			return "true"
		}
		return "false"
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return strconv.FormatInt(v.Int(), 10)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return strconv.FormatUint(v.Uint(), 10)
	case reflect.Float32, reflect.Float64:
		return strconv.FormatFloat(v.Float(), 'f', -1, 64)
	default:
		return fmt.Sprintf("%v", v.Interface())
	}
}

// --- Unmarshal ---

func populateValue(data map[string]any, v reflect.Value) error {
	for v.Kind() == reflect.Pointer {
		if v.IsNil() {
			v.Set(reflect.New(v.Type().Elem()))
		}
		v = v.Elem()
	}
	switch v.Kind() {
	case reflect.Struct:
		t := v.Type()
		for i := 0; i < t.NumField(); i++ {
			field := t.Field(i)
			if !field.IsExported() {
				continue
			}
			key := fieldKey(field)
			val, ok := data[key]
			if !ok {
				continue
			}
			if err := setField(v.Field(i), val); err != nil {
				return fmt.Errorf("field %s: %w", field.Name, err)
			}
		}
	case reflect.Map:
		if v.IsNil() {
			v.Set(reflect.MakeMap(v.Type()))
		}
		for k, val := range data {
			vv := reflect.New(v.Type().Elem()).Elem()
			if err := setField(vv, val); err != nil {
				return fmt.Errorf("key %s: %w", k, err)
			}
			v.SetMapIndex(reflect.ValueOf(k), vv)
		}
	default:
		return fmt.Errorf("unmarshal requires a pointer to a struct or map")
	}
	return nil
}

func setField(v reflect.Value, val any) error {
	if val == nil {
		return nil
	}

	for v.Kind() == reflect.Pointer {
		if v.IsNil() {
			v.Set(reflect.New(v.Type().Elem()))
		}
		v = v.Elem()
	}

	if nested, ok := val.(map[string]any); ok {
		return populateValue(nested, v)
	}

	if list, ok := val.([]any); ok {
		if v.Kind() != reflect.Slice {
			return fmt.Errorf("cannot assign list to %s", v.Kind())
		}
		slice := reflect.MakeSlice(v.Type(), len(list), len(list))
		for i, item := range list {
			if err := setField(slice.Index(i), item); err != nil {
				return fmt.Errorf("index %d: %w", i, err)
			}
		}
		v.Set(slice)
		return nil
	}

	rv := reflect.ValueOf(val)
	if rv.Type().AssignableTo(v.Type()) {
		v.Set(rv)
		return nil
	}
	if rv.Type().ConvertibleTo(v.Type()) {
		v.Set(rv.Convert(v.Type()))
		return nil
	}

	switch v.Kind() {
	case reflect.String:
		v.SetString(fmt.Sprintf("%v", val))
	case reflect.Bool:
		switch x := val.(type) {
		case bool:
			v.SetBool(x)
		case string:
			b, err := strconv.ParseBool(x)
			if err != nil {
				return fmt.Errorf("cannot convert %q to bool: %w", x, err)
			}
			v.SetBool(b)
		default:
			return fmt.Errorf("cannot assign %T to bool", val)
		}
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		switch x := val.(type) {
		case int64:
			v.SetInt(x)
		case float64:
			v.SetInt(int64(x))
		case string:
			n, err := strconv.ParseInt(strings.ReplaceAll(x, "_", ""), 10, 64)
			if err != nil {
				return fmt.Errorf("cannot convert %q to int: %w", x, err)
			}
			v.SetInt(n)
		default:
			return fmt.Errorf("cannot assign %T to int", val)
		}
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		switch x := val.(type) {
		case int64:
			v.SetUint(uint64(x))
		case float64:
			v.SetUint(uint64(x))
		case string:
			n, err := strconv.ParseUint(strings.ReplaceAll(x, "_", ""), 10, 64)
			if err != nil {
				return fmt.Errorf("cannot convert %q to uint: %w", x, err)
			}
			v.SetUint(n)
		default:
			return fmt.Errorf("cannot assign %T to uint", val)
		}
	case reflect.Float32, reflect.Float64:
		switch x := val.(type) {
		case float64:
			v.SetFloat(x)
		case int64:
			v.SetFloat(float64(x))
		case string:
			f, err := strconv.ParseFloat(strings.ReplaceAll(x, "_", ""), 64)
			if err != nil {
				return fmt.Errorf("cannot convert %q to float: %w", x, err)
			}
			v.SetFloat(f)
		default:
			return fmt.Errorf("cannot assign %T to float", val)
		}
	case reflect.Interface:
		v.Set(reflect.ValueOf(val))
	default:
		return fmt.Errorf("cannot assign %T to %s", val, v.Kind())
	}
	return nil
}

func fieldKey(f reflect.StructField) string {
	if tag := f.Tag.Get("config"); tag != "" {
		if name := strings.SplitN(tag, ",", 2)[0]; name != "" {
			return name
		}
	}
	return f.Name
}
