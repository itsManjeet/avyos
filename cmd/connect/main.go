package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"avyos.dev/pkg/connect"
)

var (
	commands = map[string]func([]string) error{
		"ping": ping,
	}

	conn *connect.Connection
)

func main() {
	flag.Parse()

	if flag.NArg() == 0 {
		flag.Usage()
		return
	}

	cmd, ok := commands[flag.Arg(0)]
	if !ok {
		fmt.Fprintf(os.Stderr, "Error: unknown command %s\n", flag.Arg(0))
		os.Exit(1)
	}

	var err error
	conn, err = connect.SystemBus()
	if err != nil {
		log.Fatal(err)
	}

	if err := cmd(flag.Args()[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
