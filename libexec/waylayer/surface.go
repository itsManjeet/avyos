package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"syscall"

	desktopapi "avyos.dev/api/desktop"
)

type shmPool struct {
	mu        sync.Mutex
	fd        int
	size      int
	refs      int
	destroyed bool
}

func (p *shmPool) retain() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.fd < 0 {
		return fmt.Errorf("shared-memory pool is closed")
	}
	p.refs++
	return nil
}

func (p *shmPool) release() {
	p.mu.Lock()
	if p.refs > 0 {
		p.refs--
	}
	if p.destroyed && p.refs == 0 && p.fd >= 0 {
		_ = syscall.Close(p.fd)
		p.fd = -1
	}
	p.mu.Unlock()
}

func (p *shmPool) markDestroyed() {
	p.mu.Lock()
	p.destroyed = true
	if p.refs == 0 && p.fd >= 0 {
		_ = syscall.Close(p.fd)
		p.fd = -1
	}
	p.mu.Unlock()
}

func (p *shmPool) close() {
	p.mu.Lock()
	if p.fd >= 0 {
		_ = syscall.Close(p.fd)
		p.fd = -1
	}
	p.destroyed = true
	p.mu.Unlock()
}

func (p *shmPool) resize(size int) error {
	if size <= 0 {
		return fmt.Errorf("invalid shared-memory pool size %d", size)
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if size < p.size {
		return fmt.Errorf("shared-memory pools cannot shrink")
	}
	p.size = size
	return nil
}

type shmBuffer struct {
	mu sync.Mutex

	pool   *shmPool
	offset int
	width  int
	height int
	stride int
	format uint32
	id     uint32

	outputs     [2]*os.File
	outputPaths [2]string
	nextOutput  int
	refs        int
	busy        bool
	destroyed   bool
	closed      bool
}

func newShmBuffer(pool *shmPool, offset, width, height, stride int, format uint32) (*shmBuffer, error) {
	if offset < 0 || width <= 0 || height <= 0 || stride < width*4 {
		return nil, fmt.Errorf("invalid buffer geometry offset=%d size=%dx%d stride=%d", offset, width, height, stride)
	}
	if format != 0 && format != 1 {
		return nil, fmt.Errorf("unsupported wl_shm format %d", format)
	}
	lastByte := int64(offset) + int64(height-1)*int64(stride) + int64(width)*4
	pool.mu.Lock()
	poolSize := pool.size
	pool.mu.Unlock()
	if lastByte > int64(poolSize) {
		return nil, fmt.Errorf("buffer exceeds shared-memory pool")
	}
	if err := pool.retain(); err != nil {
		return nil, err
	}
	return &shmBuffer{
		pool: pool, offset: offset, width: width, height: height,
		stride: stride, format: format,
	}, nil
}

func (b *shmBuffer) retain() error {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.closed {
		return fmt.Errorf("buffer is closed")
	}
	b.refs++
	return nil
}

func (b *shmBuffer) releaseRef() {
	b.mu.Lock()
	if b.refs > 0 {
		b.refs--
	}
	shouldClose := b.destroyed && b.refs == 0
	b.mu.Unlock()
	if shouldClose {
		b.close()
	}
}

func (b *shmBuffer) markDestroyed() {
	b.mu.Lock()
	b.destroyed = true
	shouldClose := b.refs == 0
	b.mu.Unlock()
	if shouldClose {
		b.close()
	}
}

func (b *shmBuffer) markBusy() {
	b.mu.Lock()
	if !b.closed {
		b.busy = true
	}
	b.mu.Unlock()
}

func (b *shmBuffer) takeRelease() (uint32, bool) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if !b.busy {
		return 0, false
	}
	b.busy = false
	return b.id, !b.destroyed && b.id != 0
}

func (b *shmBuffer) close() {
	b.mu.Lock()
	if b.closed {
		b.mu.Unlock()
		return
	}
	b.closed = true
	outputs := b.outputs
	paths := b.outputPaths
	b.outputs = [2]*os.File{}
	b.outputPaths = [2]string{}
	b.mu.Unlock()
	for i, output := range outputs {
		if output != nil {
			_ = output.Close()
		}
		if paths[i] != "" {
			_ = os.Remove(paths[i])
		}
	}
	b.pool.release()
}

