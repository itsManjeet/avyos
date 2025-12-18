package main

import (
	"flag"
	"fmt"
	"os"
)

var (
	commands = map[string]func([]string) error{
		"listen": listen,
	}
)

func main() {
	flag.Parse()

	if flag.NArg() == 0 {
		flag.Usage()
		return
	}

	cmd, ok := commands[flag.Arg(0)]
	if !ok {
		fmt.Fprintf(os.Stderr, "Error: unknown command %v\n", flag.Arg(0))
		os.Exit(1)
	}

	if err := cmd(flag.Args()[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
