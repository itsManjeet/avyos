package main

import (
	"fmt"
	"os"
	"path/filepath"

	desktopapi "avyos.dev/api/desktop"
	"avyos.dev/lib/graphics/event"
)

const xkbKeymap = `xkb_keymap {
 xkb_keycodes { include "evdev+aliases(qwerty)" };
 xkb_types { include "complete" };
 xkb_compatibility { include "complete" };
 xkb_symbols { include "pc+us+inet(evdev)" };
};
`

func (c *client) handleCloseRequested(req desktopapi.WindowRequest) {
	s := c.surfaceForWindow(req.WindowId)
	if s == nil {
		return
	}
	s.mu.Lock()
	toplevelID := s.toplevelID
	s.mu.Unlock()
	if toplevelID != 0 {
		_ = c.wire.send(toplevelID, 1, wireBuilder{})
	}
}

func (c *client) handleResize(req desktopapi.WindowResizeEvent) {
	s := c.surfaceForWindow(req.WindowId)
	if s == nil {
		return
	}
	_ = c.configureSurface(s, int(req.Width), int(req.Height))
}

func (c *client) handleInput(req desktopapi.WindowInputEvent) {
	s := c.surfaceForWindow(req.WindowId)
	if s == nil {
		return
	}
	ev, err := event.Decode(req.Payload)
	if err != nil {
		return
	}
	switch value := ev.(type) {
	case event.CursorEvent:
		c.sendPointerMotion(s, value.X, value.Y)
	case event.ButtonEvent:
		c.sendPointerMotion(s, value.X, value.Y)
		c.sendPointerButton(s, value)
	case event.ScrollEvent:
		c.sendPointerScroll(value)
	case event.KeyEvent:
		c.sendKeyboardKey(s, value)
	case event.FocusEvent:
		c.focusKeyboard(s)
	case event.BlurEvent:
		c.blurKeyboard(s)
	}
}

func (c *client) surfaceForWindow(id uint32) *surface {
	c.mu.RLock()
	s := c.windows[id]
	c.mu.RUnlock()
	return s
}

func (c *client) inputObjects(iface string) map[uint32]object {
	c.mu.RLock()
	objects := make(map[uint32]object)
	for id, obj := range c.objects {
		if obj.iface == iface {
			objects[id] = obj
		}
	}
	c.mu.RUnlock()
	return objects
}

func (c *client) sendPointerMotion(s *surface, x, y float64) {
	objects := c.inputObjects(ifacePointer)
	if len(objects) == 0 {
		return
	}
	if c.pointerSurface != s.id {
		if c.pointerSurface != 0 {
			serial := c.nextSerial()
			for id, obj := range objects {
				var leave wireBuilder
				leave.uint(serial)
				leave.uint(c.pointerSurface)
				_ = c.wire.send(id, 1, leave)
				c.sendPointerFrame(id, obj.version)
			}
		}
		c.pointerSurface = s.id
		serial := c.nextSerial()
		for id, obj := range objects {
			var enter wireBuilder
			enter.uint(serial)
			enter.uint(s.id)
			enter.fixed(x)
			enter.fixed(y)
			_ = c.wire.send(id, 0, enter)
			c.sendPointerFrame(id, obj.version)
		}
		return
	}
	for id, obj := range objects {
		var motion wireBuilder
		motion.uint(c.timeMS())
		motion.fixed(x)
		motion.fixed(y)
		_ = c.wire.send(id, 2, motion)
		c.sendPointerFrame(id, obj.version)
	}
}

func (c *client) sendPointerButton(s *surface, ev event.ButtonEvent) {
	button := linuxButton(ev.Button)
	if button == 0 {
		return
	}
	if ev.Down {
		c.focusKeyboard(s)
	}
	state := uint32(0)
	if ev.Down {
		state = 1
	}
	serial := c.nextSerial()
	for id, obj := range c.inputObjects(ifacePointer) {
		var b wireBuilder
		b.uint(serial)
		b.uint(c.timeMS())
		b.uint(button)
		b.uint(state)
		_ = c.wire.send(id, 3, b)
		c.sendPointerFrame(id, obj.version)
	}
}

