package main

import (
	"os"
	"path/filepath"
)

type fileInfo struct {
	Name  string
	IsDir bool
}

type treeEntry struct {
	Info     *fileInfo
	Children []*treeEntry
	Depth    int
}

func buildTreeRoot(path string, maxDepth int) (*treeEntry, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	entry := &treeEntry{
		Info:  &fileInfo{Name: info.Name(), IsDir: info.IsDir()},
		Depth: 0,
	}
	if info.IsDir() && maxDepth != 0 {
		if err := buildTree(entry, path, 1, maxDepth); err != nil {
			return nil, err
		}
	}
	return entry, nil
}

func buildTree(parent *treeEntry, path string, depth, maxDepth int) error {
	if maxDepth > 0 && depth > maxDepth {
		return nil
	}
	entries, err := os.ReadDir(path)
	if err != nil {
		return nil
	}
	for _, ent := range entries {
		fullPath := filepath.Join(path, ent.Name())
		info, err := os.Lstat(fullPath)
		if err != nil {
			continue
		}
		child := &treeEntry{
			Info:  &fileInfo{Name: info.Name(), IsDir: info.IsDir()},
			Depth: depth,
		}
		if info.IsDir() {
			_ = buildTree(child, fullPath, depth+1, maxDepth)
		}
		parent.Children = append(parent.Children, child)
	}
	return nil
}
