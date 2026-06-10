// Package settings watches config files and reports parsed changes.
package settings

import (
	"crypto/sha256"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"avyos.dev/lib/config"
)

// Change describes one changed config file.
type Change struct {
	Path    string
	Data    map[string]any
	Deleted bool
	Err     error
}

type fileState struct {
	sum    [32]byte
	errSig string
}

type watcher struct {
	callback func(changes ...Change)
	paths    []string
	stop     chan struct{}
	done     chan struct{}
	state    map[string]fileState
}

var (
	watchPollInterval = 500 * time.Millisecond
	homeConfigRoot    = func() string {
		home := strings.TrimSpace(os.Getenv("HOME"))
		if home == "" {
			return ""
		}
		return filepath.Join(home, ".config")
	}
	systemConfigRoot = func() string { return "/config" }
)

// OnChange registers a callback that is called whenever any watched config file changes.
//
// If no paths are provided, it watches all `.conf` files under `$HOME/.config` and `/config`.
// Each callback batch contains the files that changed during one poll cycle.
// The returned stop function blocks until the watcher goroutine exits.
func OnChange(fn func(changes ...Change), paths ...string) (func(), error) {
	if fn == nil {
		return nil, errors.New("settings: callback is required")
	}

	normalized, err := normalizeWatchPaths(paths)
	if err != nil {
		return nil, err
	}

	w := &watcher{
		callback: fn,
		paths:    normalized,
		stop:     make(chan struct{}),
		done:     make(chan struct{}),
		state:    make(map[string]fileState),
	}
	w.state = w.scanStates()

	go w.run()
	var once sync.Once
	return func() {
		once.Do(func() {
			close(w.stop)
			<-w.done
		})
	}, nil
}

func (w *watcher) run() {
	defer close(w.done)

	ticker := time.NewTicker(watchPollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-w.stop:
			return
		case <-ticker.C:
			changes := w.scanChanges()
			if len(changes) != 0 {
				w.callback(changes...)
			}
		}
	}
}

func (w *watcher) scanChanges() []Change {
	current := w.scanStates()
	changes := make([]Change, 0)

	for _, path := range sortedStateKeys(current) {
		next := current[path]
		prev, ok := w.state[path]
		if ok && prev.sum == next.sum && prev.errSig == next.errSig {
			continue
		}
		change := Change{Path: path}
		if next.errSig != "" {
			change.Err = errors.New(next.errSig)
		} else {
			cfg, err := config.ParseFile(path)
			if err != nil {
				change.Err = err
			} else {
				change.Data = cfg.Data()
			}
		}
		changes = append(changes, change)
	}

	for _, path := range sortedStateKeys(w.state) {
		if _, ok := current[path]; ok {
			continue
		}
		changes = append(changes, Change{
			Path:    path,
			Deleted: true,
		})
	}

	w.state = current
	sort.Slice(changes, func(i, j int) bool {
		return changes[i].Path < changes[j].Path
	})
	return changes
}

func (w *watcher) scanStates() map[string]fileState {
	states := make(map[string]fileState)
	for _, path := range discoverConfigFiles(w.paths) {
		data, err := os.ReadFile(path)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			states[path] = fileState{errSig: err.Error()}
			continue
		}
		states[path] = fileState{
			sum: sha256.Sum256(data),
		}
	}
	return states
}

func normalizeWatchPaths(paths []string) ([]string, error) {
	if len(paths) == 0 {
		return defaultWatchPaths(), nil
	}

	seen := make(map[string]struct{}, len(paths))
	out := make([]string, 0, len(paths))
	for _, path := range paths {
		path = filepath.Clean(strings.TrimSpace(path))
		if path == "." || path == "" {
			return nil, errors.New("settings: watch path is required")
		}
		if _, ok := seen[path]; ok {
			continue
		}
		seen[path] = struct{}{}
		out = append(out, path)
	}
	sort.Strings(out)
	return out, nil
}

func defaultWatchPaths() []string {
	paths := make([]string, 0, 2)
	seen := make(map[string]struct{}, 2)
	for _, path := range []string{homeConfigRoot(), systemConfigRoot()} {
		path = filepath.Clean(strings.TrimSpace(path))
		if path == "." || path == "" {
			continue
		}
		if _, ok := seen[path]; ok {
			continue
		}
		seen[path] = struct{}{}
		paths = append(paths, path)
	}
	sort.Strings(paths)
	return paths
}

func discoverConfigFiles(paths []string) []string {
	files := make([]string, 0, len(paths))
	seen := make(map[string]struct{})
	for _, path := range paths {
		info, err := os.Stat(path)
		if err != nil {
			if os.IsNotExist(err) {
				if isConfigFilePath(path) {
					addConfigFile(&files, seen, path)
				}
			}
			continue
		}
		if info.IsDir() {
			filepath.WalkDir(path, func(candidate string, d os.DirEntry, err error) error {
				if err != nil || d == nil || d.IsDir() {
					return nil
				}
				if isConfigFilePath(candidate) {
					addConfigFile(&files, seen, candidate)
				}
				return nil
			})
			continue
		}
		if isConfigFilePath(path) {
			addConfigFile(&files, seen, path)
		}
	}
	sort.Strings(files)
	return files
}

func addConfigFile(files *[]string, seen map[string]struct{}, path string) {
	path = filepath.Clean(path)
	if _, ok := seen[path]; ok {
		return
	}
	seen[path] = struct{}{}
	*files = append(*files, path)
}

func isConfigFilePath(path string) bool {
	return strings.EqualFold(filepath.Ext(path), ".conf")
}

func sortedStateKeys(m map[string]fileState) []string {
	keys := make([]string, 0, len(m))
	for key := range m {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
