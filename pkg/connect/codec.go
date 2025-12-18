package connect

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"math"
	"reflect"
)

const (
	headerSize = 4 + 4 + 2 + 2
)

type Message struct {
	Sender      uint32
	Destination uint32
	Event       uint16
	/* size   uint16 */
	Payload []byte
}

func Read(r io.Reader) (Message, error) {
	var buf [headerSize]byte
	if n, err := io.ReadFull(r, buf[:]); err != nil {
		return Message{}, err
	} else if n != headerSize {
		return Message{}, fmt.Errorf("invalid header")
	}
	var m Message
	m.Sender = binary.BigEndian.Uint32(buf[:4])
	m.Destination = binary.LittleEndian.Uint32(buf[4:8])
	m.Event = binary.LittleEndian.Uint16(buf[8:10])

	size := binary.LittleEndian.Uint16(buf[10:12])
	m.Payload = make([]byte, size)

	if n, err := io.ReadFull(r, m.Payload); err != nil {
		return Message{}, err
	} else if n != int(size) {
		return Message{}, fmt.Errorf("invalid payload")
	}
	return m, nil
}

func Write(w io.Writer, m Message) error {
	var buf [headerSize]byte

	binary.LittleEndian.PutUint32(buf[:4], m.Sender)
	binary.LittleEndian.PutUint32(buf[4:8], m.Destination)
	binary.LittleEndian.PutUint16(buf[8:10], m.Event)
	binary.LittleEndian.PutUint16(buf[10:12], uint16(len(m.Payload)))

	if n, err := w.Write(buf[:]); err != nil {
		return err
	} else if n != headerSize {
		return fmt.Errorf("write failed")
	}

	if n, err := w.Write(m.Payload); err != nil {
		return err
	} else if n != len(m.Payload) {
		return fmt.Errorf("write failed")
	}

	return nil
}

func Encode[T any](v T) ([]byte, error) {
	var buf bytes.Buffer
	if err := encodeValue(&buf, reflect.ValueOf(v)); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func encodeValue(w *bytes.Buffer, v reflect.Value) error {
	if v.Kind() == reflect.Pointer {
		return fmt.Errorf("pointers not supported")
	}

	switch v.Kind() {

	case reflect.Bool:
		if v.Bool() {
			w.WriteByte(1)
		} else {
			w.WriteByte(0)
		}

	case reflect.Int, reflect.Int64:
		binary.Write(w, binary.LittleEndian, uint64(v.Int()))

	case reflect.Int8:
		w.WriteByte(byte(v.Int()))

	case reflect.Int16:
		binary.Write(w, binary.LittleEndian, uint16(v.Int()))

	case reflect.Int32:
		binary.Write(w, binary.LittleEndian, uint32(v.Int()))

	case reflect.Uint, reflect.Uint64:
		binary.Write(w, binary.LittleEndian, v.Uint())

	case reflect.Uint8:
		w.WriteByte(byte(v.Uint()))

	case reflect.Uint16:
		binary.Write(w, binary.LittleEndian, uint16(v.Uint()))

	case reflect.Uint32:
		binary.Write(w, binary.LittleEndian, uint32(v.Uint()))

	case reflect.Float32:
		binary.Write(w, binary.LittleEndian, math.Float32bits(float32(v.Float())))

	case reflect.Float64:
		binary.Write(w, binary.LittleEndian, math.Float64bits(v.Float()))

	case reflect.String:
		s := v.String()
		binary.Write(w, binary.LittleEndian, uint32(len(s)))
		w.WriteString(s)

	case reflect.Slice:
		l := v.Len()
		binary.Write(w, binary.LittleEndian, uint32(l))
		for i := 0; i < l; i++ {
			if err := encodeValue(w, v.Index(i)); err != nil {
				return err
			}
		}

	case reflect.Array:
		for i := 0; i < v.Len(); i++ {
			if err := encodeValue(w, v.Index(i)); err != nil {
				return err
			}
		}

	case reflect.Struct:
		t := v.Type()
		for i := 0; i < v.NumField(); i++ {
			if !t.Field(i).IsExported() {
				continue
			}
			if err := encodeValue(w, v.Field(i)); err != nil {
				return err
			}
		}

	default:
		return fmt.Errorf("unsupported kind: %s", v.Kind())
	}

	return nil
}

func Decode[T any](b []byte) (T, error) {
	var v T
	r := bytes.NewReader(b)
	if err := decodeValue(r, reflect.ValueOf(&v).Elem()); err != nil {
		return v, err
	}
	return v, nil
}

func decodeValue(r *bytes.Reader, v reflect.Value) error {
	if v.Kind() == reflect.Pointer {
		return fmt.Errorf("pointers not supported")
	}

	switch v.Kind() {
	case reflect.Bool:
		b, _ := r.ReadByte()
		v.SetBool(b != 0)

	case reflect.Int, reflect.Int64:
		var u uint64
		binary.Read(r, binary.LittleEndian, &u)
		v.SetInt(int64(u))

	case reflect.Int8:
		b, _ := r.ReadByte()
		v.SetInt(int64(int8(b)))

	case reflect.Int16:
		var u uint16
		binary.Read(r, binary.LittleEndian, &u)
		v.SetInt(int64(int16(u)))

	case reflect.Int32:
		var u uint32
		binary.Read(r, binary.LittleEndian, &u)
		v.SetInt(int64(int32(u)))

	case reflect.Uint, reflect.Uint64:
		var u uint64
		binary.Read(r, binary.LittleEndian, &u)
		v.SetUint(u)

	case reflect.Uint8:
		b, _ := r.ReadByte()
		v.SetUint(uint64(b))

	case reflect.Uint16:
		var u uint16
		binary.Read(r, binary.LittleEndian, &u)
		v.SetUint(uint64(u))

	case reflect.Uint32:
		var u uint32
		binary.Read(r, binary.LittleEndian, &u)
		v.SetUint(uint64(u))

	case reflect.Float32:
		var u uint32
		binary.Read(r, binary.LittleEndian, &u)
		v.SetFloat(float64(math.Float32frombits(u)))

	case reflect.Float64:
		var u uint64
		binary.Read(r, binary.LittleEndian, &u)
		v.SetFloat(math.Float64frombits(u))

	case reflect.String:
		var n uint32
		binary.Read(r, binary.LittleEndian, &n)
		buf := make([]byte, n)
		r.Read(buf)
		v.SetString(string(buf))

	case reflect.Slice:
		var n uint32
		binary.Read(r, binary.LittleEndian, &n)
		v.Set(reflect.MakeSlice(v.Type(), int(n), int(n)))
		for i := 0; i < int(n); i++ {
			if err := decodeValue(r, v.Index(i)); err != nil {
				return err
			}
		}

	case reflect.Array:
		for i := 0; i < v.Len(); i++ {
			if err := decodeValue(r, v.Index(i)); err != nil {
				return err
			}
		}

	case reflect.Struct:
		t := v.Type()
		for i := 0; i < v.NumField(); i++ {
			if !t.Field(i).IsExported() {
				continue
			}
			if err := decodeValue(r, v.Field(i)); err != nil {
				return err
			}
		}

	default:
		return fmt.Errorf("unsupported kind: %s", v.Kind())
	}

	return nil
}
