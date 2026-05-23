package settings

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"unicode"

	"avyos.dev/lib/ini"
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

type Config struct {
	data map[string]any
}

func (c *Config) Data() map[string]any {
	if c == nil {
		return nil
	}
	return c.data
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
		return filepath.Join(home, ".config", "avyos", "settings.ini")
	}
	defaultSystemSettingsPath = func() string { return "/etc/avyos/settings.ini" }
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

func (s Store) Load(scope Scope) (*Config, error) {
	data, err := s.loadMap(scope)
	if err != nil {
		return nil, err
	}
	return &Config{data: data}, nil
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
	return parseSettingValue(raw), nil
}

func FormatValue(value any) string {
	return formatSettingValue(value)
}

func (s Store) loadMap(scope Scope) (map[string]any, error) {
	path, err := s.Path(scope)
	if err != nil {
		return nil, err
	}
	cfg, err := ini.ParseFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]any{}, nil
		}
		return nil, err
	}
	return mapFromINI(cfg), nil
}

func (s Store) saveMap(scope Scope, data map[string]any) error {
	path, err := s.Path(scope)
	if err != nil {
		return err
	}
	payload, err := marshalINIMap(data)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, payload, 0o644)
}

func mapFromINI(cfg *ini.Config) map[string]any {
	data := map[string]any{}
	if cfg == nil {
		return data
	}
	for _, entry := range cfg.Entries {
		if entry.Type != ini.EntryKeyValue {
			continue
		}
		parts := strings.Split(strings.Trim(entry.Key, "."), ".")
		if entry.Section != "" {
			sectionParts := strings.Split(strings.Trim(entry.Section, "."), ".")
			parts = append(sectionParts, parts...)
		}
		valid := true
		for _, part := range parts {
			if !isIdentifier(part) {
				valid = false
				break
			}
		}
		if valid && len(parts) != 0 {
			setAtPath(data, parts, parseSettingValue(entry.Value))
		}
	}
	return data
}

func marshalINIMap(data map[string]any) ([]byte, error) {
	entries := make([]Entry, 0)
	flattenEntries(nil, data, &entries)
	sort.Slice(entries, func(i, j int) bool { return entries[i].Path < entries[j].Path })

	var b bytes.Buffer
	for _, entry := range entries {
		if _, err := fmt.Fprintf(&b, "%s = %s\n", entry.Path, formatSettingValue(entry.Value)); err != nil {
			return nil, err
		}
	}
	return b.Bytes(), nil
}

func parseSettingValue(raw string) any {
	s := strings.TrimSpace(raw)
	if s == "" {
		return ""
	}
	if unquoted, err := strconv.Unquote(s); err == nil {
		return unquoted
	}
	switch strings.ToLower(s) {
	case "true", "yes", "on":
		return true
	case "false", "no", "off":
		return false
	}
	if i, err := strconv.ParseInt(s, 10, 64); err == nil && !strings.ContainsAny(s, ".eE") {
		return i
	}
	if f, err := strconv.ParseFloat(s, 64); err == nil && strings.ContainsAny(s, ".eE") {
		return f
	}
	return s
}

func formatSettingValue(value any) string {
	v := unwrapValue(reflect.ValueOf(value))
	if !v.IsValid() {
		return ""
	}
	switch v.Kind() {
	case reflect.Bool:
		if v.Bool() {
			return "true"
		}
		return "false"
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return strconv.FormatInt(v.Int(), 10)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		return strconv.FormatUint(v.Uint(), 10)
	case reflect.Float32, reflect.Float64:
		return strconv.FormatFloat(v.Float(), 'f', -1, 64)
	case reflect.String:
		return v.String()
	default:
		return fmt.Sprintf("%v", value)
	}
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
		*out = append(*out, Entry{Path: strings.Join(path, "."), Value: cloneValue(value)})
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

func unwrapValue(v reflect.Value) reflect.Value {
	for v.IsValid() && (v.Kind() == reflect.Pointer || v.Kind() == reflect.Interface) {
		if v.IsNil() {
			return reflect.Value{}
		}
		v = v.Elem()
	}
	return v
}
