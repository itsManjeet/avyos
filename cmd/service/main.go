package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	api "avyos.dev/api/init"
)

var (
	commands = map[string]func([]string) error{
		"list":   list,
		"status": status,
	}

	client *api.Init
)

func main() {
	flag.Parse()

	if flag.NArg() == 0 {
		flag.Usage()
		return
	}

	cmd, ok := commands[flag.Arg(0)]
	if !ok {
		fmt.Fprintf(os.Stderr, "Error: invalid command %s\n", flag.Arg(0))
		os.Exit(1)
	}

	var err error
	client, err = api.NewInit()
	if err != nil {
		log.Fatal(err)
	}

	if err := cmd(flag.Args()[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
