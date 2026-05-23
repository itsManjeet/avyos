package main

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	gsettings "avyos.dev/pkg/settings"
)

func TestRunCLI(t *testing.T) {
	dir := t.TempDir()
	store := gsettings.Store{
		UserPath:   filepath.Join(dir, "user.conf"),
		SystemPath: filepath.Join(dir, "system.conf"),
	}

	if _, err := captureStdout(t, func() error {
		return run([]string{"set", "appearance.theme", "graphite"}, store)
	}); err != nil {
		t.Fatalf("run set user: %v", err)
	}
	if _, err := captureStdout(t, func() error {
		return run([]string{"set", "-scope", "system", "display.brightness", "80"}, store)
	}); err != nil {
		t.Fatalf("run set system: %v", err)
	}

	out, err := captureStdout(t, func() error {
		return run([]string{"get", "appearance.theme"}, store)
	})
	if err != nil {
		t.Fatalf("run get: %v", err)
	}
	if strings.TrimSpace(out) != `"graphite"` {
		t.Fatalf("run get output = %q, want %q", strings.TrimSpace(out), `"graphite"`)
	}

	out, err = captureStdout(t, func() error {
		return run([]string{"list", "-scope", "all"}, store)
	})
	if err != nil {
		t.Fatalf("run list: %v", err)
	}
	if !strings.Contains(out, "appearance.theme") || !strings.Contains(out, "display.brightness") {
		t.Fatalf("run list output missing expected settings:\n%s", out)
	}

	if _, err := captureStdout(t, func() error {
		return run([]string{"delete", "appearance.theme"}, store)
	}); err != nil {
		t.Fatalf("run delete: %v", err)
	}
	if err := run([]string{"get", "appearance.theme"}, store); err == nil {
		t.Fatal("expected deleted setting lookup to fail")
	}
}

func captureStdout(t *testing.T, fn func() error) (string, error) {
	t.Helper()

	old := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe stdout: %v", err)
	}
	os.Stdout = w
	defer func() { os.Stdout = old }()

	runErr := fn()
	if err := w.Close(); err != nil {
		t.Fatalf("close stdout writer: %v", err)
	}
	data, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("read stdout: %v", err)
	}
	if err := r.Close(); err != nil {
		t.Fatalf("close stdout reader: %v", err)
	}
	return string(data), runErr
}
