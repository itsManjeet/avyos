package config_test

import (
	"testing"

	"avyos.dev/pkg/config"
)

const sampleConfig = `
Key = Value
MultiWordKey = Value can be multiline
MultiLineKey = "If Value is in
    inverted commas than it can span
    multiple lines, and starting spaces are
    ignored"

# These are comments and can be ignored
NumberKey = 10
NumberKeyWithDecimals = 10.5
NumberKeyWithDashes = 10_00_000 # Dashes are ignored, only with visual aid

BooleanKey = true

ListKey = [
    "List Support multiple kind comma separated values",
    10,
    true,
]

NestedKey {
    NestedValueKey {
        NestedNestedKey = "Nested nested key value"
    }
}
`

func TestParse(t *testing.T) {
	cfg, err := config.Parse(sampleConfig)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	tests := []struct {
		path     string
		expected any
	}{
		{"Key", "Value"},
		{"MultiWordKey", "Value can be multiline"},
		{"NumberKey", int64(10)},
		{"NumberKeyWithDecimals", float64(10.5)},
		{"NumberKeyWithDashes", int64(1000000)},
		{"BooleanKey", true},
		{"NestedKey.NestedValueKey.NestedNestedKey", "Nested nested key value"},
	}

	for _, tt := range tests {
		got := cfg.Get(tt.path)
		if got != tt.expected {
			t.Errorf("Get(%q) = %v (%T), want %v (%T)", tt.path, got, got, tt.expected, tt.expected)
		}
	}

	// multiline string
	ml := cfg.Get("MultiLineKey")
	if ml == nil {
		t.Error("MultiLineKey is nil")
	}

	// list
	list, ok := cfg.Get("ListKey").([]any)
	if !ok {
		t.Fatalf("ListKey is not a slice, got %T", cfg.Get("ListKey"))
	}
	if len(list) != 3 {
		t.Errorf("ListKey len = %d, want 3", len(list))
	}
	if list[1] != int64(10) {
		t.Errorf("ListKey[1] = %v, want 10", list[1])
	}
	if list[2] != true {
		t.Errorf("ListKey[2] = %v, want true", list[2])
	}
}

func TestGetDefault(t *testing.T) {
	cfg, _ := config.Parse("Key = Value")

	if got := cfg.Get("Missing"); got != nil {
		t.Errorf("expected nil, got %v", got)
	}
	if got := cfg.Get("Missing", "fallback"); got != "fallback" {
		t.Errorf("expected fallback, got %v", got)
	}
	if got := cfg.Get("Missing", 42); got != 42 {
		t.Errorf("expected 42, got %v", got)
	}
}

func TestMarshalUnmarshal(t *testing.T) {
	type Inner struct {
		Value string
	}
	type Cfg struct {
		Name   string
		Count  int
		Flag   bool
		Tags   []string
		Nested Inner
	}

	original := Cfg{
		Name:   "test",
		Count:  42,
		Flag:   true,
		Tags:   []string{"a", "b", "c"},
		Nested: Inner{Value: "deep"},
	}

	data, err := config.Marshal(original)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	var result Cfg
	if err := config.Unmarshal(data, &result); err != nil {
		t.Fatalf("Unmarshal failed: %v\ndata:\n%s", err, data)
	}

	if result.Name != original.Name {
		t.Errorf("Name: got %q, want %q", result.Name, original.Name)
	}
	if result.Count != original.Count {
		t.Errorf("Count: got %d, want %d", result.Count, original.Count)
	}
	if result.Flag != original.Flag {
		t.Errorf("Flag: got %v, want %v", result.Flag, original.Flag)
	}
	if len(result.Tags) != len(original.Tags) {
		t.Errorf("Tags len: got %d, want %d", len(result.Tags), len(original.Tags))
	}
	if result.Nested.Value != original.Nested.Value {
		t.Errorf("Nested.Value: got %q, want %q", result.Nested.Value, original.Nested.Value)
	}
}

func TestUnmarshalSample(t *testing.T) {
	type NestedNested struct {
		NestedNestedKey string
	}
	type NestedValue struct {
		NestedValueKey NestedNested
	}
	type Sample struct {
		Key                 string
		NumberKey           int
		NumberKeyWithDashes int64
		BooleanKey          bool
		NestedKey           NestedValue
	}

	var s Sample
	if err := config.Unmarshal([]byte(sampleConfig), &s); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}
	if s.Key != "Value" {
		t.Errorf("Key = %q, want %q", s.Key, "Value")
	}
	if s.NumberKey != 10 {
		t.Errorf("NumberKey = %d, want 10", s.NumberKey)
	}
	if s.NumberKeyWithDashes != 1000000 {
		t.Errorf("NumberKeyWithDashes = %d, want 1000000", s.NumberKeyWithDashes)
	}
	if !s.BooleanKey {
		t.Error("BooleanKey should be true")
	}
	if s.NestedKey.NestedValueKey.NestedNestedKey != "Nested nested key value" {
		t.Errorf("nested key = %q", s.NestedKey.NestedValueKey.NestedNestedKey)
	}
}
