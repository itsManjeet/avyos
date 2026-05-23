package main

import (
	"encoding/binary"
	"fmt"
	"net"
	"os"
	"sync"
	"sync/atomic"
	"time"

	desktopapi "avyos.dev/api/desktop"
)

const (
	ifaceDisplay       = "wl_display"
	ifaceRegistry      = "wl_registry"
	ifaceCallback      = "wl_callback"
	ifaceCompositor    = "wl_compositor"
	ifaceSurface       = "wl_surface"
	ifaceRegion        = "wl_region"
	ifaceShm           = "wl_shm"
	ifaceShmPool       = "wl_shm_pool"
	ifaceBuffer        = "wl_buffer"
	ifaceSeat          = "wl_seat"
	ifacePointer       = "wl_pointer"
	ifaceKeyboard      = "wl_keyboard"
	ifaceOutput        = "wl_output"
	ifaceXDGWMBase     = "xdg_wm_base"
	ifaceXDGSurface    = "xdg_surface"
	ifaceXDGToplevel   = "xdg_toplevel"
	ifaceDecorationMgr = "zxdg_decoration_manager_v1"
	ifaceDecoration    = "zxdg_toplevel_decoration_v1"
)

type global struct {
	name    uint32
	iface   string
	version uint32
}

var globals = []global{
	{1, ifaceCompositor, 4},
	{2, ifaceShm, 1},
	{3, ifaceSeat, 5},
	{4, ifaceOutput, 2},
	{5, ifaceXDGWMBase, 1},
	{6, ifaceDecorationMgr, 1},
}

type object struct {
	iface   string
	version uint32
	data    any
}

type client struct {
	wire    *wireConn
	desktop *desktopapi.Client

	mu      sync.RWMutex
	objects map[uint32]object
	windows map[uint32]*surface
	closed  bool

	serial atomic.Uint32
	start  time.Time

	pointerSurface  uint32
	keyboardSurface uint32
	mods            uint32
	keysDown        map[uint32]bool
}

func newClient(conn *net.UnixConn, desktop *desktopapi.Client) *client {
	c := &client{
		wire:     &wireConn{conn: conn},
		desktop:  desktop,
		objects:  map[uint32]object{1: {iface: ifaceDisplay, version: 1}},
		windows:  make(map[uint32]*surface),
		keysDown: make(map[uint32]bool),
		start:    time.Now(),
	}
	c.serial.Store(1)
	desktop.Desktop.OnCloseRequested(c.handleCloseRequested)
	desktop.Desktop.OnResize(c.handleResize)
	desktop.Desktop.OnInput(c.handleInput)
	return c
}

func (c *client) nextSerial() uint32 { return c.serial.Add(1) }
func (c *client) timeMS() uint32     { return uint32(time.Since(c.start).Milliseconds()) }

func (c *client) run() error {
	for {
		msg, err := c.wire.recv()
		if err != nil {
			return err
		}
		err = c.dispatch(msg)
		msg.close()
		if err != nil {
			_ = c.sendProtocolError(msg.object, 1, err.Error())
			return err
		}
	}
}

func (c *client) close() {
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return
	}
	c.closed = true
	objects := c.objects
	c.objects = make(map[uint32]object)
	c.windows = make(map[uint32]*surface)
	c.mu.Unlock()

	for _, obj := range objects {
		switch value := obj.data.(type) {
		case *surface:
			value.close(c)
		case *shmPool:
			value.close()
		case *shmBuffer:
			value.close()
		}
	}
	_ = c.desktop.Close()
	_ = c.wire.close()
}

