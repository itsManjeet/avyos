package settings

import (
	"path/filepath"
	"reflect"
	"testing"
)

func TestStoreSetGetListDelete(t *testing.T) {
	store := Store{
		UserPath:   filepath.Join(t.TempDir(), "user.conf"),
		SystemPath: filepath.Join(t.TempDir(), "system.conf"),
	}

	if err := store.Set(ScopeUser, "appearance.theme", "graphite"); err != nil {
		t.Fatalf("Set user theme: %v", err)
	}
	if err := store.Set(ScopeUser, "display.scale", int64(125)); err != nil {
		t.Fatalf("Set user scale: %v", err)
	}
	if err := store.Set(ScopeSystem, "network.hostname", "avyos-box"); err != nil {
		t.Fatalf("Set system hostname: %v", err)
	}

	got, ok, err := store.Get(ScopeUser, "appearance.theme")
	if err != nil {
		t.Fatalf("Get user theme: %v", err)
	}
	if !ok || got != "graphite" {
		t.Fatalf("Get user theme = (%v, %v), want (graphite, true)", got, ok)
	}

	entries, err := store.List(ScopeUser, "appearance")
	if err != nil {
		t.Fatalf("List user appearance: %v", err)
	}
	want := []Entry{{Path: "appearance.theme", Value: "graphite"}}
	if !reflect.DeepEqual(entries, want) {
		t.Fatalf("List user appearance = %#v, want %#v", entries, want)
	}

	if err := store.Delete(ScopeUser, "appearance.theme"); err != nil {
		t.Fatalf("Delete user theme: %v", err)
	}
	_, ok, err = store.Get(ScopeUser, "appearance.theme")
	if err != nil {
		t.Fatalf("Get deleted theme: %v", err)
	}
	if ok {
		t.Fatal("expected deleted user theme to be missing")
	}
}

func TestParseValue(t *testing.T) {
	value, err := ParseValue(`[true, 42, "accent"]`)
	if err != nil {
		t.Fatalf("ParseValue failed: %v", err)
	}
	want := []any{true, int64(42), "accent"}
	if !reflect.DeepEqual(value, want) {
		t.Fatalf("ParseValue = %#v, want %#v", value, want)
	}
}

func TestFormatValue(t *testing.T) {
	got := FormatValue("graphite")
	if got != `"graphite"` {
		t.Fatalf("FormatValue = %q, want %q", got, `"graphite"`)
	}
}