func (b *shmBuffer) copyRGBA() (string, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.closed {
		return "", fmt.Errorf("buffer is closed")
	}
	slot := b.nextOutput
	b.nextOutput = (b.nextOutput + 1) % len(b.outputs)
	if b.outputs[slot] == nil {
		dir := filepath.Join(fmt.Sprintf("/run/user/%d", os.Getuid()), "waylayer-buffers")
		if err := os.MkdirAll(dir, 0o700); err != nil {
			return "", fmt.Errorf("create buffer directory: %w", err)
		}
		file, err := os.CreateTemp(dir, "buffer-*.rgba")
		if err != nil {
			return "", fmt.Errorf("create converted buffer: %w", err)
		}
		if err := file.Chmod(0o600); err != nil {
			file.Close()
			os.Remove(file.Name())
			return "", err
		}
		if err := file.Truncate(int64(b.width * b.height * 4)); err != nil {
			file.Close()
			os.Remove(file.Name())
			return "", err
		}
		b.outputs[slot] = file
		b.outputPaths[slot] = filepath.Clean(file.Name())
	}

	sourceSize := (b.height-1)*b.stride + b.width*4
	source := make([]byte, sourceSize)
	b.pool.mu.Lock()
	if b.pool.fd < 0 {
		b.pool.mu.Unlock()
		return "", fmt.Errorf("shared-memory pool is closed")
	}
	n, err := syscall.Pread(b.pool.fd, source, int64(b.offset))
	b.pool.mu.Unlock()
	if err != nil {
		return "", fmt.Errorf("read Wayland buffer: %w", err)
	}
	if n < len(source) {
		return "", fmt.Errorf("short Wayland buffer read: %d of %d", n, len(source))
	}

	dest := make([]byte, b.width*b.height*4)
	for y := 0; y < b.height; y++ {
		srcRow := source[y*b.stride:]
		dstRow := dest[y*b.width*4:]
		for x := 0; x < b.width; x++ {
			s := srcRow[x*4:]
			d := dstRow[x*4:]
			// wl_shm ARGB/XRGB8888 bytes on little-endian systems are B,G,R,A/X.
			d[0], d[1], d[2] = s[2], s[1], s[0]
			if b.format == 0 {
				d[3] = s[3]
			} else {
				d[3] = 0xff
			}
		}
	}
	if _, err := b.outputs[slot].WriteAt(dest, 0); err != nil {
		return "", fmt.Errorf("write converted buffer: %w", err)
	}
	return b.outputPaths[slot], nil
}

type damageRect struct{ x, y, w, h int }

type surface struct {
	mu sync.Mutex

	id          uint32
	xdgID       uint32
	toplevelID  uint32
	windowID    uint32
	currentPath string
	cursor      bool
	closed      bool

	title string
	appID string

	minWidth, minHeight int
	maxWidth, maxHeight int
	scale               int

	pending       *shmBuffer
	pendingSet    bool
	current       *shmBuffer
	pendingDamage []damageRect
	callbacks     []uint32

	lastConfigure  uint32
	ackedConfigure uint32
}

func (s *surface) setPending(buffer *shmBuffer) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.pending != nil {
		s.pending.releaseRef()
	}
	if buffer != nil {
		if err := buffer.retain(); err != nil {
			return err
		}
	}
	s.pending = buffer
	s.pendingSet = true
	return nil
}

func (s *surface) close(c *client) {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return
	}
	s.closed = true
	pending, current := s.pending, s.current
	s.pending, s.current = nil, nil
	windowID := s.windowID
	s.windowID = 0
	s.currentPath = ""
	s.mu.Unlock()
	if pending != nil {
		pending.releaseRef()
	}
	if current != nil {
		current.releaseRef()
	}
	if windowID != 0 {
		c.mu.Lock()
		delete(c.windows, windowID)
		c.mu.Unlock()
		_ = c.desktop.Desktop.DestroyWindow(desktopapi.WindowRequest{WindowId: windowID})
	}
}

