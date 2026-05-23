package main

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"avyos.dev/lib/graphics/collections"
	"avyos.dev/lib/graphics/widget"
)

type FilesApp struct{}

func (FilesApp) CreateState() widget.State { return &FilesState{} }

type FilesState struct {
	widget.StateBase

	currentPath string
	pathInput   string
	status      string
	loadErr     string
	entries     []fileEntry
	selected    string

	history      []string
	historyIndex int
	navigating   bool
}

type fileEntry struct {
	Name    string
	Path    string
	IsDir   bool
	Size    int64
	ModTime time.Time
}

type locationItem struct {
	Label string
	Icon  string
	Path  string
}

func (s *FilesState) InitState() {
	start := startDirectory()
	s.currentPath = start
	s.pathInput = start
	s.history = []string{start}
	s.historyIndex = 0
	s.loadDirectory(start)
}

func (s *FilesState) loadDirectory(path string) {
	path = filepath.Clean(path)
	entries, err := os.ReadDir(path)
	if err != nil {
		s.SetState(func() {
			s.loadErr = err.Error()
			s.status = "Unable to open location"
		})
		return
	}

	list := make([]fileEntry, 0, len(entries))
	for _, entry := range entries {
		info, err := entry.Info()
		if err != nil {
			continue
		}
		list = append(list, fileEntry{
			Name:    entry.Name(),
			Path:    filepath.Join(path, entry.Name()),
			IsDir:   entry.IsDir(),
			Size:    info.Size(),
			ModTime: info.ModTime(),
		})
	}

	slices.SortFunc(list, func(a, b fileEntry) int {
		if a.IsDir != b.IsDir {
			if a.IsDir {
				return -1
			}
			return 1
		}
		return strings.Compare(strings.ToLower(a.Name), strings.ToLower(b.Name))
	})

	s.SetState(func() {
		s.currentPath = path
		s.pathInput = path
		s.entries = list
		s.selected = ""
		s.loadErr = ""
		s.status = fmt.Sprintf("%d items", len(list))
	})

	if s.navigating {
		return
	}
	if s.historyIndex >= 0 && s.historyIndex < len(s.history) && s.history[s.historyIndex] == path {
		return
	}
	if s.historyIndex < len(s.history)-1 {
		s.history = append([]string{}, s.history[:s.historyIndex+1]...)
	}
	s.history = append(s.history, path)
	s.historyIndex = len(s.history) - 1
}

func (s *FilesState) goToPath(path string) {
	if strings.TrimSpace(path) == "" {
		return
	}
	s.loadDirectory(path)
}

func (s *FilesState) goBack() {
	if s.historyIndex <= 0 {
		return
	}
	s.historyIndex--
	s.navigating = true
	s.loadDirectory(s.history[s.historyIndex])
	s.navigating = false
}

func (s *FilesState) goForward() {
	if s.historyIndex >= len(s.history)-1 {
		return
	}
	s.historyIndex++
	s.navigating = true
	s.loadDirectory(s.history[s.historyIndex])
	s.navigating = false
}

func (s *FilesState) openEntry(entry fileEntry) {
	s.SetState(func() { s.selected = entry.Path })
	if entry.IsDir {
		s.loadDirectory(entry.Path)
	}
}

func locations() []locationItem {
	home := homeDir()
	return []locationItem{
		{Label: "System", Icon: "default-folder-system", Path: "/"},
		{Label: "Home", Icon: "default-user-home", Path: home},
		{Label: "Documents", Icon: "default-folder-documents", Path: joinIfHome(home, "Documents")},
		{Label: "Downloads", Icon: "default-folder-download", Path: joinIfHome(home, "Downloads")},
		{Label: "Desktop", Icon: "default-user-desktop", Path: joinIfHome(home, "Desktop")},
		{Label: "Pictures", Icon: "default-folder-pictures", Path: joinIfHome(home, "Pictures")},
		{Label: "Videos", Icon: "default-folder-video", Path: joinIfHome(home, "Videos")},
	}
}

func locationDestinations() []collections.NavDestination {
	items := locations()
	out := make([]collections.NavDestination, 0, len(items))
	for _, item := range items {
		out = append(out, collections.NavDestination{
			Label: item.Label,
			Icon:  item.Icon,
		})
	}
	return out
}

func (s *FilesState) currentLocationIndex() int {
	current := filepath.Clean(s.currentPath)
	best := -1
	bestLen := -1
	for i, loc := range locations() {
		locPath := filepath.Clean(loc.Path)
		if locPath == "" {
			continue
		}
		if current == locPath || (locPath != "/" && strings.HasPrefix(current, locPath+string(os.PathSeparator))) {
			if len(locPath) > bestLen {
				best = i
				bestLen = len(locPath)
			}
		}
	}
	if best >= 0 {
		return best
	}
	return 0
}

func (s *FilesState) selectLocation(i int) {
	items := locations()
	if i < 0 || i >= len(items) {
		return
	}
	s.goToPath(items[i].Path)
}

func joinIfHome(home, child string) string {
	if home == "" {
		return "/"
	}
	return filepath.Join(home, child)
}

func startDirectory() string {
	if home := homeDir(); home != "" {
		return home
	}
	return "/"
}

func homeDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return home
}

func humanSize(size int64) string {
	if size < 1024 {
		return fmt.Sprintf("%d B", size)
	}
	units := []string{"KB", "MB", "GB", "TB"}
	value := float64(size)
	for _, unit := range units {
		value /= 1024
		if value < 1024 {
			return fmt.Sprintf("%.1f %s", value, unit)
		}
	}
	return fmt.Sprintf("%.1f PB", value/1024)
}
