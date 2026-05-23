package main

import (
	"image/png"
	"log"
	"os"

	"avyos.dev/pkg/graphics/icons"
)

func main() {
	icon, err := icons.Load("start-here", 24)
	if err != nil {
		log.Fatal(err)
	}

	if icon == nil {
		log.Fatal("icon not found")
	}

	file, err := os.Create("icon.png")
	if err != nil {
		log.Fatal(err)
	}
	defer file.Close()

	if err := png.Encode(file, icon); err != nil {
		log.Fatal(err)
	}
}
