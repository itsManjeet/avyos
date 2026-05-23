package main

import (
	"log"
	"runtime"

	"avyos.dev/lib/graphics/app"
)

func init() { runtime.LockOSThread() }

func main() {
	app.Options.Title = "Notepad"
	if err := app.Run(NotepadApp{}); err != nil {
		log.Fatal(err)
	}
}
