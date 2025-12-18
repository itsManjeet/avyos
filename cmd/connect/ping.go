package main

import (
	"log"

	"avyos.dev/pkg/connect"
)

func ping(args []string) error {
	resp, err := conn.Call(connect.BusObjectId, connect.BusPingEvent, []byte(`ping`))
	if err != nil {
		return err
	}
	log.Printf("%v\n", string(resp))
	return nil
}
