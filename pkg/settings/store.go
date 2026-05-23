package settings

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"unicode"

	"avyos.dev/pkg/config"
)

type Scope uint8

const (
	ScopeUser Scope = iota
	ScopeSystem
)

type Entry struct {
	Path  string
	Value any
}

type Store struct {
	UserPath   string
	SystemPath string
}

var (
	defaultUserSettingsPath = func() string {
		home := strings.TrimSpace(os.Getenv("HOME"))
		if home == "" {
			return ""
		}
		return filepath.Join(home, ".config", "avyos", "settings.conf")
	}
	defaultSystemSettingsPath = func() string { return "/config/avyos/settings.conf" }
)

func DefaultStore() Store {
	return Store{
		UserPath:   defaultUserSettingsPath(),
		SystemPath: defaultSystemSettingsPath(),
	}
}

func ParseScope(raw string) (Scope, error) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "", "user":
		return ScopeUser, nil
	case "system", "global":
		return ScopeSystem, nil
	default:
		return ScopeUser, fmt.Errorf("settings: unknown scope %q", raw)
	}
}

func (s Scope) String() string {
	switch s {
	case ScopeSystem:
		return "system"
	default:
		return "user"
	}
}

func (s Store) Path(scope Scope) (string, error) {
	var path string
	switch scope {
	case ScopeUser:
		path = strings.TrimSpace(s.UserPath)
	case ScopeSystem:
		path = strings.TrimSpace(s.SystemPath)
	default:
		return "", fmt.Errorf("settings: unsupported scope %d", scope)
	}
	if path == "" {
		return "", fmt.Errorf("settings: %s path is not configured", scope.String())
	}
	return path, nil
}

func (s Store) Load(scope Scope) (*config.Config, error) {
	data, err := s.loadMap(scope)
	if err != nil {
		return nil, err
	}
	payload, err := marshalConfigMap(data)
	if err != nil {
		return nil, err
	}
	return config.Parse(string(payload))
}

func (s Store) Get(scope Scope, path string) (any, bool, error) {
	parts, err := normalizeKeyPath(path, false)
	if err != nil {
		return nil, false, err
	}
	data, err := s.loadMap(scope)
	if err != nil {
		return nil, false, err
	}
	value, ok := getAtPath(data, parts)
	if !ok {
		return nil, false, nil
	}
	return cloneValue(value), true, nil
}

func (s Store) Set(scope Scope, path string, value any) error {
	parts, err := normalizeKeyPath(path, false)
	if err != nil {
		return err
	}
	data, err := s.loadMap(scope)
	if err != nil {
		return err
	}
	setAtPath(data, parts, cloneValue(value))
	return s.saveMap(scope, data)
}

func (s Store) Delete(scope Scope, path string) error {
	parts, err := normalizeKeyPath(path, false)
	if err != nil {
		return err
	}
	data, err := s.loadMap(scope)
	if err != nil {
		return err
	}
	deleteAtPath(data, parts)
	return s.saveMap(scope, data)
}

func (s Store) List(scope Scope, prefix string) ([]Entry, error) {
	parts, err := normalizeKeyPath(prefix, true)
	if err != nil {
		return nil, err
	}
	data, err := s.loadMap(scope)
	if err != nil {
		return nil, err
	}

	var root any = data
	if len(parts) != 0 {
		val, ok := getAtPath(data, parts)
		if !ok {
			return nil, nil
		}
		root = val
	}

	entries := make([]Entry, 0)
	switch v := root.(type) {
	case map[string]any:
		flattenEntries(parts, v, &entries)
	default:
		entries = append(entries, Entry{Path: strings.Join(parts, "."), Value: cloneValue(v)})
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Path < entries[j].Path })
	return entries, nil
}

func ParseValue(raw string) (any, error) {
	cfg, err := config.Parse("Value = " + raw)
	if err != nil {
		return nil, err
	}
	return cfg.Get("Value"), nil
}

func FormatValue(value any) string {
	data, err := marshalDisplayValue(cloneValue(value))
	if err != nil {
		return fmt.Sprintf("%v", value)
	}
	return strings.TrimSpace(data)
}

