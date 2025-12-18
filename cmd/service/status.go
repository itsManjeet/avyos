package main

import (
	"flag"
	"fmt"
)

func status(args []string) error {
	f := flag.NewFlagSet("status", flag.ContinueOnError)

	if err := f.Parse(args); err != nil {
		return err
	}

	if f.NArg() != 1 {
		f.Usage()
		return nil
	}

	name := f.Arg(0)
	res, err := client.Status(name)
	if err != nil {
		return err
	}
	fmt.Println(res)

	return nil
}
