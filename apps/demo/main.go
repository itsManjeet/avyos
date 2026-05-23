package main

import (
	"log"
	"runtime"

	"avyos.dev/pkg/graphics/app"
)

func init() { runtime.LockOSThread() }

func main() {
	app.Options.Title = "Widget Showcase"
	if err := app.Run(ShowcaseApp{}); err != nil {
		log.Fatal(err)
	}
}