func (s Store) loadMap(scope Scope) (map[string]any, error) {
	path, err := s.Path(scope)
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]any{}, nil
		}
		return nil, err
	}
	if len(data) == 0 {
		return map[string]any{}, nil
	}
	cfg, err := config.Parse(string(data))
	if err != nil {
		return nil, err
	}
	return cloneMap(cfg.Data()), nil
}

func (s Store) saveMap(scope Scope, data map[string]any) error {
	path, err := s.Path(scope)
	if err != nil {
		return err
	}
	payload, err := marshalConfigMap(data)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, payload, 0o644)
}

func normalizeKeyPath(path string, allowEmpty bool) ([]string, error) {
	path = strings.TrimSpace(path)
	path = strings.Trim(path, ".")
	if path == "" {
		if allowEmpty {
			return nil, nil
		}
		return nil, errors.New("settings: path is required")
	}
	parts := strings.Split(path, ".")
	for i, part := range parts {
		if !isIdentifier(part) {
			return nil, fmt.Errorf("settings: invalid path segment %q at %d", part, i)
		}
	}
	return parts, nil
}

func isIdentifier(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' {
			continue
		}
		return false
	}
	return true
}

func getAtPath(root map[string]any, parts []string) (any, bool) {
	var current any = root
	for _, part := range parts {
		m, ok := current.(map[string]any)
		if !ok {
			return nil, false
		}
		next, ok := m[part]
		if !ok {
			return nil, false
		}
		current = next
	}
	return current, true
}

func setAtPath(root map[string]any, parts []string, value any) {
	current := root
	for _, part := range parts[:len(parts)-1] {
		next, ok := current[part].(map[string]any)
		if !ok {
			next = map[string]any{}
			current[part] = next
		}
		current = next
	}
	current[parts[len(parts)-1]] = value
}

func deleteAtPath(root map[string]any, parts []string) bool {
	if len(parts) == 0 {
		return len(root) == 0
	}
	if len(parts) == 1 {
		delete(root, parts[0])
		return len(root) == 0
	}
	next, ok := root[parts[0]].(map[string]any)
	if !ok {
		return len(root) == 0
	}
	if deleteAtPath(next, parts[1:]) {
		delete(root, parts[0])
	}
	return len(root) == 0
}

func flattenEntries(prefix []string, root map[string]any, out *[]Entry) {
	keys := make([]string, 0, len(root))
	for key := range root {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		value := root[key]
		path := append(append([]string(nil), prefix...), key)
		if nested, ok := value.(map[string]any); ok {
			flattenEntries(path, nested, out)
			continue
		}
		*out = append(*out, Entry{
			Path:  strings.Join(path, "."),
			Value: cloneValue(value),
		})
	}
}

func cloneMap(src map[string]any) map[string]any {
	dst := make(map[string]any, len(src))
	for key, value := range src {
		dst[key] = cloneValue(value)
	}
	return dst
}

func cloneValue(value any) any {
	switch v := value.(type) {
	case map[string]any:
		return cloneMap(v)
	case []any:
		out := make([]any, len(v))
		for i := range v {
			out[i] = cloneValue(v[i])
		}
		return out
	default:
		return v
	}
}

func marshalConfigMap(data map[string]any) ([]byte, error) {
	var b strings.Builder
	if err := writeConfigMap(&b, data, 0); err != nil {
		return nil, err
	}
	return []byte(b.String()), nil
}

func marshalDisplayValue(value any) (string, error) {
	v := unwrapValue(reflect.ValueOf(value))
	if !v.IsValid() {
		return "", errors.New("settings: nil values are not supported")
	}

	if v.Kind() == reflect.Map {
		var b strings.Builder
		b.WriteString("{\n")
		if err := writeConfigReflectMap(&b, v, 1); err != nil {
			return "", err
		}
		b.WriteString("}")
		return b.String(), nil
	}

	return marshalInlineValue(v)
}

