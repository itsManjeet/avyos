package main

import (
	"encoding/binary"
	"io"

	"avyos.dev/pkg/connect"
)

const (
	UdevObjectId  = 2
	UdevIdleEvent = 2
)

func StartService(u *udev) error {
	conn, err := connect.SystemBus()
	if err != nil {
		return err
	}

	if err := conn.Register(UdevObjectId); err != nil {
		return err
	}

	for {
		mesg, err := conn.Recieve()
		if err != nil {
			if err == io.EOF {
				return nil
			}
			return err
		}

		switch mesg.Event {
		case UdevIdleEvent:
			r := u.isIdle()
			var buf [4]byte
			if r {
				binary.LittleEndian.PutUint32(buf[:], uint32(1))
			} else {
				binary.LittleEndian.PutUint32(buf[:], uint32(0))
			}

			conn.Send(connect.Message{
				Sender:      UdevObjectId,
				Destination: mesg.Sender,
				Event:       connect.BusReplyEvent,
				Payload:     buf[:],
			})
		default:
			conn.Send(connect.Message{
				Sender:      UdevObjectId,
				Destination: mesg.Sender,
				Event:       connect.BusErrorEvent,
				Payload:     []byte(connect.UnkownEvent.Error()),
			})
		}
	}
}
