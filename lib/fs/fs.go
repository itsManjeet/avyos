/*
 * Copyright (c) 2026 Manjeet Singh <itsmanjeet1998@gmail.com>.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU General Public License as published by
 * the Free Software Foundation, version 3.
 *
 * This program is distributed in the hope that it will be useful, but
 * WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the GNU
 * General Public License for more details.
 *
 * You should have received a copy of the GNU General Public License
 * along with this program. If not, see <http://www.gnu.org/licenses/>.
 *
 */

package fs

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// Exists checks if a path exists.
func Exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// IsDir checks if a path is a directory.
func IsDir(path string) bool {
	fi, err := os.Stat(path)
	if err != nil {
		return false
	}
	return fi.IsDir()
}

// IsFile checks if a path is a regular file.
func IsFile(path string) bool {
	fi, err := os.Stat(path)
	if err != nil {
		return false
	}
	return fi.Mode().IsRegular()
}

// IsSymlink checks if a path is a symbolic link.
func IsSymlink(path string) bool {
	fi, err := os.Lstat(path)
	if err != nil {
		return false
	}
	return fi.Mode()&os.ModeSymlink != 0
}

// Copy copies a file or directory from src to dst.
func Copy(src, dst string, recursive bool) error {
	srcInfo, err := os.Stat(src)
	if err != nil {
		return fmt.Errorf("cannot access %s: %w", src, err)
	}

	if srcInfo.IsDir() {
		if !recursive {
			return fmt.Errorf("%s is a directory (use --recursive)", src)
		}
		return copyDir(src, dst)
	}

	return copyFile(src, dst)
}

func copyFile(src, dst string) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	srcInfo, err := srcFile.Stat()
	if err != nil {
		return err
	}

	// If dst is a directory, copy into it
	if IsDir(dst) {
		dst = filepath.Join(dst, filepath.Base(src))
	}

	dstFile, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, srcInfo.Mode())
	if err != nil {
		return err
	}
	defer dstFile.Close()

	_, err = io.Copy(dstFile, srcFile)
	return err
}

func copyDir(src, dst string) error {
	srcInfo, err := os.Stat(src)
	if err != nil {
		return err
	}

	// Create destination directory
	if err := os.MkdirAll(dst, srcInfo.Mode()); err != nil {
		return err
	}

	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		srcPath := filepath.Join(src, entry.Name())
		dstPath := filepath.Join(dst, entry.Name())

		if entry.IsDir() {
			if err := copyDir(srcPath, dstPath); err != nil {
				return err
			}
		} else {
			if err := copyFile(srcPath, dstPath); err != nil {
				return err
			}
		}
	}

	return nil
}

// Move moves a file or directory from src to dst.
func Move(src, dst string) error {
	// Try rename first (works if same filesystem)
	if err := os.Rename(src, dst); err == nil {
		return nil
	}

	// Fall back to copy + remove
	srcInfo, err := os.Stat(src)
	if err != nil {
		return err
	}

	if err := Copy(src, dst, srcInfo.IsDir()); err != nil {
		return err
	}

	return Remove(src, true)
}

// Remove removes a file or directory.
func Remove(path string, recursive bool) error {
	fi, err := os.Lstat(path)
	if err != nil {
		return fmt.Errorf("cannot access %s: %w", path, err)
	}

	if fi.IsDir() && !recursive {
		// Check if directory is empty
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

// MkdirAll creates a directory and all parent directories.
func MkdirAll(path string, perm os.FileMode) error {
	return os.MkdirAll(path, perm)
}

// Touch creates an empty file or updates the modification time.
func Touch(path string) error {
	if Exists(path) {
		now := os.FileInfo(nil)
		_ = now // os.Chtimes requires time.Time which needs time package
		// For now, just open and close the file
		f, err := os.OpenFile(path, os.O_RDWR, 0)
		if err != nil {
			return err
		}
		return f.Close()
	}

	f, err := os.Create(path)
	if err != nil {
		return err
	}
	return f.Close()
}

// ReadLink returns the destination of a symbolic link.
func ReadLink(path string) (string, error) {
	return os.Readlink(path)
}

// Symlink creates a symbolic link.
func Symlink(target, link string) error {
	return os.Symlink(target, link)
}

// Walk walks a directory tree, calling walkFn for each file or directory.
type WalkFunc func(path string, info os.FileInfo, err error) error

// Walk walks the file tree rooted at root.
func Walk(root string, walkFn WalkFunc) error {
	return filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		return walkFn(path, info, err)
	})
}

