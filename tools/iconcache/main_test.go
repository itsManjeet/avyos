package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestBuildCacheIndexesFlatThemeVariants(t *testing.T) {
	root := t.TempDir()
	actionsDir := filepath.Join(root, "actions")
	if err := os.MkdirAll(actionsDir, 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}

	files := map[string]string{
		"bookmark-new.svg":           `<svg xmlns="http://www.w3.org/2000/svg"/>`,
		"bookmark-new-symbolic.svg":  `<svg xmlns="http://www.w3.org/2000/svg"/>`,
		"pane-show-symbolic-rtl.svg": `<svg xmlns="http://www.w3.org/2000/svg"/>`,
	}
	for name, contents := range files {
		if err := os.WriteFile(filepath.Join(actionsDir, name), []byte(contents), 0o644); err != nil {
			t.Fatalf("WriteFile(%s) error = %v", name, err)
		}
	}

	cache, err := buildCache(root)
	if err != nil {
		t.Fatalf("buildCache() error = %v", err)
	}

	if got := cache.Icons["bookmark-new"]; got != "actions/bookmark-new.svg" {
		t.Fatalf("regular entry = %q, want actions/bookmark-new.svg", got)
	}
	if got := cache.Icons["bookmark-new-symbolic"]; got != "actions/bookmark-new-symbolic.svg" {
		t.Fatalf("symbolic entry = %q, want actions/bookmark-new-symbolic.svg", got)
	}
	if got := cache.Icons["pane-show-symbolic-rtl"]; got != "actions/pane-show-symbolic-rtl.svg" {
		t.Fatalf("RTL symbolic entry = %q, want actions/pane-show-symbolic-rtl.svg", got)
	}
}
