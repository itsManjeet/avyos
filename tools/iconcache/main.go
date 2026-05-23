package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"strings"

	"avyos.dev/pkg/graphics/icons"
)

func main() {
	flag.Parse()
	if flag.NArg() == 0 {
		fmt.Printf("Usage: %s <path>\n", os.Args[0])
		return
	}
	themePath := flag.Arg(0)

	cache, err := buildCache(themePath)
	if err != nil {
		log.Fatal(err)
	}

	out, err := json.Marshal(cache)
	if err != nil {
		log.Fatal(err)
	}

	if err := os.WriteFile(filepath.Join(themePath, ".cache.json"), out, 0644); err != nil {
		log.Fatal(err)
	}
}

func buildCache(themePath string) (icons.Cache, error) {
	cache := icons.Cache{
		Theme: filepath.Base(themePath),
		Icons: make(map[string]string),
	}

	err := filepath.Walk(themePath, func(path string, info fs.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return err
		}
		ext := strings.ToLower(filepath.Ext(info.Name()))
		if ext != ".svg" {
			return nil
		}

		name := strings.TrimSuffix(info.Name(), ext)
		rel, err := filepath.Rel(themePath, path)
		if err != nil {
			return err
		}
		cache.Icons[name] = rel
		return nil
	})
	if err != nil {
		return icons.Cache{}, err
	}
	return cache, nil
}
