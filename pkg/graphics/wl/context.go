// This file is based on https://github.com/rajveermalviya/go-wayland
// Thanks to https://github.com/rajveermalviya for whole
package wl

import (
	"bytes"
	"fmt"
	"math"
	"net"
	"unsafe"

	"golang.org/x/sys/unix"
)

type Context struct {
	conn    *net.UnixConn
	objects map[uint32]Proxy
	curId   uint32
}

var oobSpace = unix.CmsgSpace(4)

func (c *Context) Register(p Proxy) {
	c.curId++

}

func (c *Context) Unregister(p Proxy) {
	delete(c.objects, p.Id())
}

func (c *Context) GetProxy(id uint32) Proxy {
	return c.objects[id]
}

func (c *Context) Close() error {
	return c.conn.Close()
}

func (c *Context) Dispatch() error {
	senderID, opcode, fd, data, err := c.ReadMsg()
	if err != nil {
		return fmt.Errorf("ctx.Dispatch: unable to read msg: %w", err)
	}

	sender, ok := c.objects[senderID]
	if ok {
		if sender, ok := sender.(Dispatcher); ok {
			sender.Dispatch(opcode, fd, data)
		} else {
			return fmt.Errorf("ctx.Dispatch: sender doesn't implement Dispatch method (senderID=%d)", senderID)
		}
	} else {
		return fmt.Errorf("ctx.Dispatch: unable find sender (senderID=%d)", senderID)
	}

	return nil
}

func (ctx *Context) ReadMsg() (senderID uint32, opcode uint32, fd int, msg []byte, err error) {
	fd = -1

	oob := make([]byte, oobSpace)
	header := make([]byte, 8)

	n, oobn, _, _, err := ctx.conn.ReadMsgUnix(header, oob)
	if err != nil {
		return senderID, opcode, fd, msg, err
	}
	if n != 8 {
		return senderID, opcode, fd, msg, fmt.Errorf("ctx.ReadMsg: incorrect number of bytes read for header (n=%d)", n)
	}

	if oobn > 0 {
		fds, err := getFdsFromOob(oob, oobn, "header")
		if err != nil {
			return senderID, opcode, fd, msg, fmt.Errorf("ctx.ReadMsg: %w", err)
		}

		if len(fds) > 0 {
			fd = fds[0]
		}
	}

	senderID = Uint32(header[:4])
	opcodeAndSize := Uint32(header[4:8])
	opcode = opcodeAndSize & 0xffff
	size := opcodeAndSize >> 16

	msgSize := int(size) - 8
	if msgSize == 0 {
		return senderID, opcode, fd, nil, nil
	}

	msg = make([]byte, msgSize)

	if fd == -1 {
		if oobn > 0 {
			oob = make([]byte, oobSpace)
		}

		n, oobn, _, _, err = ctx.conn.ReadMsgUnix(msg, oob)
	} else {
		n, err = ctx.conn.Read(msg)
	}
	if err != nil {
		return senderID, opcode, fd, msg, fmt.Errorf("ctx.ReadMsg: %w", err)
	}
	if n != msgSize {
		return senderID, opcode, fd, msg, fmt.Errorf("ctx.ReadMsg: incorrect number of bytes read for msg (n=%d, msgSize=%d)", n, msgSize)
	}

	if fd == -1 && oobn > 0 {
		fds, err := getFdsFromOob(oob, oobn, "msg")
		if err != nil {
			return senderID, opcode, fd, msg, fmt.Errorf("ctx.ReadMsg: %w", err)
		}

		if len(fds) > 0 {
			fd = fds[0]
		}
	}

	return senderID, opcode, fd, msg, nil
}

func getFdsFromOob(oob []byte, oobn int, source string) ([]int, error) {
	if oobn > len(oob) {
		return nil, fmt.Errorf("getFdsFromOob: incorrect number of bytes read from %s for oob (oobn=%d)", source, oobn)
	}
	scms, err := unix.ParseSocketControlMessage(oob)
	if err != nil {
		return nil, fmt.Errorf("getFdsFromOob: unable to parse control message from %s: %w", source, err)
	}

	var fdsRet []int
	for _, scm := range scms {
		fds, err := unix.ParseUnixRights(&scm)
		if err != nil {
			return nil, fmt.Errorf("getFdsFromOob: unable to parse unix rights from %s: %w", source, err)
		}

		fdsRet = append(fdsRet, fds...)
	}

	return fdsRet, nil
}

func Uint32(src []byte) uint32 {
	_ = src[3]
	return *(*uint32)(unsafe.Pointer(&src[0]))
}

func String(src []byte) string {
	idx := bytes.IndexByte(src, 0)
	src = src[:idx:idx]
	return *(*string)(unsafe.Pointer(&src))
}

func Fixed(src []byte) float64 {
	_ = src[3]
	fx := *(*int32)(unsafe.Pointer(&src[0]))
	return fixedToFloat64(fx)
}

func fixedToFloat64(f int32) float64 {
	u_i := (1023+44)<<52 + (1 << 51) + int64(f)
	u_d := math.Float64frombits(uint64(u_i))
	return u_d - (3 << 43)
}

func fixedFromfloat64(d float64) int32 {
	u_d := d + (3 << (51 - 8))
	u_i := int64(math.Float64bits(u_d))
	return int32(u_i)
}

func PaddedLen(l int) int {
	if (l & 0x3) != 0 {
		return l + (4 - (l & 0x3))
	}
	return l
}

func (ctx *Context) WriteMsg(b []byte, oob []byte) error {
	n, oobn, err := ctx.conn.WriteMsgUnix(b, oob, nil)
	if err != nil {
		return err
	}
	if n != len(b) || oobn != len(oob) {
		return fmt.Errorf("ctx.WriteMsg: incorrect number of bytes written (n=%d oobn=%d)", n, oobn)
	}

	return nil
}

func PutUint32(dst []byte, v uint32) {
	_ = dst[3]
	*(*uint32)(unsafe.Pointer(&dst[0])) = v
}

func PutFixed(dst []byte, f float64) {
	fx := fixedFromfloat64(f)
	_ = dst[3]
	*(*int32)(unsafe.Pointer(&dst[0])) = fx
}

func PutString(dst []byte, v string, l int) {
	PutUint32(dst[:4], uint32(l))
	copy(dst[4:], []byte(v))
}

func PutArray(dst []byte, a []byte) {
	PutUint32(dst[:4], uint32(len(a)))
	copy(dst[4:], a)
}