func writeConfigMap(b *strings.Builder, data map[string]any, depth int) error {
	keys := make([]string, 0, len(data))
	for key := range data {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	for _, key := range keys {
		if !isIdentifier(key) {
			return fmt.Errorf("settings: invalid key %q", key)
		}
		if err := writeConfigField(b, key, reflect.ValueOf(data[key]), depth); err != nil {
			return err
		}
	}
	return nil
}

func writeConfigReflectMap(b *strings.Builder, v reflect.Value, depth int) error {
	if v.Kind() != reflect.Map {
		return fmt.Errorf("settings: expected map, got %s", v.Kind())
	}
	if v.Type().Key().Kind() != reflect.String {
		return fmt.Errorf("settings: unsupported map key type %s", v.Type().Key())
	}

	keys := make([]string, 0, v.Len())
	for _, key := range v.MapKeys() {
		keys = append(keys, key.String())
	}
	sort.Strings(keys)

	for _, key := range keys {
		if !isIdentifier(key) {
			return fmt.Errorf("settings: invalid key %q", key)
		}
		if err := writeConfigField(b, key, v.MapIndex(reflect.ValueOf(key)), depth); err != nil {
			return err
		}
	}
	return nil
}

func writeConfigField(b *strings.Builder, key string, v reflect.Value, depth int) error {
	v = unwrapValue(v)
	if !v.IsValid() {
		return nil
	}

	indent := strings.Repeat("    ", depth)
	switch v.Kind() {
	case reflect.Map:
		b.WriteString(indent)
		b.WriteString(key)
		b.WriteString(" {\n")
		if err := writeConfigReflectMap(b, v, depth+1); err != nil {
			return err
		}
		b.WriteString(indent)
		b.WriteString("}\n")
	case reflect.Slice, reflect.Array:
		b.WriteString(indent)
		b.WriteString(key)
		b.WriteString(" = [\n")
		for i := 0; i < v.Len(); i++ {
			literal, err := marshalInlineValue(v.Index(i))
			if err != nil {
				return err
			}
			b.WriteString(indent)
			b.WriteString("    ")
			b.WriteString(literal)
			b.WriteString(",\n")
		}
		b.WriteString(indent)
		b.WriteString("]\n")
	default:
		literal, err := marshalInlineValue(v)
		if err != nil {
			return err
		}
		b.WriteString(indent)
		b.WriteString(key)
		b.WriteString(" = ")
		b.WriteString(literal)
		b.WriteString("\n")
	}
	return nil
}

func marshalInlineValue(v reflect.Value) (string, error) {
	v = unwrapValue(v)
	if !v.IsValid() {
		return "", errors.New("settings: nil values are not supported")
	}

	switch v.Kind() {
	case reflect.String:
		return quoteString(v.String()), nil
	case reflect.Bool:
		if v.Bool() {
			return "true", nil
		}
		return "false", nil
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return strconv.FormatInt(v.Int(), 10), nil
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		return strconv.FormatUint(v.Uint(), 10), nil
	case reflect.Float32, reflect.Float64:
		return strconv.FormatFloat(v.Float(), 'f', -1, 64), nil
	case reflect.Slice, reflect.Array:
		values := make([]string, 0, v.Len())
		for i := 0; i < v.Len(); i++ {
			literal, err := marshalInlineValue(v.Index(i))
			if err != nil {
				return "", err
			}
			values = append(values, literal)
		}
		return "[" + strings.Join(values, ", ") + "]", nil
	case reflect.Map:
		return "", errors.New("settings: maps are only supported as named blocks")
	default:
		return "", fmt.Errorf("settings: unsupported value kind %s", v.Kind())
	}
}

func unwrapValue(v reflect.Value) reflect.Value {
	for v.IsValid() && (v.Kind() == reflect.Interface || v.Kind() == reflect.Pointer) {
		if v.IsNil() {
			return reflect.Value{}
		}
		v = v.Elem()
	}
	return v
}

func quoteString(value string) string {
	replacer := strings.NewReplacer(
		"\\", "\\\\",
		"\"", "\\\"",
		"\n", "\\n",
		"\t", "\\t",
	)
	return `"` + replacer.Replace(value) + `"`
}