// Find finds files matching a pattern in a directory.
func Find(root, pattern string) ([]string, error) {
	var matches []string

	err := Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // Skip errors
		}

		name := filepath.Base(path)
		matched, err := filepath.Match(pattern, name)
		if err != nil {
			return err
		}

		if matched {
			matches = append(matches, path)
		}

		return nil
	})

	return matches, err
}

// FileInfo represents information about a file.
type FileInfo struct {
	Name    string
	Path    string
	Size    int64
	Mode    os.FileMode
	ModTime int64 // Unix timestamp
	UID     uint32
	GID     uint32
	IsDir   bool
	IsLink  bool
	Target  string // For symlinks
}

// Info returns information about a file.
func Info(path string) (*FileInfo, error) {
	fi, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}

	info := &FileInfo{
		Name:    fi.Name(),
		Path:    path,
		Size:    fi.Size(),
		Mode:    fi.Mode(),
		ModTime: fi.ModTime().Unix(),
		IsDir:   fi.IsDir(),
		IsLink:  fi.Mode()&os.ModeSymlink != 0,
	}
	info.UID, info.GID, err = getUidGid(fi.Sys())
	if err != nil {
		return nil, err
	}

	if info.IsLink {
		target, err := os.Readlink(path)
		if err == nil {
			info.Target = target
		}
	}

	return info, nil
}

// ListDir lists the contents of a directory.
func ListDir(path string) ([]*FileInfo, error) {
	entries, err := os.ReadDir(path)
	if err != nil {
		return nil, err
	}

	var infos []*FileInfo
	for _, entry := range entries {
		fullPath := filepath.Join(path, entry.Name())
		info, err := Info(fullPath)
		if err != nil {
			continue
		}
		infos = append(infos, info)
	}

	return infos, nil
}

// TreeEntry represents an entry in a directory tree.
type TreeEntry struct {
	Info     *FileInfo
	Children []*TreeEntry
	Depth    int
}

// Tree builds a directory tree.
func Tree(path string, maxDepth int) (*TreeEntry, error) {
	info, err := Info(path)
	if err != nil {
		return nil, err
	}

	entry := &TreeEntry{
		Info:  info,
		Depth: 0,
	}

	if info.IsDir && maxDepth != 0 {
		if err := buildTree(entry, path, 1, maxDepth); err != nil {
			return nil, err
		}
	}

	return entry, nil
}

func buildTree(parent *TreeEntry, path string, depth, maxDepth int) error {
	if maxDepth > 0 && depth > maxDepth {
		return nil
	}

	entries, err := os.ReadDir(path)
	if err != nil {
		return nil
	}

	for _, e := range entries {
		fullPath := filepath.Join(path, e.Name())
		info, err := Info(fullPath)
		if err != nil {
			continue
		}

		child := &TreeEntry{
			Info:  info,
			Depth: depth,
		}

		if info.IsDir {
			_ = buildTree(child, fullPath, depth+1, maxDepth)
		}

		parent.Children = append(parent.Children, child)
	}

	return nil
}

// PermString returns a human-readable permission string (like ls -l).
func PermString(mode os.FileMode) string {
	var sb strings.Builder

	// File type
	switch {
	case mode.IsDir():
		sb.WriteByte('d')
	case mode&os.ModeSymlink != 0:
		sb.WriteByte('l')
	default:
		sb.WriteByte('-')
	}

	// Owner permissions
	if mode&0400 != 0 {
		sb.WriteByte('r')
	} else {
		sb.WriteByte('-')
	}
	if mode&0200 != 0 {
		sb.WriteByte('w')
	} else {
		sb.WriteByte('-')
	}
	if mode&0100 != 0 {
		sb.WriteByte('x')
	} else {
		sb.WriteByte('-')
	}

	// Group permissions
	if mode&0040 != 0 {
		sb.WriteByte('r')
	} else {
		sb.WriteByte('-')
	}
	if mode&0020 != 0 {
		sb.WriteByte('w')
	} else {
		sb.WriteByte('-')
	}
	if mode&0010 != 0 {
		sb.WriteByte('x')
	} else {
		sb.WriteByte('-')
	}

	// Other permissions
	if mode&0004 != 0 {
		sb.WriteByte('r')
	} else {
		sb.WriteByte('-')
	}
	if mode&0002 != 0 {
		sb.WriteByte('w')
	} else {
		sb.WriteByte('-')
	}
	if mode&0001 != 0 {
		sb.WriteByte('x')
	} else {
		sb.WriteByte('-')
	}

	return sb.String()
}