func (c *client) handleSurface(msg *wireMessage, s *surface) error {
	switch msg.opcode {
	case 0:
		s.close(c)
		c.removeObject(msg.object)
		return nil
	case 1:
		bufferID, err := msg.uint()
		if err != nil {
			return err
		}
		var buffer *shmBuffer
		if bufferID != 0 {
			obj, err := c.object(bufferID, ifaceBuffer)
			if err != nil {
				return err
			}
			buffer = obj.data.(*shmBuffer)
			buffer.id = bufferID
		}
		if _, err := msg.int(); err != nil { // x
			return err
		}
		if _, err := msg.int(); err != nil { // y
			return err
		}
		return s.setPending(buffer)
	case 2, 9:
		x, err := msg.int()
		if err != nil {
			return err
		}
		y, err := msg.int()
		if err != nil {
			return err
		}
		w, err := msg.int()
		if err != nil {
			return err
		}
		h, err := msg.int()
		if err != nil {
			return err
		}
		if w > 0 && h > 0 {
			s.mu.Lock()
			s.pendingDamage = append(s.pendingDamage, damageRect{int(x), int(y), int(w), int(h)})
			s.mu.Unlock()
		}
		return nil
	case 3:
		id, err := msg.uint()
		if err != nil {
			return err
		}
		if err := c.addObject(id, object{iface: ifaceCallback, version: 1}); err != nil {
			return err
		}
		s.mu.Lock()
		s.callbacks = append(s.callbacks, id)
		s.mu.Unlock()
		return nil
	case 4, 5:
		regionID, err := msg.uint()
		if err != nil || regionID == 0 {
			return err
		}
		_, err = c.object(regionID, ifaceRegion)
		return err
	case 6:
		return c.commitSurface(s)
	case 7:
		transform, err := msg.int()
		if err != nil {
			return err
		}
		if transform != 0 {
			return fmt.Errorf("buffer transform %d is not supported", transform)
		}
		return nil
	case 8:
		scale, err := msg.int()
		if err != nil {
			return err
		}
		if scale <= 0 {
			return fmt.Errorf("invalid buffer scale %d", scale)
		}
		s.mu.Lock()
		s.scale = int(scale)
		s.mu.Unlock()
		return nil
	default:
		return fmt.Errorf("unknown request")
	}
}

func (c *client) commitSurface(s *surface) error {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return fmt.Errorf("surface is closed")
	}
	pending, pendingSet := s.pending, s.pendingSet
	s.pending, s.pendingSet = nil, false
	damage := s.pendingDamage
	s.pendingDamage = nil
	callbacks := s.callbacks
	s.callbacks = nil
	cursor := s.cursor
	windowID := s.windowID
	currentPath := s.currentPath
	old := s.current
	toplevelID := s.toplevelID
	ackedConfigure := s.ackedConfigure
	if pendingSet {
		s.current = pending
	} else if old != nil {
		// wl_surface attachment state is persistent. A client may update the
		// current shm buffer and commit only damage/frame state.
		pending = old
	}
	s.mu.Unlock()
	oldNeedsRelease := pendingSet && old != nil
	defer func() {
		if !oldNeedsRelease {
			return
		}
		if old == pending {
			// attach retained a pending reference; the original current
			// reference remains authoritative when the same buffer is reused.
			pending.releaseRef()
		} else {
			c.releaseBuffer(old)
		}
	}()

	if cursor {
		if pendingSet && pending != nil {
			pending.markBusy()
			c.notifyBufferRelease(pending)
		}
		c.finishCallbacks(callbacks)
		return nil
	}
	if pendingSet && pending == nil {
		if windowID != 0 {
			_ = c.desktop.Desktop.DestroyWindow(desktopapi.WindowRequest{WindowId: windowID})
			c.mu.Lock()
			delete(c.windows, windowID)
			c.mu.Unlock()
			s.mu.Lock()
			s.windowID = 0
			s.currentPath = ""
			s.mu.Unlock()
		}
		c.finishCallbacks(callbacks)
		return nil
	}
	if pending == nil {
		s.mu.Lock()
		needsInitialConfigure := s.toplevelID != 0 && s.lastConfigure == 0
		s.mu.Unlock()
		if needsInitialConfigure {
			if err := c.configureSurface(s, 0, 0); err != nil {
				return err
			}
		}
		c.finishCallbacks(callbacks)
		return nil
	}
	if toplevelID == 0 || ackedConfigure == 0 {
		return fmt.Errorf("buffer committed before xdg_toplevel configure was acknowledged")
	}

	if pendingSet {
		pending.markBusy()
	}
	path, err := pending.copyRGBA()
	if err != nil {
		return err
	}
	width, height := pending.width, pending.height
	if windowID == 0 {
		resp, err := c.desktop.Desktop.CreateWindow(desktopapi.WindowCreateRequest{
			AppId: s.appID, AppName: s.appID, Title: s.title,
			BufferPath: path, Width: uint32(width), Height: uint32(height),
			MinWidth: uint32(max(s.minWidth, 0)), MinHeight: uint32(max(s.minHeight, 0)),
			ScaleMilli: 1000,
		})
		if err != nil {
			return fmt.Errorf("create desktop window: %w", err)
		}
		windowID = resp.WindowId
		s.mu.Lock()
		s.windowID = windowID
		s.currentPath = path
		s.mu.Unlock()
		c.mu.Lock()
		c.windows[windowID] = s
		c.mu.Unlock()
	} else if currentPath != path {
		if err := c.desktop.Desktop.UpdateWindowBuffer(desktopapi.WindowBufferRequest{
			WindowId: windowID, BufferPath: path, Width: uint32(width), Height: uint32(height), ScaleMilli: 1000,
		}); err != nil {
			return fmt.Errorf("update desktop buffer: %w", err)
		}
		s.mu.Lock()
		s.currentPath = path
		s.mu.Unlock()
	}

	rects := make([]desktopapi.DamageRect, 0, len(damage))
	for _, r := range damage {
		x := max(r.x, 0)
		y := max(r.y, 0)
		w := min(r.w, width-x)
		h := min(r.h, height-y)
		if w > 0 && h > 0 {
			rects = append(rects, desktopapi.DamageRect{X: uint32(x), Y: uint32(y), W: uint32(w), H: uint32(h)})
		}
	}
	if err := c.desktop.Desktop.PresentWindow(desktopapi.WindowPresentRequest{
		WindowId: windowID, BufferPath: path, DamageRects: rects,
	}); err != nil {
		return fmt.Errorf("present desktop window: %w", err)
	}
	c.notifyBufferRelease(pending)
	c.finishCallbacks(callbacks)
	return nil
}

