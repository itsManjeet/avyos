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

package drmkms

import (
	"os"
	"strconv"
	"sync"
	"syscall"
	"unsafe"

	"avyos.dev/lib/graphics/event"
)

// Linux input_event (64-bit): {timeval(16), type(2), code(2), value(4)} = 24 bytes.
type inputEvent struct {
	Sec, Usec int64
	Type      uint16
	Code      uint16
	Value     int32
}

const inputEventSize = 24

// Event types.
const (
	evSyn = 0x00
	evKey = 0x01
	evRel = 0x02
	evAbs = 0x03
)

// Relative axis codes.
const (
	relX      = 0x00
	relY      = 0x01
	relHWheel = 0x06
	relWheel  = 0x08
)

// Absolute axis codes.
const (
	absX = 0x00
	absY = 0x01
)

// Button codes.
const (
	btnLeft   = 0x110
	btnRight  = 0x111
	btnMiddle = 0x112
	btnSide   = 0x113
	btnExtra  = 0x114
)

// EVIOCGBIT(ev, len) = _IOC(_IOC_READ, 'E', 0x20+ev, len)
func eviocgbit(ev, length uintptr) uintptr {
	return (2 << 30) | (length << 16) | ('E' << 8) | (0x20 + ev)
}

func eviocgabs(abs uintptr) uintptr {
	return ioc(2, 'E', 0x40+abs, unsafe.Sizeof(inputAbsInfo{}))
}

type inputAbsInfo struct {
	Value      int32
	Minimum    int32
	Maximum    int32
	Fuzz       int32
	Flat       int32
	Resolution int32
}

func hasBit(bits []byte, bit int) bool {
	if bit/8 >= len(bits) {
		return false
	}
	return bits[bit/8]&(1<<uint(bit%8)) != 0
}

type inputDevice struct {
	file    *os.File
	fd      int
	isKB    bool
	isMouse bool
	absX    inputAbsInfo
	absY    inputAbsInfo
	hasAbsX bool
	hasAbsY bool
}

// motionAccum accumulates pointer deltas between SYN events.
type motionAccum struct {
	dx, dy         int32
	absX, absY     int32
	wheel, hwheel  int32
	hasMouse       bool
	hasAbsX        bool
	hasAbsY        bool
	hasWheel, hasH bool
}

type evdevManager struct {
	backend *Backend
	devices []*inputDevice
	done    chan struct{}
	wg      sync.WaitGroup

	mu       sync.Mutex
	pointerX float64
	pointerY float64
	mods     event.Modifiers
	screenW  int
	screenH  int
}

func newEvdevManager(b *Backend, screenW, screenH int) *evdevManager {
	return &evdevManager{
		backend:  b,
		done:     make(chan struct{}),
		screenW:  screenW,
		screenH:  screenH,
		pointerX: float64(screenW) / 2,
		pointerY: float64(screenH) / 2,
	}
}

func (m *evdevManager) start() {
	for i := range 32 {
		path := "/dev/input/event" + strconv.Itoa(i)
		if path == "" {
			continue
		}
		f, err := os.OpenFile(path, os.O_RDONLY|syscall.O_NONBLOCK, 0)
		if err != nil {
			continue
		}
		fd := int(f.Fd())

		// Query device capabilities.
		var evBits [4]byte
		if ioctl(uintptr(fd), eviocgbit(0, 4), unsafe.Pointer(&evBits)) != nil {
			f.Close()
			continue
		}
		hasKey := hasBit(evBits[:], evKey)
		hasRel := hasBit(evBits[:], evRel)
		hasAbs := hasBit(evBits[:], evAbs)
		if !hasKey && !hasRel && !hasAbs {
			f.Close()
			continue
		}

		dev := &inputDevice{file: f, fd: fd}
		if hasKey {
			var keyBits [96]byte // 768 bits
			if ioctl(uintptr(fd), eviocgbit(evKey, 96), unsafe.Pointer(&keyBits)) == nil {
				if hasBit(keyBits[:], 30) { // KEY_A
					dev.isKB = true
				}
				if hasBit(keyBits[:], btnLeft) {
					dev.isMouse = true
				}
			}
		}
		if hasRel {
			dev.isMouse = true
		}
		if hasAbs {
			var absBits [8]byte
			if ioctl(uintptr(fd), eviocgbit(evAbs, uintptr(len(absBits))), unsafe.Pointer(&absBits[0])) == nil {
				if info, ok := readAbsInfo(fd, absX); ok && hasBit(absBits[:], absX) {
					dev.absX = info
					dev.hasAbsX = true
				}
				if info, ok := readAbsInfo(fd, absY); ok && hasBit(absBits[:], absY) {
					dev.absY = info
					dev.hasAbsY = true
				}
				if dev.hasAbsX || dev.hasAbsY {
					dev.isMouse = true
				}
			}
		}
		if !dev.isKB && !dev.isMouse {
			f.Close()
			continue
		}

		// Switch back to blocking for the read goroutine.
		_ = syscall.SetNonblock(fd, false)
		m.devices = append(m.devices, dev)
		m.wg.Add(1)
		go m.readDevice(dev)
	}
}

