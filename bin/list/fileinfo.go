package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"
)

type fileInfo struct {
	Name    string
	Path    string
	Size    int64
	Mode    os.FileMode
	ModTime int64
	UID     uint32
	GID     uint32
	IsDir   bool
	IsLink  bool
	Target  string
}

func infoFor(path string) (*fileInfo, error) {
	fi, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	info := &fileInfo{
		Name:    fi.Name(),
		Path:    path,
		Size:    fi.Size(),
		Mode:    fi.Mode(),
		ModTime: fi.ModTime().Unix(),
		IsDir:   fi.IsDir(),
		IsLink:  fi.Mode()&os.ModeSymlink != 0,
	}
	st, ok := fi.Sys().(*syscall.Stat_t)
	if !ok || st == nil {
		return nil, fmt.Errorf("invalid stat info")
	}
	info.UID = uint32(st.Uid)
	info.GID = uint32(st.Gid)
	if info.IsLink {
		if target, err := os.Readlink(path); err == nil {
			info.Target = target
		}
	}
	return info, nil
}

func listDir(path string) ([]*fileInfo, error) {
	entries, err := os.ReadDir(path)
	if err != nil {
		return nil, err
	}
	infos := make([]*fileInfo, 0, len(entries))
	for _, entry := range entries {
		info, err := infoFor(filepath.Join(path, entry.Name()))
		if err == nil {
			infos = append(infos, info)
		}
	}
	return infos, nil
}

func permString(mode os.FileMode) string {
	var sb strings.Builder
	if mode.IsDir() {
		sb.WriteByte('d')
	} else if mode&os.ModeSymlink != 0 {
		sb.WriteByte('l')
	} else {
		sb.WriteByte('-')
	}
	for _, bit := range []os.FileMode{0400, 0200, 0100, 0040, 0020, 0010, 0004, 0002, 0001} {
		ch := byte('-')
		switch bit {
		case 0400, 0040, 0004:
			ch = 'r'
		case 0200, 0020, 0002:
			ch = 'w'
		case 0100, 0010, 0001:
			ch = 'x'
		}
		if mode&bit == 0 {
			ch = '-'
		}
		sb.WriteByte(ch)
	}
	return sb.String()
}
