package main

import (
	"log"
	"os"
	"path/filepath"
	"runtime"

	"avyos.dev/pkg/graphics/app"
)

func init() { runtime.LockOSThread() }

func main() {
	ensureUserDirs()
	app.Options.Title = "File Manager"
	if err := app.Run(FilesApp{}); err != nil {
		log.Fatal(err)
	}
}

func ensureUserDirs() {
	if HOME := os.Getenv("HOME"); HOME != "" {
		for _, dir := range []string{"Desktop", "Documents", "Downloads", "Pictures", "Music", "Videos"} {
			if err := os.MkdirAll(filepath.Join(HOME, dir), 0755); err != nil {
				log.Printf("Failed to create user dir %s: %v", dir, err)
			}
		}
	}
}