func (c *client) dispatch(msg *wireMessage) error {
	c.mu.RLock()
	obj, ok := c.objects[msg.object]
	c.mu.RUnlock()
	if !ok {
		return fmt.Errorf("request for unknown object %d", msg.object)
	}

	var err error
	switch obj.iface {
	case ifaceDisplay:
		err = c.handleDisplay(msg)
	case ifaceRegistry:
		err = c.handleRegistry(msg)
	case ifaceCompositor:
		err = c.handleCompositor(msg)
	case ifaceSurface:
		err = c.handleSurface(msg, obj.data.(*surface))
	case ifaceRegion:
		err = c.handleRegion(msg)
	case ifaceShm:
		err = c.handleShm(msg)
	case ifaceShmPool:
		err = c.handleShmPool(msg, obj.data.(*shmPool))
	case ifaceBuffer:
		err = c.handleBuffer(msg, obj.data.(*shmBuffer))
	case ifaceSeat:
		err = c.handleSeat(msg, obj.version)
	case ifacePointer:
		err = c.handlePointer(msg, obj.version)
	case ifaceKeyboard:
		err = c.handleKeyboard(msg, obj.version)
	case ifaceXDGWMBase:
		err = c.handleXDGWMBase(msg)
	case ifaceXDGSurface:
		err = c.handleXDGSurface(msg, obj.data.(*surface))
	case ifaceXDGToplevel:
		err = c.handleXDGToplevel(msg, obj.data.(*surface))
	case ifaceDecorationMgr:
		err = c.handleDecorationManager(msg)
	case ifaceDecoration:
		err = c.handleDecoration(msg)
	default:
		err = fmt.Errorf("unsupported request on %s", obj.iface)
	}
	if err != nil {
		return fmt.Errorf("%s@%d opcode %d: %w", obj.iface, msg.object, msg.opcode, err)
	}
	return msg.done()
}