func (m *evdevManager) stop() {
	close(m.done)
	for _, d := range m.devices {
		d.file.Close()
	}
	m.wg.Wait()
}

func (m *evdevManager) readDevice(dev *inputDevice) {
	defer m.wg.Done()
	buf := make([]byte, inputEventSize*64)
	var acc motionAccum
	for {
		select {
		case <-m.done:
			return
		default:
		}
		n, err := syscall.Read(dev.fd, buf)
		if err != nil {
			if err == syscall.EINTR || err == syscall.EAGAIN {
				continue
			}
			return
		}
		for off := 0; off+inputEventSize <= n; off += inputEventSize {
			ev := (*inputEvent)(unsafe.Pointer(&buf[off]))
			switch ev.Type {
			case evSyn:
				m.flushAccum(dev, &acc)
			case evKey:
				m.handleKey(ev)
			case evRel:
				m.accumRel(&acc, ev)
			case evAbs:
				m.accumAbs(&acc, ev)
			}
		}
	}
}

func readAbsInfo(fd int, code uint16) (inputAbsInfo, bool) {
	var info inputAbsInfo
	if ioctl(uintptr(fd), eviocgabs(uintptr(code)), unsafe.Pointer(&info)) != nil {
		return inputAbsInfo{}, false
	}
	return info, true
}

func (m *evdevManager) accumRel(a *motionAccum, ev *inputEvent) {
	switch ev.Code {
	case relX:
		a.dx += ev.Value
		a.hasMouse = true
	case relY:
		a.dy += ev.Value
		a.hasMouse = true
	case relWheel:
		a.wheel += ev.Value
		a.hasWheel = true
	case relHWheel:
		a.hwheel += ev.Value
		a.hasH = true
	}
}

func (m *evdevManager) accumAbs(a *motionAccum, ev *inputEvent) {
	switch ev.Code {
	case absX:
		a.absX = ev.Value
		a.hasAbsX = true
		a.hasMouse = true
	case absY:
		a.absY = ev.Value
		a.hasAbsY = true
		a.hasMouse = true
	}
}

func scaleAbsolute(value int32, info inputAbsInfo, screen int) float64 {
	if screen <= 1 {
		return 0
	}
	if info.Maximum <= info.Minimum {
		if value < 0 {
			return 0
		}
		if value >= int32(screen) {
			return float64(screen - 1)
		}
		return float64(value)
	}

	if value < info.Minimum {
		value = info.Minimum
	}
	if value > info.Maximum {
		value = info.Maximum
	}

	span := float64(info.Maximum - info.Minimum)
	pos := float64(value-info.Minimum) * float64(screen-1) / span
	if pos < 0 {
		return 0
	}
	max := float64(screen - 1)
	if pos > max {
		return max
	}
	return pos
}

func (m *evdevManager) flushAccum(dev *inputDevice, a *motionAccum) {
	m.mu.Lock()
	if a.hasAbsX && dev.hasAbsX {
		m.pointerX = scaleAbsolute(a.absX, dev.absX, m.screenW)
	}
	if a.hasAbsY && dev.hasAbsY {
		m.pointerY = scaleAbsolute(a.absY, dev.absY, m.screenH)
	}
	if a.hasMouse {
		m.pointerX += float64(a.dx)
		m.pointerY += float64(a.dy)
		if m.pointerX < 0 {
			m.pointerX = 0
		}
		if m.pointerY < 0 {
			m.pointerY = 0
		}
		if m.pointerX >= float64(m.screenW) {
			m.pointerX = float64(m.screenW - 1)
		}
		if m.pointerY >= float64(m.screenH) {
			m.pointerY = float64(m.screenH - 1)
		}
	}
	x, y := m.pointerX, m.pointerY
	m.mu.Unlock()

	if a.hasWheel || a.hasH {
		m.backend.pushEvent(event.ScrollEvent{
			X: x, Y: y,
			DX: float64(a.hwheel) * 15,
			DY: float64(-a.wheel) * 15,
		})
	}
	if a.hasMouse {
		m.backend.pushEvent(event.CursorEvent{X: x, Y: y})
		if m.backend.cursor != nil {
			m.backend.cursor.move(int32(x), int32(y))
		}
	}
	*a = motionAccum{}
}