func (c *client) sendPointerScroll(ev event.ScrollEvent) {
	for id, obj := range c.inputObjects(ifacePointer) {
		if obj.version >= 5 {
			var source wireBuilder
			source.uint(0) // wheel
			_ = c.wire.send(id, 6, source)
		}
		if ev.DX != 0 {
			c.sendAxis(id, obj.version, 1, ev.DX)
		}
		if ev.DY != 0 {
			c.sendAxis(id, obj.version, 0, ev.DY)
		}
		c.sendPointerFrame(id, obj.version)
	}
}

func (c *client) sendAxis(id, version, axis uint32, value float64) {
	if version >= 5 {
		var discrete wireBuilder
		discrete.uint(axis)
		step := int32(1)
		if value < 0 {
			step = -1
		}
		discrete.int(step)
		_ = c.wire.send(id, 8, discrete)
	}
	var b wireBuilder
	b.uint(c.timeMS())
	b.uint(axis)
	b.fixed(value)
	_ = c.wire.send(id, 4, b)
}

func (c *client) sendPointerFrame(id, version uint32) {
	if version >= 5 {
		_ = c.wire.send(id, 5, wireBuilder{})
	}
}

func linuxButton(button event.Button) uint32 {
	switch button {
	case event.ButtonLeft:
		return 0x110
	case event.ButtonRight:
		return 0x111
	case event.ButtonMiddle:
		return 0x112
	case event.ButtonBack:
		return 0x116
	case event.ButtonForward:
		return 0x115
	default:
		return 0
	}
}

func (c *client) focusKeyboard(s *surface) {
	if c.keyboardSurface == s.id {
		return
	}
	if c.keyboardSurface != 0 {
		c.blurKeyboardID(c.keyboardSurface)
	}
	c.keyboardSurface = s.id
	serial := c.nextSerial()
	for id := range c.inputObjects(ifaceKeyboard) {
		var enter wireBuilder
		enter.uint(serial)
		enter.uint(s.id)
		enter.array(nil)
		_ = c.wire.send(id, 1, enter)
		c.sendModifiers(id, serial, 0)
	}
}

func (c *client) blurKeyboard(s *surface) {
	if c.keyboardSurface == s.id {
		c.blurKeyboardID(s.id)
	}
}

func (c *client) blurKeyboardID(surfaceID uint32) {
	serial := c.nextSerial()
	for id := range c.inputObjects(ifaceKeyboard) {
		var leave wireBuilder
		leave.uint(serial)
		leave.uint(surfaceID)
		_ = c.wire.send(id, 2, leave)
	}
	c.keyboardSurface = 0
	c.mods = 0
	c.keysDown = make(map[uint32]bool)
}

func (c *client) sendKeyboardKey(s *surface, ev event.KeyEvent) {
	c.focusKeyboard(s)
	key := linuxKey(ev.Key)
	if key == 0 {
		return
	}
	if ev.Down == c.keysDown[key] {
		// The DRM input layer folds Linux repeat (value 2) into Down=true.
		// Clients repeat locally using wl_keyboard.repeat_info, so duplicate
		// presses must not be forwarded as a second logical key-down.
		return
	}
	if ev.Down {
		c.keysDown[key] = true
	} else {
		delete(c.keysDown, key)
	}
	state := uint32(0)
	if ev.Down {
		state = 1
	}
	serial := c.nextSerial()
	for id := range c.inputObjects(ifaceKeyboard) {
		var b wireBuilder
		b.uint(serial)
		b.uint(c.timeMS())
		b.uint(key)
		b.uint(state)
		_ = c.wire.send(id, 3, b)
		mods := xkbModifiers(ev.Mods)
		if mods != c.mods {
			c.sendModifiers(id, serial, mods)
		}
	}
	c.mods = xkbModifiers(ev.Mods)
}

func (c *client) sendModifiers(id, serial, depressed uint32) {
	var b wireBuilder
	b.uint(serial)
	b.uint(depressed)
	b.uint(0)
	b.uint(0)
	b.uint(0)
	_ = c.wire.send(id, 4, b)
}