func (c *client) addObject(id uint32, obj object) error {
	if id < 2 {
		return fmt.Errorf("invalid new object ID %d", id)
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if _, exists := c.objects[id]; exists {
		return fmt.Errorf("object ID %d already exists", id)
	}
	c.objects[id] = obj
	return nil
}

func (c *client) removeObject(id uint32) {
	c.mu.Lock()
	delete(c.objects, id)
	c.mu.Unlock()
	var b wireBuilder
	b.uint(id)
	_ = c.wire.send(1, 1, b)
}

func (c *client) object(id uint32, iface string) (object, error) {
	c.mu.RLock()
	obj, ok := c.objects[id]
	c.mu.RUnlock()
	if !ok || obj.iface != iface {
		return object{}, fmt.Errorf("object %d is not %s", id, iface)
	}
	return obj, nil
}

func (c *client) sendProtocolError(id, code uint32, message string) error {
	var b wireBuilder
	b.uint(id)
	b.uint(code)
	b.string(message)
	return c.wire.send(1, 0, b)
}

func (c *client) handleDisplay(msg *wireMessage) error {
	switch msg.opcode {
	case 0: // sync
		id, err := msg.uint()
		if err != nil {
			return err
		}
		if err := c.addObject(id, object{iface: ifaceCallback, version: 1}); err != nil {
			return err
		}
		var b wireBuilder
		b.uint(c.timeMS())
		if err := c.wire.send(id, 0, b); err != nil {
			return err
		}
		c.removeObject(id)
		return nil
	case 1: // get_registry
		id, err := msg.uint()
		if err != nil {
			return err
		}
		if err := c.addObject(id, object{iface: ifaceRegistry, version: 1}); err != nil {
			return err
		}
		for _, g := range globals {
			var b wireBuilder
			b.uint(g.name)
			b.string(g.iface)
			b.uint(g.version)
			if err := c.wire.send(id, 0, b); err != nil {
				return err
			}
		}
		return nil
	default:
		return fmt.Errorf("unknown request")
	}
}

func (c *client) handleRegistry(msg *wireMessage) error {
	if msg.opcode != 0 {
		return fmt.Errorf("unknown request")
	}
	name, err := msg.uint()
	if err != nil {
		return err
	}
	iface, err := msg.string()
	if err != nil {
		return err
	}
	version, err := msg.uint()
	if err != nil {
		return err
	}
	id, err := msg.uint()
	if err != nil {
		return err
	}
	var found *global
	for i := range globals {
		if globals[i].name == name {
			found = &globals[i]
			break
		}
	}
	if found == nil || found.iface != iface {
		return fmt.Errorf("invalid global %d (%s)", name, iface)
	}
	if version == 0 || version > found.version {
		return fmt.Errorf("unsupported %s version %d", iface, version)
	}
	if err := c.addObject(id, object{iface: iface, version: version}); err != nil {
		return err
	}
	switch iface {
	case ifaceShm:
		for _, format := range []uint32{0, 1} { // ARGB8888, XRGB8888
			var b wireBuilder
			b.uint(format)
			if err := c.wire.send(id, 0, b); err != nil {
				return err
			}
		}
	case ifaceSeat:
		if version >= 2 {
			var b wireBuilder
			b.string("seat0")
			if err := c.wire.send(id, 1, b); err != nil {
				return err
			}
		}
		var b wireBuilder
		b.uint(3) // pointer | keyboard
		return c.wire.send(id, 0, b)
	case ifaceOutput:
		return c.sendOutput(id, version)
	}
	return nil
}

func (c *client) handleCompositor(msg *wireMessage) error {
	id, err := msg.uint()
	if err != nil {
		return err
	}
	switch msg.opcode {
	case 0:
		s := &surface{id: id, scale: 1}
		return c.addObject(id, object{iface: ifaceSurface, version: 4, data: s})
	case 1:
		return c.addObject(id, object{iface: ifaceRegion, version: 1})
	default:
		return fmt.Errorf("unknown request")
	}
}

func (c *client) handleRegion(msg *wireMessage) error {
	switch msg.opcode {
	case 0:
		c.removeObject(msg.object)
		return nil
	case 1, 2:
		for range 4 {
			if _, err := msg.int(); err != nil {
				return err
			}
		}
		return nil
	default:
		return fmt.Errorf("unknown request")
	}
}

func (c *client) handleShm(msg *wireMessage) error {
	if msg.opcode != 0 {
		return fmt.Errorf("unknown request")
	}
	id, err := msg.uint()
	if err != nil {
		return err
	}
	fd, err := msg.fd()
	if err != nil {
		return err
	}
	size, err := msg.int()
	if err != nil {
		_ = syscallClose(fd)
		return err
	}
	if size <= 0 {
		_ = syscallClose(fd)
		return fmt.Errorf("invalid shared-memory pool size %d", size)
	}
	pool := &shmPool{fd: fd, size: int(size)}
	if err := c.addObject(id, object{iface: ifaceShmPool, version: 1, data: pool}); err != nil {
		pool.close()
		return err
	}
	return nil
}

func (c *client) handleShmPool(msg *wireMessage, pool *shmPool) error {
	switch msg.opcode {
	case 0:
		id, err := msg.uint()
		if err != nil {
			return err
		}
		offset, err := msg.int()
		if err != nil {
			return err
		}
		width, err := msg.int()
		if err != nil {
			return err
		}
		height, err := msg.int()
		if err != nil {
			return err
		}
		stride, err := msg.int()
		if err != nil {
			return err
		}
		format, err := msg.uint()
		if err != nil {
			return err
		}
		buffer, err := newShmBuffer(pool, int(offset), int(width), int(height), int(stride), format)
		if err != nil {
			return err
		}
		if err := c.addObject(id, object{iface: ifaceBuffer, version: 1, data: buffer}); err != nil {
			buffer.close()
			return err
		}
		return nil
	case 1:
		pool.markDestroyed()
		c.removeObject(msg.object)
		return nil
	case 2:
		size, err := msg.int()
		if err != nil {
			return err
		}
		return pool.resize(int(size))
	default:
		return fmt.Errorf("unknown request")
	}
}

func (c *client) handleBuffer(msg *wireMessage, buffer *shmBuffer) error {
	if msg.opcode != 0 {
		return fmt.Errorf("unknown request")
	}
	buffer.markDestroyed()
	c.removeObject(msg.object)
	return nil
}

func (c *client) handleSeat(msg *wireMessage, version uint32) error {
	switch msg.opcode {
	case 0:
		id, err := msg.uint()
		if err != nil {
			return err
		}
		return c.addObject(id, object{iface: ifacePointer, version: version})
	case 1:
		id, err := msg.uint()
		if err != nil {
			return err
		}
		if err := c.addObject(id, object{iface: ifaceKeyboard, version: version}); err != nil {
			return err
		}
		return c.sendKeymap(id, version)
	case 2:
		return fmt.Errorf("touch input is not supported")
	case 3:
		if version < 5 {
			return fmt.Errorf("release requires version 5")
		}
		c.removeObject(msg.object)
		return nil
	default:
		return fmt.Errorf("unknown request")
	}
}

func (c *client) handlePointer(msg *wireMessage, version uint32) error {
	switch msg.opcode {
	case 0: // set_cursor: custom cursor surfaces are accepted but not composited
		if _, err := msg.uint(); err != nil {
			return err
		}
		surfaceID, err := msg.uint()
		if err != nil {
			return err
		}
		if surfaceID != 0 {
			obj, err := c.object(surfaceID, ifaceSurface)
			if err != nil {
				return err
			}
			s := obj.data.(*surface)
			s.mu.Lock()
			s.cursor = true
			s.mu.Unlock()
		}
		if _, err := msg.int(); err != nil {
			return err
		}
		_, err = msg.int()
		return err
	case 1:
		if version < 3 {
			return fmt.Errorf("release requires version 3")
		}
		c.removeObject(msg.object)
		return nil
	default:
		return fmt.Errorf("unknown request")
	}
}

func (c *client) handleKeyboard(msg *wireMessage, version uint32) error {
	if msg.opcode != 0 || version < 3 {
		return fmt.Errorf("unknown request")
	}
	c.removeObject(msg.object)
	return nil
}

func (c *client) handleXDGWMBase(msg *wireMessage) error {
	switch msg.opcode {
	case 0:
		c.removeObject(msg.object)
		return nil
	case 1:
		return fmt.Errorf("xdg_positioner is not supported")
	case 2:
		id, err := msg.uint()
		if err != nil {
			return err
		}
		surfaceID, err := msg.uint()
		if err != nil {
			return err
		}
		obj, err := c.object(surfaceID, ifaceSurface)
		if err != nil {
			return err
		}
		s := obj.data.(*surface)
		s.mu.Lock()
		hasRole := s.xdgID != 0 || s.cursor
		if !hasRole {
			s.xdgID = id
		}
		s.mu.Unlock()
		if hasRole {
			return fmt.Errorf("surface already has a role")
		}
		return c.addObject(id, object{iface: ifaceXDGSurface, version: 1, data: s})
	case 3:
		_, err := msg.uint()
		return err
	default:
		return fmt.Errorf("unknown request")
	}
}

func (c *client) handleXDGSurface(msg *wireMessage, s *surface) error {
	switch msg.opcode {
	case 0:
		s.mu.Lock()
		hasToplevel := s.toplevelID != 0
		if !hasToplevel {
			s.xdgID = 0
		}
		s.mu.Unlock()
		if hasToplevel {
			return fmt.Errorf("xdg_toplevel still exists")
		}
		c.removeObject(msg.object)
		return nil
	case 1:
		s.mu.Lock()
		hasToplevel := s.toplevelID != 0
		s.mu.Unlock()
		if hasToplevel {
			return fmt.Errorf("surface already has a toplevel")
		}
		id, err := msg.uint()
		if err != nil {
			return err
		}
		if err := c.addObject(id, object{iface: ifaceXDGToplevel, version: 1, data: s}); err != nil {
			return err
		}
		s.mu.Lock()
		s.toplevelID = id
		s.mu.Unlock()
		return nil
	case 2:
		return fmt.Errorf("xdg_popup is not supported")
	case 3:
		for range 4 {
			if _, err := msg.int(); err != nil {
				return err
			}
		}
		return nil
	case 4:
		serial, err := msg.uint()
		if err == nil {
			s.ackedConfigure = serial
		}
		return err
	default:
		return fmt.Errorf("unknown request")
	}
}

func (c *client) handleXDGToplevel(msg *wireMessage, s *surface) error {
	switch msg.opcode {
	case 0:
		s.mu.Lock()
		s.toplevelID = 0
		s.mu.Unlock()
		c.unmapWindow(s)
		c.removeObject(msg.object)
		return nil
	case 1:
		_, err := msg.uint()
		return err
	case 2:
		title, err := msg.string()
		if err != nil {
			return err
		}
		s.title = title
		return c.updateWindowState(s)
	case 3:
		appID, err := msg.string()
		if err != nil {
			return err
		}
		s.appID = appID
		return c.updateWindowState(s)
	case 4, 5, 6: // window menu, interactive move, interactive resize
		if _, err := msg.uint(); err != nil {
			return err
		}
		if _, err := msg.uint(); err != nil {
			return err
		}
		if msg.opcode == 4 {
			if _, err := msg.int(); err != nil {
				return err
			}
			_, err := msg.int()
			return err
		}
		if msg.opcode == 6 {
			_, err := msg.uint()
			return err
		}
		return nil
	case 7:
		w, err := msg.int()
		if err != nil {
			return err
		}
		h, err := msg.int()
		if err != nil {
			return err
		}
		s.maxWidth, s.maxHeight = int(w), int(h)
		return nil
	case 8:
		w, err := msg.int()
		if err != nil {
			return err
		}
		h, err := msg.int()
		if err != nil {
			return err
		}
		s.minWidth, s.minHeight = int(w), int(h)
		return nil
	case 9, 10, 12, 13:
		return nil
	case 11:
		_, err := msg.uint()
		return err
	default:
		return fmt.Errorf("unknown request")
	}
}

func (c *client) handleDecorationManager(msg *wireMessage) error {
	switch msg.opcode {
	case 0:
		c.removeObject(msg.object)
		return nil
	case 1:
		id, err := msg.uint()
		if err != nil {
			return err
		}
		toplevelID, err := msg.uint()
		if err != nil {
			return err
		}
		if _, err := c.object(toplevelID, ifaceXDGToplevel); err != nil {
			return err
		}
		if err := c.addObject(id, object{iface: ifaceDecoration, version: 1}); err != nil {
			return err
		}
		var b wireBuilder
		b.uint(2) // server-side decorations
		return c.wire.send(id, 0, b)
	default:
		return fmt.Errorf("unknown request")
	}
}

func (c *client) handleDecoration(msg *wireMessage) error {
	switch msg.opcode {
	case 0:
		c.removeObject(msg.object)
		return nil
	case 1:
		_, err := msg.uint()
		return err
	case 2:
		return nil
	default:
		return fmt.Errorf("unknown request")
	}
}

func (c *client) sendOutput(id, version uint32) error {
	var geometry wireBuilder
	geometry.int(0)
	geometry.int(0)
	geometry.int(508)
	geometry.int(285)
	geometry.int(0)
	geometry.string("AvyOS")
	geometry.string("Virtual Display")
	geometry.int(0)
	if err := c.wire.send(id, 0, geometry); err != nil {
		return err
	}
	var mode wireBuilder
	mode.uint(3) // current | preferred
	mode.int(1920)
	mode.int(1080)
	mode.int(60000)
	if err := c.wire.send(id, 1, mode); err != nil {
		return err
	}
	if version >= 2 {
		var scale wireBuilder
		scale.int(1)
		if err := c.wire.send(id, 3, scale); err != nil {
			return err
		}
		return c.wire.send(id, 2, wireBuilder{})
	}
	return nil
}

func syscallClose(fd int) error { return os.NewFile(uintptr(fd), "wayland-fd").Close() }

func uint32Array(values ...uint32) []byte {
	b := make([]byte, len(values)*4)
	for i, value := range values {
		binary.LittleEndian.PutUint32(b[i*4:], value)
	}
	return b
}
