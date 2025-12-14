package main

import (
	"fmt"

	"avyos.dev/pkg/lipi"
)

func builtinClear([]lipi.Value) (lipi.Value, error) {
	fmt.Printf("\033[H\033[J")
	return nil, nil
}
