// Copyright (c) 2026 Manjeet Singh <itsmanjeet1998@gmail.com>.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, version 3.

// Package icons loads themed icons from the data filesystem.
package icons

import (
	"encoding/json"
	"errors"
	"image"
	"image/gif"
	"image/jpeg"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"avyos.dev/lib/graphics/svg"
)

var (
	Theme = "default"

	cache     Cache
	cacheOnce sync.Once
	cacheErr  error
)

func Load(name string, size int) (image.Image, error) {
	cacheOnce.Do(func() {
		f, err := os.Open(resolveThemeFile(Theme, ".cache.json"))
		if err != nil {
			cacheErr = err
			return
		}
		defer f.Close()

		cacheErr = json.NewDecoder(f).Decode(&cache)
	})
	if cacheErr != nil {
		return nil, cacheErr
	}

	iconPath := getIconPath(name)
	if iconPath == "" {
		return nil, errors.New("not found")
	}
	iconPath = resolveThemeFile(Theme, iconPath)

	switch strings.ToLower(filepath.Ext(iconPath)) {
	case ".svg":
		if size > 0 {
			return svg.DecodeSizedFile(iconPath, size, size)
		}
		return svg.DecodeFile(iconPath)
	}

	f, err := os.Open(iconPath)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	switch strings.ToLower(filepath.Ext(iconPath)) {
	case ".png":
		return png.Decode(f)
	case ".jpg":
		return jpeg.Decode(f)
	case ".gif":
		return gif.Decode(f)
	default:
		return nil, nil
	}
}

func getIconPath(name string) string {
	return cache.Icons[name]
}

func resolveThemeFile(theme, rel string) string {
	resolved := filepath.Join("/usr/share/icons", theme, rel)
	if fileExists(resolved) {
		return resolved
	}

	if local, ok := resolveRepoThemeFile(theme, rel); ok {
		return local
	}

	return resolved
}

func resolveRepoThemeFile(theme, rel string) (string, bool) {
	dir, err := os.Getwd()
	if err != nil {
		return "", false
	}

	for {
		candidate := filepath.Join(dir, "data", "icons", theme, rel)
		if fileExists(candidate) {
			return candidate, true
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			return "", false
		}
		dir = parent
	}
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
