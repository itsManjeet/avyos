package main

import (
	"syscall"
	"testing"
)

func TestShouldStopDesktopOnSignal(t *testing.T) {
	if shouldStopDesktopOnSignal(syscall.SIGINT) {
		t.Fatal("SIGINT should be ignored by the desktop service")
	}
	if !shouldStopDesktopOnSignal(syscall.SIGTERM) {
		t.Fatal("SIGTERM should stop the desktop service")
	}
	if !shouldStopDesktopOnSignal(syscall.SIGHUP) {
		t.Fatal("SIGHUP should stop the desktop service")
	}
	if !shouldStopDesktopOnSignal(syscall.SIGQUIT) {
		t.Fatal("SIGQUIT should stop the desktop service")
	}
}
