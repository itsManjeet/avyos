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

// Package event defines input and lifecycle events for the graphics framework.
package event

import (
	"encoding/binary"
	"errors"
	"math"
)

// Event is the common interface for all input and lifecycle events.
// Only types defined in this package implement Event.
type Event any

// KeyCode identifies a physical keyboard key.
type KeyCode uint32

const (
	KeyUnknown KeyCode = iota

	// Letters
	KeyA
	KeyB
	KeyC
	KeyD
	KeyE
	KeyF
	KeyG
	KeyH
	KeyI
	KeyJ
	KeyK
	KeyL
	KeyM
	KeyN
	KeyO
	KeyP
	KeyQ
	KeyR
	KeyS
	KeyT
	KeyU
	KeyV
	KeyW
	KeyX
	KeyY
	KeyZ

	// Digits
	Key0
	Key1
	Key2
	Key3
	Key4
	Key5
	Key6
	Key7
	Key8
	Key9

	// Navigation
	KeyArrowUp
	KeyArrowDown
	KeyArrowLeft
	KeyArrowRight
	KeyHome
	KeyEnd
	KeyPageUp
	KeyPageDown

	// Editing
	KeyBackspace
	KeyDelete
	KeyInsert
	KeyTab
	KeyEnter
	KeyEscape
	KeySpace

	// Function keys
	KeyF1
	KeyF2
	KeyF3
	KeyF4
	KeyF5
	KeyF6
	KeyF7
	KeyF8
	KeyF9
	KeyF10
	KeyF11
	KeyF12

	// Modifiers
	KeyShift
	KeyCtrl
	KeyAlt
	KeySuper

	// Printable punctuation
	KeyMinus
	KeyEqual
	KeyLeftBracket
	KeyRightBracket
	KeySemicolon
	KeyApostrophe
	KeyGrave
	KeyBackslash
	KeyComma
	KeyPeriod
	KeySlash
)

// Button identifies a mouse button.
type Button uint8

const (
	ButtonLeft Button = iota + 1
	ButtonMiddle
	ButtonRight
	ButtonBack
	ButtonForward
)

// Modifiers is a bitmask of active modifier keys.
type Modifiers uint8

const (
	ModShift Modifiers = 1 << iota
	ModCtrl
	ModAlt
	ModSuper
)

// CursorShape identifies the preferred pointer visual for the current hover target.
type CursorShape uint32

const (
	CursorDefault CursorShape = iota
	CursorText
	CursorMove
	CursorResizeNS
	CursorResizeEW
	CursorResizeNWSE
	CursorResizeNESW
)

// KeyEvent is emitted when a keyboard key is pressed or released.
type KeyEvent struct {
	Key  KeyCode
	Mods Modifiers
	Down bool // true = pressed, false = released
}

// TextInputEvent is emitted when printable text is typed.
type TextInputEvent struct {
	Rune rune
	Mods Modifiers
}

// ButtonEvent is emitted when a mouse button is pressed or released.
type ButtonEvent struct {
	Button Button
	X, Y   float64
	Mods   Modifiers
	Down   bool // true = pressed, false = released
}

// CursorEvent is emitted when the mouse cursor moves.
type CursorEvent struct {
	X, Y float64
}

// ScrollEvent is emitted when the scroll wheel or touchpad scrolls.
type ScrollEvent struct {
	X, Y   float64
	DX, DY float64
}

// ResizeEvent is emitted when the window is resized.
type ResizeEvent struct {
	Width, Height int
}

// CloseEvent is emitted when the window is requested to close.
type CloseEvent struct{}

// FocusEvent is emitted when the window gains keyboard focus.
type FocusEvent struct{}

// BlurEvent is emitted when the window loses keyboard focus.
type BlurEvent struct{}

// Wire type codes used by Encode and Decode.
const (
	codeKey    = 0x01
	codeText   = 0x02
	codeButton = 0x03
	codeCursor = 0x04
	codeScroll = 0x05
	codeResize = 0x06
	codeClose  = 0x07
	codeFocus  = 0x08
	codeBlur   = 0x09
)

