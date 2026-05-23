package main

import (
	"log"
	"runtime"

	"avyos.dev/pkg/graphics/app"
)

func init() { runtime.LockOSThread() }

func main() {
	app.Options.Title = "Terminal"
	if err := app.Run(TerminalApp{}); err != nil {
		log.Fatal(err)
	}
}