func xkbModifiers(mods event.Modifiers) uint32 {
	var mask uint32
	if mods&event.ModShift != 0 {
		mask |= 1 << 0
	}
	if mods&event.ModCtrl != 0 {
		mask |= 1 << 2
	}
	if mods&event.ModAlt != 0 {
		mask |= 1 << 3
	}
	if mods&event.ModSuper != 0 {
		mask |= 1 << 6
	}
	return mask
}

func linuxKey(key event.KeyCode) uint32 {
	// Linux letter keycodes follow keyboard rows, not alphabetic order.
	letters := map[event.KeyCode]uint32{
		event.KeyA: 30, event.KeyB: 48, event.KeyC: 46, event.KeyD: 32,
		event.KeyE: 18, event.KeyF: 33, event.KeyG: 34, event.KeyH: 35,
		event.KeyI: 23, event.KeyJ: 36, event.KeyK: 37, event.KeyL: 38,
		event.KeyM: 50, event.KeyN: 49, event.KeyO: 24, event.KeyP: 25,
		event.KeyQ: 16, event.KeyR: 19, event.KeyS: 31, event.KeyT: 20,
		event.KeyU: 22, event.KeyV: 47, event.KeyW: 17, event.KeyX: 45,
		event.KeyY: 21, event.KeyZ: 44,
	}
	if value := letters[key]; value != 0 {
		return value
	}
	if key >= event.Key0 && key <= event.Key9 {
		if key == event.Key0 {
			return 11
		}
		return 2 + uint32(key-event.Key1)
	}
	keys := map[event.KeyCode]uint32{
		event.KeyArrowUp: 103, event.KeyArrowDown: 108, event.KeyArrowLeft: 105, event.KeyArrowRight: 106,
		event.KeyHome: 102, event.KeyEnd: 107, event.KeyPageUp: 104, event.KeyPageDown: 109,
		event.KeyBackspace: 14, event.KeyDelete: 111, event.KeyInsert: 110, event.KeyTab: 15,
		event.KeyEnter: 28, event.KeyEscape: 1, event.KeySpace: 57,
		event.KeyF1: 59, event.KeyF2: 60, event.KeyF3: 61, event.KeyF4: 62,
		event.KeyF5: 63, event.KeyF6: 64, event.KeyF7: 65, event.KeyF8: 66,
		event.KeyF9: 67, event.KeyF10: 68, event.KeyF11: 87, event.KeyF12: 88,
		event.KeyShift: 42, event.KeyCtrl: 29, event.KeyAlt: 56, event.KeySuper: 125,
		event.KeyMinus: 12, event.KeyEqual: 13, event.KeyLeftBracket: 26, event.KeyRightBracket: 27,
		event.KeySemicolon: 39, event.KeyApostrophe: 40, event.KeyGrave: 41, event.KeyBackslash: 43,
		event.KeyComma: 51, event.KeyPeriod: 52, event.KeySlash: 53,
	}
	return keys[key]
}

func (c *client) sendKeymap(id, version uint32) error {
	dir := filepath.Join(fmt.Sprintf("/run/user/%d", os.Getuid()), "waylayer-keymaps")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	file, err := os.CreateTemp(dir, "keymap-*.xkb")
	if err != nil {
		return err
	}
	name := file.Name()
	defer func() {
		file.Close()
		os.Remove(name)
	}()
	data := append([]byte(xkbKeymap), 0)
	if _, err := file.Write(data); err != nil {
		return err
	}
	if _, err := file.Seek(0, 0); err != nil {
		return err
	}
	var b wireBuilder
	b.uint(1)                 // xkb_v1
	b.uint(uint32(len(data))) // fd occupies no bytes; size follows format
	if err := c.wire.send(id, 0, b, int(file.Fd())); err != nil {
		return fmt.Errorf("send keymap: %w", err)
	}
	if version >= 4 {
		var repeat wireBuilder
		repeat.int(25)
		repeat.int(600)
		return c.wire.send(id, 5, repeat)
	}
	return nil
}
