package settings

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"
)

func TestOnChangeSpecificFile(t *testing.T) {
	oldInterval := watchPollInterval
	watchPollInterval = 10 * time.Millisecond
	defer func() { watchPollInterval = oldInterval }()

	root := t.TempDir()
	path := filepath.Join(root, "app.conf")
	writeTestConfig(t, path, "Theme = light\n")

	changesCh := make(chan []Change, 1)
	stop, err := OnChange(func(changes ...Change) {
		select {
		case changesCh <- append([]Change(nil), changes...):
		default:
		}
	}, path)
	if err != nil {
		t.Fatalf("OnChange failed: %v", err)
	}
	defer stop()

	writeTestConfig(t, path, "Theme = dark\n")

	changes := waitForChanges(t, changesCh)
	if len(changes) != 1 {
		t.Fatalf("changes len = %d, want 1", len(changes))
	}
	if changes[0].Path != path {
		t.Fatalf("change path = %q, want %q", changes[0].Path, path)
	}
	if changes[0].Deleted {
		t.Fatal("expected file update, got delete")
	}
	if changes[0].Err != nil {
		t.Fatalf("unexpected change error: %v", changes[0].Err)
	}
	if got := changes[0].Data["Theme"]; got != "dark" {
		t.Fatalf("Theme = %v, want dark", got)
	}
}

func TestOnChangeDefaultRoots(t *testing.T) {
	oldInterval := watchPollInterval
	oldHomeRoot := homeConfigRoot
	oldSystemRoot := systemConfigRoot
	watchPollInterval = 10 * time.Millisecond
	defer func() {
		watchPollInterval = oldInterval
		homeConfigRoot = oldHomeRoot
		systemConfigRoot = oldSystemRoot
	}()

	root := t.TempDir()
	homeConfigRoot = func() string { return filepath.Join(root, "home", ".config") }
	systemConfigRoot = func() string { return filepath.Join(root, "config") }

	changesCh := make(chan []Change, 4)
	stop, err := OnChange(func(changes ...Change) {
		select {
		case changesCh <- append([]Change(nil), changes...):
		default:
		}
	})
	if err != nil {
		t.Fatalf("OnChange failed: %v", err)
	}
	defer stop()

	homePath := filepath.Join(homeConfigRoot(), "demo", "user.conf")
	systemPath := filepath.Join(systemConfigRoot(), "system.conf")
	writeTestConfig(t, homePath, "Accent = ocean\n")
	writeTestConfig(t, systemPath, "Locale = en_US\n")

	changes := waitForChangesWithPaths(t, changesCh, homePath, systemPath)
	if len(changes) != 2 {
		t.Fatalf("changes len = %d, want 2", len(changes))
	}

	got := map[string]map[string]any{}
	for _, change := range changes {
		if change.Err != nil {
			t.Fatalf("unexpected change error for %s: %v", change.Path, change.Err)
		}
		got[change.Path] = change.Data
	}

	if !reflect.DeepEqual(got[homePath], map[string]any{"Accent": "ocean"}) {
		t.Fatalf("home change = %#v", got[homePath])
	}
	if !reflect.DeepEqual(got[systemPath], map[string]any{"Locale": "en_US"}) {
		t.Fatalf("system change = %#v", got[systemPath])
	}
}

func writeTestConfig(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll(%s): %v", path, err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("WriteFile(%s): %v", path, err)
	}
}

func waitForChanges(t *testing.T, ch <-chan []Change) []Change {
	t.Helper()
	select {
	case changes := <-ch:
		return changes
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for settings change")
		return nil
	}
}

func waitForChangesWithPaths(t *testing.T, ch <-chan []Change, wantPaths ...string) []Change {
	t.Helper()
	deadline := time.After(2 * time.Second)
	seen := make(map[string]Change, len(wantPaths))
	for {
		select {
		case changes := <-ch:
			for _, change := range changes {
				seen[change.Path] = change
			}
			if len(seen) >= len(wantPaths) {
				out := make([]Change, 0, len(wantPaths))
				for _, path := range wantPaths {
					change, ok := seen[path]
					if !ok {
						goto keepWaiting
					}
					out = append(out, change)
				}
				return out
			}
		case <-deadline:
			t.Fatalf("timed out waiting for paths %v, got %v", wantPaths, seen)
		}
	keepWaiting:
	}
}
