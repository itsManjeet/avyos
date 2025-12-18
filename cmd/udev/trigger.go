package main

import (
	"encoding/binary"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

// const (
// 	defaultTimeout = 5
// )

func trigger(args []string) error {
	f := flag.NewFlagSet("trigger", flag.ContinueOnError)

	// timeout := f.Int("timeout", defaultTimeout, "Timeout in seconds")
	wait := f.Bool("wait", false, "Wait until finish")

	if err := f.Parse(args); err != nil {
		return err
	}

	filepath.Walk("/sys", func(path string, info fs.FileInfo, err error) error {
		if err != nil || info.IsDir() || filepath.Base(path) != "uevent" {
			return err
		}
		_ = os.WriteFile(path, []byte("add\n"), 0644)
		return nil
	})
	if *wait {
		r, err := conn.Call(2, 2, nil)
		if err != nil {
			return err
		}
		res := binary.LittleEndian.Uint32(r) == 1
		fmt.Printf("IsIdle == %v", res)
	}
	return nil
}
