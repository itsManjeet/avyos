//go:build linux

// Copyright (c) 2026 Manjeet Singh <itsmanjeet1998@gmail.com>.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, version 3.
//
// This program is distributed in the hope that it will be useful, but
// WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the GNU
// General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with this program. If not, see <http://www.gnu.org/licenses/>.

package shell

import (
	"fmt"
	"image"
	"os"
	"sync"
	"syscall"
)

// liveMapping holds a single shared-memory buffer that has been mmap'd.
type liveMapping struct {
	data []byte
	img  *image.NRGBA
}

// surface holds a memory-mapped pixel buffer for a remote window.
// The pixel format expected from clients is NRGBA (non-premultiplied RGBA, 4 bytes/pixel).
//
// Buffer lifetime: mappings are added to live on first use and are never
// released until Close(). This is intentional: the compositor alternates
// between two buffer paths every frame (double-buffering), so both mappings
// must remain valid while the main goroutine may still be painting the
// previous frame's image. Calling setBuffer therefore never unmaps memory;
// only Close() does.
type surface struct {
	mu     sync.RWMutex
	img    *image.NRGBA
	live   map[string]*liveMapping // all currently-mapped paths
	width  int
	height int
	scale  int
}

func newSurface() *surface {
	return &surface{live: make(map[string]*liveMapping)}
}

// setBuffer switches the active image to the buffer at path.
// If path has been seen before its existing mapping is reused; otherwise the
// file is mmap'd and added to the live cache. The old mapping is never
// released here — see the type comment for why.
func (s *surface) setBuffer(path string, width, height, scaleMilli int) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if path == "" || width <= 0 || height <= 0 {
		return fmt.Errorf("shell/surface: invalid buffer params path=%q w=%d h=%d", path, width, height)
	}

	// Reuse an existing mapping if the path is already live.
	if m, ok := s.live[path]; ok {
		s.img = m.img
		s.width = width
		s.height = height
		s.scale = scaleMilli
		return nil
	}

	// New path: mmap it and add to the live cache.
	size := width * height * 4
	if size <= 0 {
		return fmt.Errorf("shell/surface: computed size %d is non-positive", size)
	}

	f, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("shell/surface: open buffer file %q: %w", path, err)
	}
	defer f.Close()

	data, err := syscall.Mmap(int(f.Fd()), 0, size, syscall.PROT_READ, syscall.MAP_SHARED)
	if err != nil {
		return fmt.Errorf("shell/surface: mmap %q: %w", path, err)
	}

	img := &image.NRGBA{
		Pix:    data,
		Stride: width * 4,
		Rect:   image.Rect(0, 0, width, height),
	}
	s.live[path] = &liveMapping{data: data, img: img}
	s.img = img
	s.width = width
	s.height = height
	s.scale = scaleMilli
	return nil
}

// image returns the current mapped image, or nil if no buffer is mapped.
// The lock is NOT held after this returns; use lockImage/unlockImage when
// the image pointer must remain valid across a longer operation.
func (s *surface) image() image.Image {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.img == nil {
		return nil
	}
	return s.img
}

// lockImage acquires a read lock and returns the current mapped image.
// The caller MUST call unlockImage exactly once when done using the image.
// Holding the lock prevents setBuffer from swapping the image pointer (and
// thus prevents the client from being acked and reusing the buffer) until
// the caller is finished — eliminating the read/write race during Paint.
func (s *surface) lockImage() image.Image {
	s.mu.RLock()
	return s.img
}

// unlockImage releases the read lock acquired by lockImage.
func (s *surface) unlockImage() {
	s.mu.RUnlock()
}

// Close releases all live mappings. Must only be called from the main
// goroutine after the window has been removed from the compositor's render
// list so that no in-flight Paint call can touch the unmapped memory.
func (s *surface) Close() {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, m := range s.live {
		_ = syscall.Munmap(m.data)
	}
	s.live = make(map[string]*liveMapping)
	s.img = nil
}
