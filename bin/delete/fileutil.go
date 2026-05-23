package main

import (
	"fmt"
	"os"
)

func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func removePath(path string, recursive bool) error {
	info, err := os.Lstat(path)
	if err != nil {
		return fmt.Errorf("cannot access %s: %w", path, err)
	}
	if info.IsDir() && !recursive {
		entries, err := os.ReadDir(path)
		if err != nil {
			return err
		}
		if len(entries) > 0 {
			return fmt.Errorf("%s is not empty (use --recursive)", path)
		}
		return os.Remove(path)
	}
	return os.RemoveAll(path)
}