func (c *client) unmapWindow(s *surface) {
	s.mu.Lock()
	windowID := s.windowID
	current := s.current
	s.windowID = 0
	s.current = nil
	s.currentPath = ""
	s.mu.Unlock()
	if windowID != 0 {
		c.mu.Lock()
		delete(c.windows, windowID)
		c.mu.Unlock()
		_ = c.desktop.Desktop.DestroyWindow(desktopapi.WindowRequest{WindowId: windowID})
	}
	if current != nil {
		c.releaseBuffer(current)
	}
}

func (c *client) releaseBuffer(buffer *shmBuffer) {
	c.notifyBufferRelease(buffer)
	buffer.releaseRef()
}

func (c *client) notifyBufferRelease(buffer *shmBuffer) {
	if id, send := buffer.takeRelease(); send {
		_ = c.wire.send(id, 0, wireBuilder{})
	}
}

func (c *client) finishCallbacks(callbacks []uint32) {
	for _, id := range callbacks {
		var b wireBuilder
		b.uint(c.timeMS())
		_ = c.wire.send(id, 0, b)
		c.removeObject(id)
	}
}

func (c *client) configureSurface(s *surface, width, height int) error {
	s.mu.Lock()
	toplevelID, xdgID := s.toplevelID, s.xdgID
	s.mu.Unlock()
	if toplevelID == 0 || xdgID == 0 {
		return nil
	}
	var top wireBuilder
	top.int(int32(width))
	top.int(int32(height))
	top.array(nil)
	if err := c.wire.send(toplevelID, 0, top); err != nil {
		return err
	}
	serial := c.nextSerial()
	var xdg wireBuilder
	xdg.uint(serial)
	if err := c.wire.send(xdgID, 0, xdg); err != nil {
		return err
	}
	s.mu.Lock()
	s.lastConfigure = serial
	s.mu.Unlock()
	return nil
}

func (c *client) updateWindowState(s *surface) error {
	s.mu.Lock()
	windowID, title, appID := s.windowID, s.title, s.appID
	s.mu.Unlock()
	if windowID == 0 {
		return nil
	}
	return c.desktop.Desktop.UpdateWindowState(desktopapi.WindowStateRequest{
		WindowId: windowID, Title: title, Subtitle: appID,
	})
}
