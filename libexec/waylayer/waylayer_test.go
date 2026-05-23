package main

import (
	"encoding/binary"
	"os"
	"path/filepath"
	"syscall"
	"testing"

	"avyos.dev/lib/graphics/event"
)

func TestWireStringRoundTrip(t *testing.T) {
	var b wireBuilder
	b.uint(42)
	b.string("xdg_toplevel")
	m := &wireMessage{data: b.data}

	v, err := m.uint()
	if err != nil || v != 42 {
		t.Fatalf("uint = %d, %v", v, err)
	}
	value, err := m.string()
	if err != nil || value != "xdg_toplevel" {
		t.Fatalf("string = %q, %v", value, err)
	}
	if err := m.done(); err != nil {
		t.Fatal(err)
	}
}

func TestWireStringRejectsMissingNUL(t *testing.T) {
	data := make([]byte, 8)
	binary.LittleEndian.PutUint32(data, 4)
	copy(data[4:], "nope")
	m := &wireMessage{data: data}
	if _, err := m.string(); err == nil {
		t.Fatal("expected malformed string to fail")
	}
}

func TestCopyRGBAConvertsWaylandFormats(t *testing.T) {
	tests := []struct {
		name   string
		format uint32
		wantA  byte
	}{
		{"argb8888", 0, 0x44},
		{"xrgb8888", 1, 0xff},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			sourcePath := filepath.Join(dir, "source")
			if err := os.WriteFile(sourcePath, []byte{0x11, 0x22, 0x33, 0x44}, 0o600); err != nil {
				t.Fatal(err)
			}
			fd, err := syscall.Open(sourcePath, syscall.O_RDONLY, 0)
			if err != nil {
				t.Fatal(err)
			}
			pool := &shmPool{fd: fd, size: 4}
			buffer, err := newShmBuffer(pool, 0, 1, 1, 4, tt.format)
			if err != nil {
				t.Fatal(err)
			}
			outputPath := filepath.Join(dir, "output")
			output, err := os.OpenFile(outputPath, os.O_CREATE|os.O_RDWR, 0o600)
			if err != nil {
				t.Fatal(err)
			}
			if err := output.Truncate(4); err != nil {
				t.Fatal(err)
			}
			buffer.outputs[0] = output
			buffer.outputPaths[0] = outputPath

			path, err := buffer.copyRGBA()
			if err != nil {
				t.Fatal(err)
			}
			got, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			want := []byte{0x33, 0x22, 0x11, tt.wantA}
			if string(got) != string(want) {
				t.Fatalf("RGBA = %v, want %v", got, want)
			}
			buffer.close()
			pool.close()
		})
	}
}

func TestLinuxKeyMapping(t *testing.T) {
	tests := map[event.KeyCode]uint32{
		event.KeyA:     30,
		event.KeyB:     48,
		event.KeyQ:     16,
		event.Key0:     11,
		event.KeyEnter: 28,
	}
	for key, want := range tests {
		if got := linuxKey(key); got != want {
			t.Errorf("linuxKey(%d) = %d, want %d", key, got, want)
		}
	}
}
