package main

import (
	"bufio"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"
)

type Entry struct {
	Type  string
	Path  string
	Mode  fs.FileMode
	User  int
	Group int
	Age   string
	Args  string
}

func Open(path string) ([]Entry, error) {
	file, err := os.OpenFile(path, os.O_RDONLY, 0)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var entries []Entry
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || line[0] == '#' {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		entry := Entry{
			Type: fields[0],
			Path: fields[1],
		}

		if len(fields) > 2 {
			mode, err := strconv.ParseInt(fields[2], 8, 32)
			if err == nil {
				entry.Mode = fs.FileMode(mode)
			}
		}

		if len(fields) > 3 && fields[3] != "-" {
			uid, err := strconv.Atoi(fields[3])
			if err != nil {
				u, err := user.Lookup(fields[3])
				if err == nil {
					uid, _ = strconv.Atoi(u.Uid)
				}
			}
			entry.User = uid
		}

		if len(fields) > 4 && fields[4] != "-" {
			gid, err := strconv.Atoi(fields[4])
			if err != nil {
				g, err := user.LookupGroup(fields[4])
				if err == nil {
					gid, _ = strconv.Atoi(g.Gid)
				}
			}
			entry.Group = gid
		}

		if len(fields) > 5 && fields[5] != "-" {
			entry.Age = fields[5]
		}

		if len(fields) > 6 && fields[6] != "-" {
			entry.Args = strings.Join(fields[6:], " ")
		}

		entries = append(entries, entry)
	}

	_ = scanner.Err()

	return entries, nil
}

func OpenPath(path string) ([]Entry, error) {
	var entries []Entry
	files, err := os.ReadDir(path + "/tmpfiles.d")
	if err == nil {
		for _, file := range files {
			if file.IsDir() || filepath.Ext(file.Name()) != ".conf" {
				continue
			}
			subentries, err := Open(filepath.Join(path, "tmpfiles.d", file.Name()))
			if err == nil {
				entries = append(entries, subentries...)
			}
		}
	}

	subentries, err := Open(filepath.Join(path, "tmpfiles.conf"))
	if err == nil {
		entries = append(entries, subentries...)
	}

	return entries, nil
}

func setupTmpfiles() {
	var entries []Entry
	for _, path := range []string{"/run", "/etc", "/usr/lib", "/usr/share"} {
		subentries, err := OpenPath(path)
		if err == nil {
			entries = append(entries, subentries...)
		}
	}

	for _, entry := range entries {
		switch entry.Type {
		case "C":
			copy(filepath.Join("/usr/share/factory", entry.Path), entry.Path, false)
		case "L", "L+":
			if entry.Args == "" {
				continue
			}
			if _, err := os.Stat(entry.Path); err == nil && entry.Type != "L+" {
				continue
			}
			os.Symlink(entry.Args, entry.Path)
		}
	}
}

func copy(src, dest string, overwrite bool) error {
	info, err := os.Lstat(src)
	if err != nil {
		return fmt.Errorf("stat source: %w", err)
	}

	if info.Mode()&os.ModeSymlink != 0 {
		return copySymlink(src, dest, overwrite)
	}

	if info.IsDir() {
		return copyDir(src, dest, overwrite)
	}

	return copyFile(src, dest, info.Mode(), overwrite)
}

func copyDir(src, dest string, overwrite bool) error {
	srcInfo, err := os.Stat(src)
	if err != nil {
		return fmt.Errorf("stat source dir: %w", err)
	}

	if !srcInfo.IsDir() {
		return fmt.Errorf("source is not a directory: %s", src)
	}

	if err := os.MkdirAll(dest, srcInfo.Mode()); err != nil {
		return fmt.Errorf("create destination dir: %w", err)
	}

	return filepath.WalkDir(src, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}

		if rel == "." {
			return nil
		}

		target := filepath.Join(dest, rel)

		info, err := os.Lstat(path)
		if err != nil {
			return err
		}

		if info.Mode()&os.ModeSymlink != 0 {
			return copySymlink(path, target, overwrite)
		}

		if d.IsDir() {
			return os.MkdirAll(target, info.Mode())
		}

		if d.Type().IsRegular() {
			return copyFile(path, target, info.Mode(), overwrite)
		}

		return nil
	})
}

func copyFile(src, dest string, mode os.FileMode, overwrite bool) error {
	if existing, err := os.Stat(dest); err == nil {
		if existing.IsDir() {
			dest = filepath.Join(dest, filepath.Base(src))
		} else if !overwrite {
			return fmt.Errorf("destination already exists: %s", dest)
		}
	}

	if err := os.MkdirAll(filepath.Dir(dest), 0755); err != nil {
		return fmt.Errorf("create parent directory: %w", err)
	}

	in, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("open source file: %w", err)
	}
	defer in.Close()

	out, err := os.OpenFile(dest, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode)
	if err != nil {
		return fmt.Errorf("open destination file: %w", err)
	}

	_, copyErr := io.Copy(out, in)
	closeErr := out.Close()

	if copyErr != nil {
		return fmt.Errorf("copy file data: %w", copyErr)
	}

	if closeErr != nil {
		return fmt.Errorf("close destination file: %w", closeErr)
	}

	return os.Chmod(dest, mode)
}

func copySymlink(src, dest string, overwrite bool) error {
	linkTarget, err := os.Readlink(src)
	if err != nil {
		return fmt.Errorf("read symlink: %w", err)
	}

	if _, err := os.Lstat(dest); err == nil {
		if !overwrite {
			return fmt.Errorf("destination already exists: %s", dest)
		}

		if err := os.RemoveAll(dest); err != nil {
			return fmt.Errorf("remove existing destination: %w", err)
		}
	}

	if err := os.MkdirAll(filepath.Dir(dest), 0755); err != nil {
		return fmt.Errorf("create parent directory: %w", err)
	}

	if err := os.Symlink(linkTarget, dest); err != nil {
		return fmt.Errorf("create symlink: %w", err)
	}

	return nil
}