func (m *evdevManager) handleKey(ev *inputEvent) {
	code := ev.Code
	value := ev.Value // 0=release, 1=press, 2=repeat

	// Mouse buttons.
	if code >= btnLeft && code <= btnExtra {
		var btn event.Button
		switch code {
		case btnLeft:
			btn = event.ButtonLeft
		case btnRight:
			btn = event.ButtonRight
		case btnMiddle:
			btn = event.ButtonMiddle
		case btnSide:
			btn = event.ButtonBack
		case btnExtra:
			btn = event.ButtonForward
		default:
			return
		}
		m.mu.Lock()
		x, y, mods := m.pointerX, m.pointerY, m.mods
		m.mu.Unlock()
		m.backend.pushEvent(event.ButtonEvent{
			Button: btn, X: x, Y: y, Mods: mods,
			Down: value != 0,
		})
		return
	}

	// Keyboard keys.
	kc := linuxKeyToKeyCode(uint32(code))
	m.mu.Lock()
	switch kc {
	case event.KeyShift:
		if value != 0 {
			m.mods |= event.ModShift
		} else {
			m.mods &^= event.ModShift
		}
	case event.KeyCtrl:
		if value != 0 {
			m.mods |= event.ModCtrl
		} else {
			m.mods &^= event.ModCtrl
		}
	case event.KeyAlt:
		if value != 0 {
			m.mods |= event.ModAlt
		} else {
			m.mods &^= event.ModAlt
		}
	case event.KeySuper:
		if value != 0 {
			m.mods |= event.ModSuper
		} else {
			m.mods &^= event.ModSuper
		}
	}
	mods := m.mods
	m.mu.Unlock()

	// value: 0=release, 1=press, 2=repeat (treated as press)
	m.backend.pushEvent(event.KeyEvent{Key: kc, Mods: mods, Down: value != 0})
}

// linuxKeyToKeyCode maps Linux input keycodes (KEY_*) to event.KeyCode.
func linuxKeyToKeyCode(k uint32) event.KeyCode {
	switch k {
	case 1:
		return event.KeyEscape
	case 2:
		return event.Key1
	case 3:
		return event.Key2
	case 4:
		return event.Key3
	case 5:
		return event.Key4
	case 6:
		return event.Key5
	case 7:
		return event.Key6
	case 8:
		return event.Key7
	case 9:
		return event.Key8
	case 10:
		return event.Key9
	case 11:
		return event.Key0
	case 12:
		return event.KeyMinus
	case 13:
		return event.KeyEqual
	case 14:
		return event.KeyBackspace
	case 15:
		return event.KeyTab
	case 16:
		return event.KeyQ
	case 17:
		return event.KeyW
	case 18:
		return event.KeyE
	case 19:
		return event.KeyR
	case 20:
		return event.KeyT
	case 21:
		return event.KeyY
	case 22:
		return event.KeyU
	case 23:
		return event.KeyI
	case 24:
		return event.KeyO
	case 25:
		return event.KeyP
	case 26:
		return event.KeyLeftBracket
	case 27:
		return event.KeyRightBracket
	case 28:
		return event.KeyEnter
	case 29:
		return event.KeyCtrl
	case 30:
		return event.KeyA
	case 31:
		return event.KeyS
	case 32:
		return event.KeyD
	case 33:
		return event.KeyF
	case 34:
		return event.KeyG
	case 35:
		return event.KeyH
	case 36:
		return event.KeyJ
	case 37:
		return event.KeyK
	case 38:
		return event.KeyL
	case 39:
		return event.KeySemicolon
	case 40:
		return event.KeyApostrophe
	case 41:
		return event.KeyGrave
	case 42:
		return event.KeyShift
	case 43:
		return event.KeyBackslash
	case 44:
		return event.KeyZ
	case 45:
		return event.KeyX
	case 46:
		return event.KeyC
	case 47:
		return event.KeyV
	case 48:
		return event.KeyB
	case 49:
		return event.KeyN
	case 50:
		return event.KeyM
	case 51:
		return event.KeyComma
	case 52:
		return event.KeyPeriod
	case 53:
		return event.KeySlash
	case 54:
		return event.KeyShift
	case 56:
		return event.KeyAlt
	case 57:
		return event.KeySpace
	case 59:
		return event.KeyF1
	case 60:
		return event.KeyF2
	case 61:
		return event.KeyF3
	case 62:
		return event.KeyF4
	case 63:
		return event.KeyF5
	case 64:
		return event.KeyF6
	case 65:
		return event.KeyF7
	case 66:
		return event.KeyF8
	case 67:
		return event.KeyF9
	case 68:
		return event.KeyF10
	case 87:
		return event.KeyF11
	case 88:
		return event.KeyF12
	case 97:
		return event.KeyCtrl
	case 100:
		return event.KeyAlt
	case 102:
		return event.KeyHome
	case 103:
		return event.KeyArrowUp
	case 104:
		return event.KeyPageUp
	case 105:
		return event.KeyArrowLeft
	case 106:
		return event.KeyArrowRight
	case 107:
		return event.KeyEnd
	case 108:
		return event.KeyArrowDown
	case 109:
		return event.KeyPageDown
	case 110:
		return event.KeyInsert
	case 111:
		return event.KeyDelete
	case 125:
		return event.KeySuper
	default:
		return event.KeyUnknown
	}
}
