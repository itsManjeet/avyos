package main

import "fmt"

func list(args []string) error {
	services, err := client.List("")
	if err != nil {
		return err
	}
	for _, s := range services {
		fmt.Printf("- %s\n", s)
	}
	return nil
}