// Encode serialises e to a compact binary representation for IPC transport.
// Returns nil if e is nil.
func Encode(e Event) []byte {
	switch ev := e.(type) {
	case KeyEvent:
		buf := make([]byte, 7)
		buf[0] = codeKey
		binary.LittleEndian.PutUint32(buf[1:], uint32(ev.Key))
		buf[5] = uint8(ev.Mods)
		if ev.Down {
			buf[6] = 1
		}
		return buf
	case TextInputEvent:
		buf := make([]byte, 6)
		buf[0] = codeText
		binary.LittleEndian.PutUint32(buf[1:], uint32(ev.Rune))
		buf[5] = uint8(ev.Mods)
		return buf
	case ButtonEvent:
		buf := make([]byte, 20)
		buf[0] = codeButton
		buf[1] = uint8(ev.Button)
		binary.LittleEndian.PutUint64(buf[2:], math.Float64bits(ev.X))
		binary.LittleEndian.PutUint64(buf[10:], math.Float64bits(ev.Y))
		buf[18] = uint8(ev.Mods)
		if ev.Down {
			buf[19] = 1
		}
		return buf
	case CursorEvent:
		buf := make([]byte, 17)
		buf[0] = codeCursor
		binary.LittleEndian.PutUint64(buf[1:], math.Float64bits(ev.X))
		binary.LittleEndian.PutUint64(buf[9:], math.Float64bits(ev.Y))
		return buf
	case ScrollEvent:
		buf := make([]byte, 33)
		buf[0] = codeScroll
		binary.LittleEndian.PutUint64(buf[1:], math.Float64bits(ev.X))
		binary.LittleEndian.PutUint64(buf[9:], math.Float64bits(ev.Y))
		binary.LittleEndian.PutUint64(buf[17:], math.Float64bits(ev.DX))
		binary.LittleEndian.PutUint64(buf[25:], math.Float64bits(ev.DY))
		return buf
	case ResizeEvent:
		buf := make([]byte, 9)
		buf[0] = codeResize
		binary.LittleEndian.PutUint32(buf[1:], uint32(ev.Width))
		binary.LittleEndian.PutUint32(buf[5:], uint32(ev.Height))
		return buf
	case CloseEvent:
		return []byte{codeClose}
	case FocusEvent:
		return []byte{codeFocus}
	case BlurEvent:
		return []byte{codeBlur}
	default:
		return nil
	}
}

// Decode deserialises an Event from IPC wire data produced by Encode.
func Decode(data []byte) (Event, error) {
	if len(data) < 1 {
		return nil, errors.New("event: empty data")
	}
	switch data[0] {
	case codeKey:
		if len(data) < 7 {
			return nil, errors.New("event: short KeyEvent")
		}
		return KeyEvent{
			Key:  KeyCode(binary.LittleEndian.Uint32(data[1:])),
			Mods: Modifiers(data[5]),
			Down: data[6] != 0,
		}, nil
	case codeText:
		if len(data) < 6 {
			return nil, errors.New("event: short TextInputEvent")
		}
		return TextInputEvent{
			Rune: rune(binary.LittleEndian.Uint32(data[1:])),
			Mods: Modifiers(data[5]),
		}, nil
	case codeButton:
		if len(data) < 20 {
			return nil, errors.New("event: short ButtonEvent")
		}
		return ButtonEvent{
			Button: Button(data[1]),
			X:      math.Float64frombits(binary.LittleEndian.Uint64(data[2:])),
			Y:      math.Float64frombits(binary.LittleEndian.Uint64(data[10:])),
			Mods:   Modifiers(data[18]),
			Down:   data[19] != 0,
		}, nil
	case codeCursor:
		if len(data) < 17 {
			return nil, errors.New("event: short CursorEvent")
		}
		return CursorEvent{
			X: math.Float64frombits(binary.LittleEndian.Uint64(data[1:])),
			Y: math.Float64frombits(binary.LittleEndian.Uint64(data[9:])),
		}, nil
	case codeScroll:
		if len(data) < 33 {
			return nil, errors.New("event: short ScrollEvent")
		}
		return ScrollEvent{
			X:  math.Float64frombits(binary.LittleEndian.Uint64(data[1:])),
			Y:  math.Float64frombits(binary.LittleEndian.Uint64(data[9:])),
			DX: math.Float64frombits(binary.LittleEndian.Uint64(data[17:])),
			DY: math.Float64frombits(binary.LittleEndian.Uint64(data[25:])),
		}, nil
	case codeResize:
		if len(data) < 9 {
			return nil, errors.New("event: short ResizeEvent")
		}
		return ResizeEvent{
			Width:  int(int32(binary.LittleEndian.Uint32(data[1:]))),
			Height: int(int32(binary.LittleEndian.Uint32(data[5:]))),
		}, nil
	case codeClose:
		return CloseEvent{}, nil
	case codeFocus:
		return FocusEvent{}, nil
	case codeBlur:
		return BlurEvent{}, nil
	default:
		return nil, errors.New("event: unknown type code")
	}
}
