package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNewAppCommandUsesSeparateProcessGroup(t *testing.T) {
	cmd, err := newAppCommand(launcherApp{
		Name:     "Test App",
		ExecPath: "sh",
	})
	if err != nil {
		t.Fatalf("newAppCommand failed: %v", err)
	}
	if cmd.SysProcAttr == nil || !cmd.SysProcAttr.Setpgid {
		t.Fatal("expected app command to run in a separate process group")
	}
	if cmd.Stdin == nil {
		t.Fatal("expected app command stdin to be redirected away from the desktop tty")
	}
}

func TestLoadLauncherAppDecodesSVGAtLauncherSize(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "manifest.json"), []byte(`{"id":"dev.avyos.test","name":"Test App"}`), 0o644); err != nil {
		t.Fatalf("WriteFile(manifest): %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "icon.svg"), []byte(`<svg xmlns="http://www.w3.org/2000/svg" width="16" height="16"><rect width="16" height="16" fill="#5294e2"/></svg>`), 0o644); err != nil {
		t.Fatalf("WriteFile(icon): %v", err)
	}

	app, ok := loadLauncherApp(dir)
	if !ok {
		t.Fatal("expected launcher app to load")
	}
	if app.Icon == nil {
		t.Fatal("expected launcher app icon to load")
	}
	if got := app.Icon.Bounds().Dx(); got != launcherIconDecodeSize {
		t.Fatalf("icon width = %d, want %d", got, launcherIconDecodeSize)
	}
	if got := app.Icon.Bounds().Dy(); got != launcherIconDecodeSize {
		t.Fatalf("icon height = %d, want %d", got, launcherIconDecodeSize)
	}
}
