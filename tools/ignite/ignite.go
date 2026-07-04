package main

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"io/fs"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"

	"gopkg.in/yaml.v3"
)

type Compiler struct {
	file   string
	script string
}

type State struct {
	id     string
	recipe Recipe
	cached bool
}

type ContainerType int

const (
	ContainerBuild ContainerType = iota
	ContainerShell
)

type Ignite struct {
	config        *Config
	projectPath   string
	cachePath     string
	workspacePath string
	pool          map[string]Recipe
	compilers     map[string]Compiler
	hashCache     map[string]string
	hashMu        sync.Mutex
	mirrors       map[string]string
}

func NewIgnite(config *Config, projectPath, cachePath, workspacePath, arch string) (*Ignite, error) {
	file := filepath.Join(projectPath, "config-"+arch+".yml")
	if _, err := os.Stat(file); err != nil {
		return nil, fmt.Errorf("failed to load configuration file %q", file)
	}
	if err := config.UpdateFromFile(file); err != nil {
		return nil, err
	}
	if workspacePath == "" {
		workspacePath = filepath.Join(cachePath, "workspaces")
	} else if !filepath.IsAbs(workspacePath) {
		workspacePath = filepath.Join(projectPath, workspacePath)
	}
	i := &Ignite{
		config: config, projectPath: projectPath, cachePath: cachePath, workspacePath: workspacePath,
		pool: map[string]Recipe{}, compilers: map[string]Compiler{}, hashCache: map[string]string{},
	}
	if node := config.Node("compiler"); node != nil && node.Kind == yaml.MappingNode {
		for idx := 0; idx+1 < len(node.Content); idx += 2 {
			body := node.Content[idx+1]
			i.compilers[node.Content[idx].Value] = Compiler{
				file:   scalarString(mapValue(body, "file")),
				script: scalarString(mapValue(body, "script")),
			}
		}
	}
	if node := config.Node("mirrors"); node != nil && node.Kind == yaml.MappingNode {
		i.mirrors = map[string]string{}
		for idx := 0; idx+1 < len(node.Content); idx += 2 {
			i.mirrors[node.Content[idx].Value] = node.Content[idx+1].Value
		}
	}
	return i, nil
}

func (i *Ignite) Load() error {
	root := filepath.Join(i.projectPath, "elements")
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() || filepath.Ext(path) != ".yml" {
			return nil
		}
		rel, _ := filepath.Rel(root, path)
		rel = filepath.ToSlash(rel)
		recipe, err := LoadRecipe(path, i.projectPath)
		if err != nil {
			return fmt.Errorf("failed to load %q because %w", rel, err)
		}
		i.pool[rel] = recipe
		return nil
	})
	if err != nil {
		return err
	}
	fmt.Printf("Ignite::load(): Loaded %d elements\n", len(i.pool))
	return nil
}

func (i *Ignite) Resolve(ids []string, devel, includeDepends, includeExtra bool) ([]State, error) {
	var output []State
	visited := map[string]bool{}
	var dfs func(string) error
	dfs = func(id string) error {
		visited[id] = true
		recipe, ok := i.pool[id]
		if !ok {
			return fmt.Errorf("MISSING %s", id)
		}
		depends := append([]string{}, recipe.depends...)
		if devel {
			depends = append(depends, recipe.buildTimeDepends...)
		}
		if includeExtra {
			depends = append(depends, recipe.config.StringSlice("include")...)
		}
		if includeDepends {
			for _, dep := range depends {
				if visited[dep] {
					continue
				}
				if err := dfs(dep); err != nil {
					return fmt.Errorf("%w\n\tTRACEBACK %s", err, id)
				}
			}
		}
		resolved := recipe
		hash, err := i.Hash(resolved)
		if err != nil {
			return err
		}
		resolved.cache = hash
		cached := !i.WorkspaceAvailable(resolved) && exists(i.CacheFile(resolved))
		for _, dep := range depends {
			found := false
			for _, state := range output {
				if state.id == dep {
					found = true
					if !state.cached {
						cached = false
					}
					break
				}
			}
			if found {
				continue
			}
			local, ok := i.pool[dep]
			if !ok {
				return fmt.Errorf("internal error %s not in a pool for %s", dep, id)
			}
			hash, err := i.Hash(local)
			if err != nil {
				return err
			}
			local.cache = hash
			if i.WorkspaceAvailable(local) || !exists(i.CacheFile(local)) {
				cached = false
				break
			}
		}
		output = append(output, State{id, resolved, cached})
		return nil
	}
	for _, id := range ids {
		if err := dfs(id); err != nil {
			return nil, err
		}
	}
	return output, nil
}

func (i *Ignite) Hash(recipe Recipe) (string, error) {
	key := recipe.elementID
	if key == "" {
		key = recipe.id
	}
	i.hashMu.Lock()
	if value, ok := i.hashCache[key]; ok {
		i.hashMu.Unlock()
		return value, nil
	}
	i.hashMu.Unlock()
	data, err := yaml.Marshal(recipe.config.node)
	if err != nil {
		return "", err
	}
	sum := sha256Hex(data)
	includes := recipe.config.StringSlice("include")
	for _, deps := range [][]string{recipe.depends, recipe.buildTimeDepends, includes} {
		for _, dep := range deps {
			depRecipe, ok := i.pool[dep]
			if !ok {
				return "", fmt.Errorf("missing required element %q for %s", dep, recipe.id)
			}
			depHash, err := i.Hash(depRecipe)
			if err != nil {
				return "", err
			}
			sum = sha256Hex([]byte(depHash + sum))
		}
	}
	i.hashMu.Lock()
	i.hashCache[key] = sum
	i.hashMu.Unlock()
	return sum, nil
}

func (i *Ignite) CacheFile(recipe Recipe) string {
	if i.WorkspaceAvailable(recipe) {
		return i.WorkspaceCacheFile(recipe)
	}
	return filepath.Join(i.cachePath, "cache", recipe.PackageName(recipe.elementID))
}

func (i *Ignite) WorkspacePath(recipe Recipe) string {
	return filepath.Join(i.workspacePath, workspaceComponentID(recipe))
}

func (i *Ignite) WorkspaceAvailable(recipe Recipe) bool {
	return isDir(i.WorkspacePath(recipe)) && exists(filepath.Join(i.WorkspacePath(recipe), ".ignite-workspace", "metadata"))
}

func (i *Ignite) WorkspaceCacheFile(recipe Recipe) string {
	return filepath.Join(i.cachePath, "cache", workspacePackageName(recipe))
}

func (i *Ignite) SetupContainer(recipe Recipe, typ ContainerType) (Container, error) {
	env := []string{"NOCONFIGURE=1", "HOME=/", "SHELL=/bin/sh", "TERM=dumb", "USER=nishu", "LOGNAME=nishu", "LC_ALL=C", "TZ=UTC", "SOURCE_DATE_EPOCH=918239400", "PKGSYSTEM_ENABLE_FSYNC=0"}
	env = append(env, i.config.StringSlice("environ")...)
	env = append(env, recipe.config.StringSlice("environ")...)
	ccache := i.config.Bool("ccache", true)
	if recipe.config.Has("ccache") {
		ccache = recipe.config.Bool("ccache", true)
	}
	hostRoot := filepath.Join(i.cachePath, "temp", recipe.PackageName(recipe.elementID))
	if !removeTree(hostRoot) {
		return Container{}, fmt.Errorf("failed to clean stale build root %q", hostRoot)
	}
	if err := os.MkdirAll(hostRoot, 0755); err != nil {
		return Container{}, err
	}
	binds := [][2]string{
		{"/sources", filepath.Join(i.cachePath, "sources")},
		{"/cache", filepath.Join(i.cachePath, "cache")},
		{"/files", filepath.Join(i.projectPath, "files")},
		{"/patches", filepath.Join(i.projectPath, "patches")},
		{"/avyos", i.projectPath},
	}
	if ccache {
		env = enableCCacheEnvironment(env)
		binds = append(binds, [2]string{"/ccache", filepath.Join(i.cachePath, "ccache")})
	}
	c := Container{
		environ:      env,
		binds:        binds,
		capabilities: recipe.config.StringSlice("capabilities"),
		hostRoot:     hostRoot,
		baseDir:      i.projectPath,
		name:         recipe.PackageName(recipe.elementID),
	}
	dirs := []string{"sources", "cache"}
	if ccache {
		dirs = append(dirs, "ccache")
	}
	for _, dir := range dirs {
		_ = os.MkdirAll(filepath.Join(i.cachePath, dir), 0755)
	}
	i.config.SetString("dir.build", hostRoot)
	_ = os.MkdirAll(filepath.Join(hostRoot, "usr", "local", "include"), 0755)
	if ccache {
		if err := i.IntegrateCachedTool(&c, "components/ccache.yml"); err != nil {
			return c, err
		}
	}

	depends := append([]string{}, recipe.depends...)
	if typ == ContainerBuild {
		depends = append(depends, recipe.buildTimeDepends...)
	}
	states, err := i.Resolve(depends, true, true, false)
	if err != nil {
		return c, err
	}
	for _, state := range states {
		if err := i.Integrate(&c, state.recipe, ""); err != nil {
			return c, err
		}
	}
	if typ == ContainerShell {
		if err := i.Integrate(&c, recipe, ""); err != nil {
			return c, err
		}
	}
	includes := recipe.config.StringSlice("include")
	if len(includes) > 0 {
		resolved := make([]string, 0, len(includes))
		for _, inc := range includes {
			v, err := recipe.ResolveValue(inc, *i.config, nil)
			if err != nil {
				return c, err
			}
			resolved = append(resolved, v)
		}
		states, err = i.Resolve(resolved, false, recipe.config.Bool("include-depends", true), false)
		if err != nil {
			return c, err
		}
		if upon := scalarString(recipe.config.Node("include-upon")); upon != "" {
			sub, err := i.Resolve([]string{upon}, false, true, false)
			if err != nil {
				return c, err
			}
			drop := map[string]bool{}
			for _, s := range sub {
				drop[s.id] = true
			}
			filtered := states[:0]
			for _, s := range states {
				if !drop[s.id] {
					filtered = append(filtered, s)
				}
			}
			states = filtered
		}
		for _, state := range states {
			path := filepath.Join("install-root", recipe.PackageName(recipe.elementID))
			if v, _ := recipe.config.String(recipe.Name()+"-include-path", ""); v != "" {
				path = v
			} else if v, _ := recipe.config.String("include-root", ""); v != "" {
				path = v
			}
			if err := i.Integrate(&c, state.recipe, path); err != nil {
				return c, err
			}
		}
	}
	return c, nil
}

func (i *Ignite) IntegrateCachedTool(container *Container, id string) error {
	if _, ok := i.pool[id]; !ok {
		return nil
	}
	states, err := i.Resolve([]string{id}, false, true, false)
	if err != nil {
		return err
	}
	for _, state := range states {
		if !exists(i.CacheFile(state.recipe)) {
			return nil
		}
	}
	for _, state := range states {
		if err := i.Integrate(container, state.recipe, ""); err != nil {
			return err
		}
	}
	return nil
}

func (i *Ignite) Integrate(container *Container, recipe Recipe, root string) error {
	containerRoot := filepath.Join(container.hostRoot, strings.TrimPrefix(root, "/"))
	fmt.Printf("Ignite::integrate(%s)\n", recipe.PackageName())
	if err := os.MkdirAll(containerRoot, 0755); err != nil {
		return err
	}
	cacheFilePath := i.CacheFile(recipe)
	ex := NewExecutor("/bin/tar").Arg("-xPhf").Arg(cacheFilePath).Arg("-C").Arg(containerRoot)
	if root == "" {
		for _, e := range []string{"./etc/hosts", "./etc/hostname", "./etc/resolv.conf", "./proc", "./run", "./sys", "./dev"} {
			ex.Arg("--exclude=" + e)
		}
	}
	if err := ex.Execute(); err != nil {
		return fmt.Errorf("failed to integrate %s %w", recipe.PackageName(recipe.elementID), err)
	}
	if root == "" {
		if recipe.integration != "" {
			script, err := recipe.ResolveValue(recipe.integration, *i.config, nil)
			if err != nil {
				return err
			}
			return NewExecutor("/bin/sh").Arg("-ec").Arg(script).Container(container).Execute()
		}
		return nil
	}
	dataDir := filepath.Join(containerRoot, "usr", "share", "pkgupd", "manifest", recipe.PackageName())
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return err
	}
	fmt.Printf("Iginite::integrate::save_data(%s)@%s\n", recipe.PackageName(), recipe.PackageName())
	if err := os.WriteFile(filepath.Join(dataDir, "info"), []byte(recipe.String()), 0644); err != nil {
		return err
	}
	if recipe.integration != "" {
		script, err := recipe.ResolveValue(recipe.integration, *i.config, nil)
		if err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(dataDir, "integration"), []byte(script), 0644); err != nil {
			return err
		}
	}
	file, err := os.Create(filepath.Join(dataDir, "files"))
	if err != nil {
		return err
	}
	defer file.Close()
	ex = NewExecutor("/bin/tar").Arg("-tf").Arg(cacheFilePath)
	for _, e := range []string{"./etc/hosts", "./etc/hostname", "./etc/resolv.conf", "./proc", "./run", "./sys", "./dev"} {
		ex.Arg("--exclude=" + e)
	}
	status, out := ex.Output()
	_, _ = file.WriteString(out)
	if out != "" && !strings.HasSuffix(out, "\n") {
		_, _ = file.WriteString("\n")
	}
	if status != 0 {
		return fmt.Errorf("failed to read tar files from %s", cacheFilePath)
	}
	return nil
}

func (i *Ignite) Build(recipe Recipe) error {
	container, err := i.SetupContainer(recipe, ContainerBuild)
	if err != nil {
		return err
	}
	defer removeTree(container.hostRoot)
	_ = os.MkdirAll(filepath.Join(i.cachePath, "logs"), 0755)
	log, err := os.Create(filepath.Join(i.cachePath, "logs", recipe.PackageName(recipe.elementID)+".log"))
	if err != nil {
		return err
	}
	defer log.Close()
	container.logger = log
	packagePath := i.CacheFile(recipe)
	subdir := "."
	if i.WorkspaceAvailable(recipe) {
		fmt.Println("Ignite::build(): using workspace", i.WorkspacePath(recipe))
		if _, err := i.PrepareWorkspaceSources(recipe, filepath.Join(container.hostRoot, "build-root")); err != nil {
			return err
		}
	} else if s, err := i.PrepareSources(recipe, filepath.Join(i.cachePath, "sources"), filepath.Join(container.hostRoot, "build-root")); err != nil {
		return err
	} else if s != "" {
		subdir = s
	}
	buildRoot := filepath.Join("build-root", configString(recipe.config, "build-dir", subdir))
	buildRoot, err = recipe.ResolveValue(buildRoot, *i.config, nil)
	if err != nil {
		return err
	}
	if err := i.CompileSource(recipe, &container, buildRoot, "install-root"); err != nil {
		fmt.Println("ERROR:", err)
		_ = NewExecutor("/bin/sh").Container(&container).Interactive().Run()
		return err
	}
	return i.Pack(recipe, &container, filepath.Join(container.hostRoot, "install-root"), packagePath)
}

func (i *Ignite) findMirror(url string) string {
	for src, mirror := range i.mirrors {
		if strings.HasPrefix(url, src) {
			return strings.Replace(url, src, mirror, 1)
		}
	}
	return url
}

func (i *Ignite) FetchSourceFile(source, sourceDir string, force bool) error {
	spec, err := parseSourceSpec(source)
	if err != nil {
		return err
	}
	filePath := filepath.Join(sourceDir, spec.filename)
	tmp := filePath + ".tmp"
	_ = os.MkdirAll(sourceDir, 0755)
	if force {
		_ = os.RemoveAll(filePath)
		_ = os.RemoveAll(tmp)
	} else if exists(filePath) {
		return nil
	}
	if strings.HasPrefix(spec.url, "http") {
		url := i.findMirror(spec.url)
		if err := NewExecutor("/bin/wget").Arg("-c").Arg("-U").Arg("Avyos/0.1 (+https://avyos.dev)").Arg(url).Arg("-O").Arg(tmp).Execute(); err != nil {
			return err
		}
		return os.Rename(tmp, filePath)
	}
	return copyPath(filepath.Join(i.projectPath, spec.url), filePath)
}

func (i *Ignite) VerifySourceFile(path string) error {
	lockFile := filepath.Join(i.projectPath, "checksum.lock")
	if !exists(lockFile) {
		return nil
	}
	checksums, err := readChecksumLock(lockFile)
	if err != nil {
		return err
	}
	filename := filepath.Base(path)
	expected, ok := checksums[filename]
	if !ok {
		return fmt.Errorf("checksum.lock has no entry for source %q", filename)
	}
	actual, err := fileSHA256(path)
	if err != nil {
		return err
	}
	if actual != expected {
		return fmt.Errorf("checksum mismatch for source %q: expected %s, got %s", filename, expected, actual)
	}
	return nil
}

func (i *Ignite) FetchSources(ids []string, force bool) error {
	var recipes []Recipe
	if len(ids) == 0 {
		for _, recipe := range i.pool {
			recipes = append(recipes, recipe)
		}
	} else {
		states, err := i.Resolve(ids, true, true, true)
		if err != nil {
			return err
		}
		for _, state := range states {
			recipes = append(recipes, state.recipe)
		}
	}
	sourceDir := filepath.Join(i.cachePath, "sources")
	lockFile := filepath.Join(i.projectPath, "checksum.lock")
	checksums := map[string]string{}
	if !force && exists(lockFile) {
		var err error
		checksums, err = readChecksumLock(lockFile)
		if err != nil {
			return err
		}
	}
	for _, recipe := range recipes {
		if err := recipe.ResolveSources(*i.config, nil); err != nil {
			return fmt.Errorf("failed to fetch sources for %q (%s): %w", elementName(recipe), recipe.id, err)
		}
		for _, source := range recipe.sources {
			spec, err := parseSourceSpec(source)
			if err != nil {
				return err
			}
			path := filepath.Join(sourceDir, spec.filename)
			if err := i.FetchSourceFile(source, sourceDir, force); err != nil {
				return err
			}
			if _, ok := checksums[spec.filename]; !force && ok {
				fmt.Println("Using locked source:", spec.filename)
				continue
			}
			sum, err := fileSHA256(path)
			if err != nil {
				return err
			}
			checksums[spec.filename] = sum
			fmt.Println("Fetched source:", spec.filename)
		}
		if err := writeChecksumLock(lockFile, checksums); err != nil {
			return err
		}
		fmt.Println("Checkpointed checksum lock after:", recipe.elementID)
	}
	fmt.Println("Wrote checksum lock:", lockFile)
	return nil
}

func (i *Ignite) PrepareSources(recipe Recipe, sourceDir, buildRoot string) (string, error) {
	subdir := ""
	if err := os.MkdirAll(buildRoot, 0755); err != nil {
		return "", err
	}
	for _, source := range recipe.sources {
		spec, err := parseSourceSpec(source)
		if err != nil {
			return "", err
		}
		path := filepath.Join(sourceDir, spec.filename)
		if err := i.FetchSourceFile(source, sourceDir, false); err != nil {
			return "", err
		}
		if err := i.VerifySourceFile(path); err != nil {
			return "", err
		}
		targetRoot := buildRoot
		if subdir != "" {
			targetRoot = filepath.Join(buildRoot, subdir)
		}
		if isPatchFile(spec.filename) {
			if err := applyPatchFile(path, targetRoot); err != nil {
				return "", fmt.Errorf("failed to apply source patch %q: %w", spec.filename, err)
			}
			continue
		}
		if isArchive(path) && !spec.noextract {
			files, err := extract(path, targetRoot)
			if err != nil {
				return "", err
			}
			if subdir == "" && len(files) > 0 {
				dir := files[0]
				if idx := strings.IndexByte(dir, '/'); idx >= 0 {
					dir = dir[:idx]
				}
				subdir = dir
			}
		} else {
			if err := os.MkdirAll(targetRoot, 0755); err != nil {
				return "", err
			}
			if err := copyFile(path, filepath.Join(targetRoot, spec.filename)); err != nil {
				return "", err
			}
		}
	}
	return subdir, nil
}

func (i *Ignite) PrepareWorkspaceSources(recipe Recipe, buildRoot string) (string, error) {
	if !i.WorkspaceAvailable(recipe) {
		return "", fmt.Errorf("workspace is not available for %q", recipe.id)
	}
	if err := copyWorkspaceTree(i.WorkspacePath(recipe), buildRoot); err != nil {
		return "", err
	}
	return ".", nil
}

func (i *Ignite) WorkspaceInit(recipe Recipe) error {
	workspace := i.WorkspacePath(recipe)
	if exists(workspace) {
		return fmt.Errorf("workspace already exists at %q", workspace)
	}
	_ = os.MkdirAll(i.workspacePath, 0755)
	_ = os.MkdirAll(filepath.Join(i.cachePath, "sources"), 0755)
	tmp := workspace + ".tmp." + strconv.Itoa(os.Getpid())
	for suffix := 0; exists(tmp); suffix++ {
		tmp = workspace + ".tmp." + strconv.Itoa(os.Getpid()) + "." + strconv.Itoa(suffix)
	}
	defer func() {
		if exists(tmp) {
			_ = os.RemoveAll(tmp)
		}
	}()
	subdir, err := i.PrepareSources(recipe, filepath.Join(i.cachePath, "sources"), tmp)
	if err != nil {
		return err
	}
	if subdir != "" && subdir != "." && isDir(filepath.Join(tmp, subdir)) {
		if err := moveDirectoryContents(filepath.Join(tmp, subdir), tmp); err != nil {
			return err
		}
		_ = os.Remove(filepath.Join(tmp, subdir))
	}
	meta := filepath.Join(tmp, ".ignite-workspace")
	patches := filepath.Join(meta, "patches")
	original := filepath.Join(meta, "original")
	if err := os.MkdirAll(patches, 0755); err != nil {
		return err
	}
	if err := copyWorkspaceTree(tmp, original); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(meta, "metadata"), []byte(fmt.Sprintf("id: %s\nelement: %s\nversion: %s\n", recipe.id, recipe.elementID, recipe.version)), 0644); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(meta, "env"), []byte("export QUILT_PATCHES=.ignite-workspace/patches\nexport QUILT_SERIES=series\nexport QUILT_PC=.pc\n"), 0644); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(patches, "series"), nil, 0644); err != nil {
		return err
	}
	if err := os.Rename(tmp, workspace); err != nil {
		return err
	}
	fmt.Printf("Workspace initialized: %s\nUse quilt with: source .ignite-workspace/env\n", workspace)
	return nil
}

func (i *Ignite) WorkspaceFinish(recipe Recipe) error {
	workspace := i.WorkspacePath(recipe)
	if !i.WorkspaceAvailable(recipe) {
		return fmt.Errorf("workspace is not available for %q", recipe.id)
	}
	meta := filepath.Join(workspace, ".ignite-workspace")
	patchesDir := filepath.Join(meta, "patches")
	patches, err := readQuiltSeries(filepath.Join(patchesDir, "series"))
	if err != nil {
		return err
	}
	if len(patches) > 0 {
		quilt, err := findBinary("quilt", "quilt not found; install quilt to finish workspaces")
		if err != nil {
			return err
		}
		status, out := NewExecutor(quilt).Arg("applied").Path(workspace).Environ(quiltEnvironment()...).Output()
		if status == 0 && out != "" {
			if err := NewExecutor(quilt).Arg("refresh").Path(workspace).Environ(quiltEnvironment()...).Execute(); err != nil {
				return err
			}
		}
	}
	var exportedSources []string
	outputDir := filepath.Join(i.projectPath, "patches", recipe.id)
	_ = os.MkdirAll(outputDir, 0755)
	if len(patches) == 0 {
		diffRoot := filepath.Join(i.cachePath, "temp", "workspace-diff-"+workspaceComponentID(recipe)+"-"+strconv.Itoa(os.Getpid()))
		_ = os.RemoveAll(diffRoot)
		_ = os.MkdirAll(diffRoot, 0755)
		defer os.RemoveAll(diffRoot)
		original := filepath.Join(meta, "original")
		if isDir(original) {
			if err := copyWorkspaceTree(original, filepath.Join(diffRoot, "a")); err != nil {
				return err
			}
		} else {
			subdir, err := i.PrepareSources(recipe, filepath.Join(i.cachePath, "sources"), filepath.Join(diffRoot, "a"))
			if err != nil {
				return err
			}
			if subdir != "" && subdir != "." && isDir(filepath.Join(diffRoot, "a", subdir)) {
				if err := moveDirectoryContents(filepath.Join(diffRoot, "a", subdir), filepath.Join(diffRoot, "a")); err != nil {
					return err
				}
				_ = os.Remove(filepath.Join(diffRoot, "a", subdir))
			}
		}
		if err := copyWorkspaceTree(workspace, filepath.Join(diffRoot, "b")); err != nil {
			return err
		}
		diffBin, err := findBinary("diff", "diff not found; install diffutils to finish workspaces")
		if err != nil {
			return err
		}
		status, diff := NewExecutor(diffBin).Arg("-Naur").Arg("a").Arg("b").Path(diffRoot).Output()
		if status > 1 {
			return fmt.Errorf("failed to generate workspace diff: %s", diff)
		}
		if diff == "" {
			fmt.Println("Workspace has no source changes; no patches exported")
			_ = os.RemoveAll(workspace)
			fmt.Println("Workspace closed:", workspace)
			return nil
		}
		out := nextPatchPath(outputDir, "0001-"+patchSafeName(recipe.id)+"-workspace.patch")
		if !strings.HasSuffix(diff, "\n") {
			diff += "\n"
		}
		if err := os.WriteFile(out, []byte(diff), 0644); err != nil {
			return err
		}
		fmt.Println("Exported patch:", out)
		exportedSources = append(exportedSources, workspacePatchSource(recipe, out))
		if err := i.RecordWorkspacePatches(recipe, exportedSources); err != nil {
			return err
		}
		_ = os.RemoveAll(workspace)
		fmt.Println("Workspace closed:", workspace)
		return nil
	}
	for _, patch := range patches {
		source := filepath.Join(patchesDir, patch)
		if !exists(source) {
			return fmt.Errorf("quilt patch listed in series is missing: %s", source)
		}
		name := filepath.Base(source)
		if filepath.Ext(name) != ".patch" {
			name += ".patch"
		}
		out := nextPatchPath(outputDir, name)
		if err := copyFile(source, out); err != nil {
			return err
		}
		fmt.Println("Exported patch:", out)
		exportedSources = append(exportedSources, workspacePatchSource(recipe, out))
	}
	if err := i.RecordWorkspacePatches(recipe, exportedSources); err != nil {
		return err
	}
	_ = os.RemoveAll(workspace)
	fmt.Println("Workspace closed:", workspace)
	return nil
}

func workspacePatchSource(recipe Recipe, patchPath string) string {
	return filepath.ToSlash(filepath.Join("patches", recipe.id, filepath.Base(patchPath)))
}

func (i *Ignite) RecordWorkspacePatches(recipe Recipe, sources []string) error {
	if len(sources) == 0 {
		return nil
	}
	if recipe.file == "" {
		return fmt.Errorf("cannot update sources for %q because recipe file is unknown", recipe.id)
	}
	if err := appendSourcesToRecipeFile(recipe.file, recipe.config.StringSlice("sources"), sources); err != nil {
		return err
	}
	if err := i.UpdateChecksumLockForSources(sources); err != nil {
		return err
	}
	fmt.Println("Updated recipe sources:", recipe.file)
	fmt.Println("Updated checksum lock:", filepath.Join(i.projectPath, "checksum.lock"))
	return nil
}

func (i *Ignite) UpdateChecksumLockForSources(sources []string) error {
	lockFile := filepath.Join(i.projectPath, "checksum.lock")
	checksums := map[string]string{}
	if exists(lockFile) {
		var err error
		checksums, err = readChecksumLock(lockFile)
		if err != nil {
			return err
		}
	}
	for _, source := range sources {
		spec, err := parseSourceSpec(source)
		if err != nil {
			return err
		}
		if strings.HasPrefix(spec.url, "http://") || strings.HasPrefix(spec.url, "https://") {
			continue
		}
		path := filepath.Join(i.projectPath, spec.url)
		sum, err := fileSHA256(path)
		if err != nil {
			return err
		}
		checksums[spec.filename] = sum
	}
	return writeChecksumLock(lockFile, checksums)
}

func appendSourcesToRecipeFile(path string, knownSources, sources []string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("failed to read recipe file %q: %w", path, err)
	}
	existing := map[string]bool{}
	for _, source := range knownSources {
		existing[normalizeSourceRef(source)] = true
	}
	for _, source := range scanRecipeSourceLines(string(data)) {
		existing[normalizeSourceRef(source)] = true
	}
	var add []string
	for _, source := range sources {
		key := normalizeSourceRef(source)
		if key == "" || existing[key] {
			continue
		}
		add = append(add, source)
		existing[key] = true
	}
	if len(add) == 0 {
		return nil
	}
	updated := appendSourcesToRecipeText(string(data), add)
	return os.WriteFile(path, []byte(updated), 0644)
}

func appendSourcesToRecipeText(text string, sources []string) string {
	lines := strings.Split(text, "\n")
	sourcesIdx := -1
	for idx, line := range lines {
		if isTopLevelKeyLine(line, "sources") {
			sourcesIdx = idx
			break
		}
	}
	var insert []string
	for _, source := range sources {
		insert = append(insert, "  - "+source)
	}
	if sourcesIdx < 0 {
		base := strings.TrimRight(text, "\n")
		if base != "" {
			base += "\n\n"
		}
		return base + "sources:\n" + strings.Join(insert, "\n") + "\n"
	}
	lines[sourcesIdx] = "sources:"
	indent := "  "
	insertAt := sourcesIdx + 1
	blockEnd := len(lines)
	for idx := sourcesIdx + 1; idx < len(lines); idx++ {
		line := lines[idx]
		if isTopLevelMappingLine(line) {
			blockEnd = idx
			break
		}
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "-") {
			indent = line[:len(line)-len(strings.TrimLeft(line, " \t"))]
			insertAt = idx + 1
		}
	}
	for idx := range insert {
		insert[idx] = indent + "- " + strings.TrimPrefix(insert[idx], "  - ")
	}
	if insertAt > blockEnd {
		insertAt = blockEnd
	}
	out := append([]string{}, lines[:insertAt]...)
	out = append(out, insert...)
	out = append(out, lines[insertAt:]...)
	result := strings.Join(out, "\n")
	if !strings.HasSuffix(result, "\n") {
		result += "\n"
	}
	return result
}

func scanRecipeSourceLines(text string) []string {
	lines := strings.Split(text, "\n")
	sourcesIdx := -1
	for idx, line := range lines {
		if isTopLevelKeyLine(line, "sources") {
			sourcesIdx = idx
			break
		}
	}
	if sourcesIdx < 0 {
		return nil
	}
	var out []string
	for idx := sourcesIdx + 1; idx < len(lines); idx++ {
		line := lines[idx]
		if isTopLevelMappingLine(line) {
			break
		}
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, "-") {
			continue
		}
		value := strings.TrimSpace(strings.TrimPrefix(trimmed, "-"))
		if comment := strings.IndexByte(value, '#'); comment >= 0 {
			value = strings.TrimSpace(value[:comment])
		}
		value = strings.Trim(value, "'\"")
		if value != "" {
			out = append(out, value)
		}
	}
	return out
}

func isTopLevelKeyLine(line, key string) bool {
	if strings.HasPrefix(line, " ") || strings.HasPrefix(line, "\t") {
		return false
	}
	trimmed := strings.TrimSpace(line)
	return trimmed == key+":" || strings.HasPrefix(trimmed, key+": ") || strings.HasPrefix(trimmed, key+":#")
}

func isTopLevelMappingLine(line string) bool {
	if strings.TrimSpace(line) == "" || strings.HasPrefix(line, " ") || strings.HasPrefix(line, "\t") || strings.HasPrefix(strings.TrimSpace(line), "#") {
		return false
	}
	idx := strings.IndexByte(line, ':')
	if idx <= 0 {
		return false
	}
	key := line[:idx]
	for _, r := range key {
		if !((r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_') {
			return false
		}
	}
	return true
}

func normalizeSourceRef(source string) string {
	source = strings.TrimSpace(source)
	if source == "" {
		return ""
	}
	spec, err := parseSourceSpec(source)
	if err == nil {
		source = spec.url
	}
	return filepath.ToSlash(filepath.Clean(source))
}

func (i *Ignite) CompileSource(recipe Recipe, container *Container, buildRoot, installRoot string) error {
	env := append([]string{}, i.config.StringSlice("environ")...)
	env = append(env, recipe.config.StringSlice("environ")...)
	resolvedInstallRoot := filepath.Join(container.hostRoot, installRoot, recipe.PackageName())
	resolvedBuildRoot := filepath.Join(container.hostRoot, buildRoot)
	extra := map[string]string{
		"install-root": filepath.ToSlash(filepath.Join("/", installRoot, recipe.PackageName())),
		"build-root":   filepath.ToSlash(filepath.Join("/", buildRoot)),
	}
	if script, _ := recipe.config.String("pre-script", ""); script != "" {
		resolved, err := recipe.ResolveValue(script, *i.config, extra)
		if err != nil {
			return err
		}
		fmt.Println("Exec(pre-script)")
		if err := NewExecutor("/bin/sh").Arg("-ec").Arg(resolved).Path(extra["build-root"]).Environ(env...).Container(container).Execute(); err != nil {
			return err
		}
	}
	if bt, _ := recipe.config.String("build-type", ""); bt == "import" {
		source := filepath.Join(resolvedBuildRoot, configString(recipe.config, "source", ""))
		target := filepath.Join(resolvedInstallRoot, configString(recipe.config, "target", ""))
		if err := os.MkdirAll(target, 0755); err != nil {
			return err
		}
		if err := NewExecutor("/bin/cp").Arg("-rap").Arg(source + string(os.PathSeparator) + ".").Arg("-t").Arg(target).Execute(); err != nil {
			return err
		}
	} else {
		script, _ := recipe.config.String("script", "")
		if script == "" {
			compiler, err := i.GetCompiler(recipe, resolvedBuildRoot)
			if err != nil {
				return err
			}
			script = compiler.script
		}
		resolved, err := recipe.ResolveValue(script, *i.config, extra)
		if err != nil {
			return err
		}
		fmt.Println("Exec(script)")
		if len(resolved) > 500 {
			scriptPath := filepath.Join(resolvedBuildRoot, "pkgupd_exec_script.sh")
			if err := os.WriteFile(scriptPath, []byte(resolved), 0644); err != nil {
				return err
			}
			if err := NewExecutor("/bin/sh").Arg("-e").Arg("pkgupd_exec_script.sh").Path(extra["build-root"]).Environ(env...).Container(container).Execute(); err != nil {
				return err
			}
		} else if err := NewExecutor("/bin/sh").Arg("-ec").Arg(resolved).Path(extra["build-root"]).Environ(env...).Container(container).Execute(); err != nil {
			return err
		}
	}
	if script, _ := recipe.config.String("post-script", ""); script != "" {
		resolved, err := recipe.ResolveValue(script, *i.config, extra)
		if err != nil {
			return err
		}
		fmt.Println("Exec(post-script)")
		if err := NewExecutor("/bin/sh").Arg("-ec").Arg(resolved).Path(extra["build-root"]).Environ(env...).Container(container).Execute(); err != nil {
			return err
		}
	}
	if recipe.config.Bool("strip", true) {
		return i.Strip(recipe, resolvedInstallRoot)
	}
	return nil
}

func (i *Ignite) Strip(recipe Recipe, installRoot string) error {
	mimes := append([]string{}, i.config.StringSlice("strip-mimetype")...)
	mimes = append(mimes, recipe.config.StringSlice("strip-mimetype")...)
	allowed := map[string]bool{}
	for _, m := range mimes {
		allowed[m] = true
	}
	return filepath.WalkDir(installRoot, func(path string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return err
		}
		info, err := entry.Info()
		if err != nil || !info.Mode().IsRegular() {
			return err
		}
		name := filepath.Base(path)
		ext := filepath.Ext(path)
		if !((ext == ".so" || ext == ".a" || strings.Contains(name, ".so.")) || info.Mode()&0111 != 0) {
			return nil
		}
		if info.Mode()&0200 == 0 {
			return nil
		}
		status, mime := NewExecutor("/bin/file").Arg("-b").Arg("--mime-type").Arg(path).Output()
		if status != 0 {
			fmt.Fprintln(os.Stderr, "failed to read MIME TYPE for "+path+": "+mime)
			return nil
		}
		if !allowed[mime] {
			return nil
		}
		dbg := path + ".dbg"
		if err := NewExecutor("/bin/objcopy").Arg("--only-keep-debug").Arg(path).Arg(dbg).Silent().Execute(); err != nil {
			fmt.Fprintln(os.Stderr, "failed to strip", path, "with mimetype", mime, "because", err)
			return nil
		}
		stripArg := "--strip-all"
		if ext == ".a" {
			stripArg = "--strip-debug"
		} else if ext == ".so" || strings.Contains(name, ".so.") {
			stripArg = "--strip-unneeded"
		}
		if err := NewExecutor("/bin/strip").Arg(stripArg).Arg(path).Silent().Execute(); err != nil {
			fmt.Fprintln(os.Stderr, "failed to strip", path, "with mimetype", mime, "because", err)
			return nil
		}
		if err := NewExecutor("/bin/objcopy").Arg("--add-gnu-debuglink=" + filepath.Base(path) + ".dbg").Arg(path).Path(filepath.Dir(path)).Silent().Execute(); err != nil {
			fmt.Fprintln(os.Stderr, "failed to strip", path, "with mimetype", mime, "because", err)
		}
		return nil
	})
}

func (i *Ignite) Pack(recipe Recipe, container *Container, installRoot, packagePath string) error {
	installRootPackage := filepath.Join(installRoot, recipe.PackageName())
	installRootDbg := filepath.Join(installRoot, recipe.PackageName()+".dbg")
	_ = os.MkdirAll(installRootDbg, 0755)
	keep := []*regexp.Regexp{}
	for _, pattern := range recipe.config.StringSlice("keep-files") {
		keep = append(keep, regexp.MustCompile(pattern))
	}
	keepFile := func(name string) bool {
		for _, r := range keep {
			if r.MatchString(name) {
				return true
			}
		}
		return false
	}
	moveFile := func(path, newRoot string) error {
		rel, _ := filepath.Rel(installRootPackage, path)
		target := filepath.Join(newRoot, rel)
		if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
			return err
		}
		return os.Rename(path, target)
	}
	for _, d := range []string{"usr/src", "usr/lib/debug"} {
		path := filepath.Join(installRootPackage, d)
		if exists(path) {
			if err := moveFile(path, installRootDbg); err != nil {
				return err
			}
		}
	}
	var dirs, removeFiles, dbgFiles []string
	cleanEmpty := recipe.config.Bool("clean-empty-dir", true)
	if exists(installRootPackage) {
		if err := filepath.WalkDir(installRootPackage, func(path string, entry fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if entry.IsDir() {
				if cleanEmpty {
					dirs = append(dirs, path)
				}
				return nil
			}
			if len(keep) > 0 && keepFile(filepath.Base(path)) {
				return nil
			}
			if filepath.Ext(path) == ".la" {
				removeFiles = append(removeFiles, path)
			} else if filepath.Ext(path) == ".dbg" {
				dbgFiles = append(dbgFiles, path)
			}
			return nil
		}); err != nil {
			return err
		}
	}
	for _, p := range removeFiles {
		_ = os.Remove(p)
	}
	for _, p := range dbgFiles {
		if err := moveFile(p, installRootDbg); err != nil {
			return err
		}
	}
	sort.Sort(sort.Reverse(sort.StringSlice(dirs)))
	for _, dir := range dirs {
		entries, err := os.ReadDir(dir)
		if err == nil && len(entries) == 0 {
			_ = os.Remove(dir)
		}
	}
	fmt.Println("Compressing", recipe.Name())
	userMap := filepath.Join(installRoot, "user-map")
	groupMap := filepath.Join(installRoot, "group-map")
	_ = os.WriteFile(userMap, []byte(fmt.Sprintf("+%d root:0\n%s\n%s\n", os.Getuid(), configString(*i.config, "user-map", ""), configString(recipe.config, "user-map", ""))), 0644)
	_ = os.WriteFile(groupMap, []byte(fmt.Sprintf("+%d root:0\n%s\n%s\n", os.Getgid(), configString(*i.config, "group-map", ""), configString(recipe.config, "group-map", ""))), 0644)
	compressor := configString(*i.config, "package-compressor", "zstd -T0 -1")
	for suffix, root := range map[string]string{"": installRootPackage, ".dbg": installRootDbg} {
		if err := NewExecutor("/bin/tar").Arg("--use-compress-program=" + compressor).Arg("--owner-map=" + userMap).Arg("--group-map=" + groupMap).Arg("-cPf").Arg(packagePath + suffix).Arg("-C").Arg(root).Arg(".").Execute(); err != nil {
			return err
		}
	}
	return nil
}

func (i *Ignite) GetCompiler(recipe Recipe, buildRoot string) (Compiler, error) {
	buildType, _ := recipe.config.String("build-type", "")
	if buildType == "" {
		for _, name := range []string{"autotools", "meson", "cmake", "python", "pysetup", "go-pkg", "cargo", "perl"} {
			if exists(filepath.Join(buildRoot, i.compilers[name].file)) {
				buildType = name
				break
			}
		}
	}
	compiler, ok := i.compilers[buildType]
	if buildType == "" || !ok {
		return Compiler{}, fmt.Errorf("unknown build-type or failed to detect build-type %q at %s", buildType, buildRoot)
	}
	return compiler, nil
}

type SourceSpec struct {
	filename  string
	url       string
	noextract bool
}

func parseSourceSpec(source string) (SourceSpec, error) {
	parts := strings.Split(source, "::")
	var values []string
	spec := SourceSpec{}
	for _, part := range parts {
		if part == "noextract" {
			spec.noextract = true
		} else {
			values = append(values, part)
		}
	}
	if len(values) == 0 {
		return spec, fmt.Errorf("source has no url: %q", source)
	}
	spec.url = values[len(values)-1]
	if len(values) > 1 {
		spec.filename = values[0]
	} else if u, err := url.Parse(spec.url); err == nil && u.Path != "" {
		spec.filename = filepath.Base(u.Path)
	} else {
		spec.filename = filepath.Base(spec.url)
	}
	return spec, nil
}

func extract(path, outputPath string) ([]string, error) {
	_ = os.MkdirAll(outputPath, 0755)
	exe := "/bin/tar"
	if filepath.Ext(path) == ".zip" {
		exe = "/bin/bsdtar"
	}
	status, out := NewExecutor(exe).Arg("-xvf").Arg(path).Arg("-C").Arg(outputPath).Output()
	var files []string
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimPrefix(line, "./")
		line = strings.TrimPrefix(line, "x ")
		if line != "" {
			files = append(files, line)
		}
	}
	if status != 0 {
		return nil, fmt.Errorf("failed to extract %s :%s", path, out)
	}
	return files, nil
}

func isPatchFile(path string) bool {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".patch", ".diff":
		return true
	default:
		return false
	}
}

func patchCandidateRoots(root string) []string {
	roots := []string{root}
	entries, err := os.ReadDir(root)
	if err != nil {
		return roots
	}
	for _, entry := range entries {
		if entry.IsDir() {
			roots = append(roots, filepath.Join(root, entry.Name()))
		}
	}
	return roots
}

func applyPatchFile(patchPath, root string) error {
	if !isDir(root) {
		return fmt.Errorf("source root %q does not exist", root)
	}
	stripLevels := []int{1, 0, 2, 3, 4}
	var attempts []string
	for _, candidate := range patchCandidateRoots(root) {
		for _, strip := range stripLevels {
			stripArg := fmt.Sprintf("-p%d", strip)
			status, out := NewExecutor("/bin/patch").Arg("-f").Arg("-s").Arg("--dry-run").Arg(stripArg).Arg("-i").Arg(patchPath).Path(candidate).Output()
			if status != 0 {
				attempts = append(attempts, fmt.Sprintf("%s %s: %s", candidate, stripArg, out))
				continue
			}
			fmt.Printf("Applying source patch: %s in %s with %s\n", filepath.Base(patchPath), candidate, stripArg)
			if err := NewExecutor("/bin/patch").Arg("-f").Arg(stripArg).Arg("-i").Arg(patchPath).Path(candidate).Execute(); err != nil {
				return err
			}
			return nil
		}
	}
	if len(attempts) > 6 {
		attempts = attempts[:6]
	}
	return fmt.Errorf("no matching source tree found under %q for %s; tried %s", root, filepath.Base(patchPath), strings.Join(attempts, "; "))
}

func isArchive(path string) bool {
	switch filepath.Ext(path) {
	case ".tar", ".zip", ".gz", ".xz", ".bzip2", ".tgz", ".txz", ".bz2", ".zst", ".zstd", ".lz":
		return true
	default:
		return false
	}
}

func sha256Hex(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func fileSHA256(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("failed to read source %q for checksum", path)
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil || !info.Mode().IsRegular() {
		return "", fmt.Errorf("cannot checksum non-regular source %q", path)
	}
	h := sha256.New()
	if _, err := io.Copy(h, file); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func readChecksumLock(path string) (map[string]string, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read checksum lock %q", path)
	}
	defer file.Close()
	out := map[string]string{}
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if idx := strings.IndexByte(line, '#'); idx >= 0 {
			line = line[:idx]
		}
		fields := strings.Fields(line)
		if len(fields) >= 2 {
			out[fields[1]] = fields[0]
		}
	}
	return out, scanner.Err()
}

func writeChecksumLock(path string, checksums map[string]string) error {
	var keys []string
	for key := range checksums {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	var b strings.Builder
	for _, key := range keys {
		fmt.Fprintf(&b, "%s  %s\n", checksums[key], key)
	}
	return os.WriteFile(path, []byte(b.String()), 0644)
}

func readQuiltSeries(path string) ([]string, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read quilt series file %q", path)
	}
	defer file.Close()
	var out []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if idx := strings.IndexByte(line, '#'); idx >= 0 {
			line = line[:idx]
		}
		fields := strings.Fields(line)
		if len(fields) > 0 {
			out = append(out, fields[0])
		}
	}
	return out, scanner.Err()
}

func exists(path string) bool {
	_, err := os.Lstat(path)
	return err == nil
}

func isDir(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

func makeTreeRemovable(root string) {
	if !exists(root) {
		return
	}
	_ = filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err == nil {
			_ = os.Chmod(path, 0777)
		}
		return nil
	})
}

func removeTree(root string) bool {
	makeTreeRemovable(root)
	_ = os.RemoveAll(root)
	if exists(root) {
		_ = NewExecutor("/bin/rm").Arg("-r").Arg("-f").Arg(root).Run()
	}
	return !exists(root)
}

func copyPath(src, dst string) error {
	info, err := os.Lstat(src)
	if err != nil {
		return err
	}
	if info.IsDir() {
		return filepath.WalkDir(src, func(path string, entry fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			rel, _ := filepath.Rel(src, path)
			target := filepath.Join(dst, rel)
			if entry.IsDir() {
				return os.MkdirAll(target, 0755)
			}
			return copyFile(path, target)
		})
	}
	return copyFile(src, dst)
}

func copyFile(src, dst string) error {
	info, err := os.Lstat(src)
	if err != nil {
		return err
	}
	_ = os.MkdirAll(filepath.Dir(dst), 0755)
	if info.Mode()&os.ModeSymlink != 0 {
		target, err := os.Readlink(src)
		if err != nil {
			return err
		}
		_ = os.Remove(dst)
		return os.Symlink(target, dst)
	}
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, info.Mode().Perm())
	if err != nil {
		return err
	}
	defer out.Close()
	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return os.Chmod(dst, info.Mode().Perm())
}

func isWorkspaceMetadata(rel string) bool {
	first := strings.Split(filepath.ToSlash(rel), "/")[0]
	return first == ".ignite-workspace" || first == ".pc"
}

func copyWorkspaceTree(src, dst string) error {
	if err := os.MkdirAll(dst, 0755); err != nil {
		return err
	}
	return filepath.WalkDir(src, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if path == src {
			return nil
		}
		rel, _ := filepath.Rel(src, path)
		if isWorkspaceMetadata(rel) {
			if entry.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		target := filepath.Join(dst, rel)
		if entry.IsDir() {
			return os.MkdirAll(target, 0755)
		}
		return copyFile(path, target)
	})
}

func moveDirectoryContents(src, dst string) error {
	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		target := filepath.Join(dst, entry.Name())
		if exists(target) {
			return fmt.Errorf("workspace source collision at %q", target)
		}
		if err := os.Rename(filepath.Join(src, entry.Name()), target); err != nil {
			return err
		}
	}
	return nil
}

func workspaceComponentID(recipe Recipe) string {
	id := recipe.elementID
	if id == "" {
		id = recipe.id
	}
	id = strings.ReplaceAll(id, "/", "-")
	id = strings.ReplaceAll(id, "\\", "-")
	return id
}

func workspacePackageName(recipe Recipe) string {
	name := recipe.PackageName(recipe.elementID)
	name = strings.TrimSuffix(name, ".pkg")
	return name + "-workspace.pkg"
}

func quiltEnvironment() []string {
	return []string{"HOME=/", "PATH=/usr/bin:/bin:/usr/local/bin", "QUILT_PATCHES=.ignite-workspace/patches", "QUILT_SERIES=series", "QUILT_PC=.pc"}
}

func enableCCacheEnvironment(env []string) []string {
	out := make([]string, 0, len(env)+3)
	path := "/usr/bin:/bin:/usr/local/bin"
	for _, item := range env {
		key, value, ok := strings.Cut(item, "=")
		if ok && key == "PATH" {
			path = value
			continue
		}
		if ok && (key == "CCACHE_DIR" || key == "CCACHE_BASEDIR") {
			continue
		}
		out = append(out, item)
	}
	out = append(out, "PATH=/usr/lib/ccache/bin:"+path)
	out = append(out, "CCACHE_DIR=/ccache")
	out = append(out, "CCACHE_BASEDIR=/build-root")
	return out
}

func findBinary(name, errMsg string) (string, error) {
	for _, path := range []string{"/usr/bin/" + name, "/bin/" + name, "/usr/local/bin/" + name} {
		if exists(path) {
			return path, nil
		}
	}
	return "", fmt.Errorf("%s", errMsg)
}

func patchSafeName(value string) string {
	if value == "" {
		value = "workspace"
	}
	var b strings.Builder
	for _, r := range value {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' || r == '.' {
			b.WriteRune(r)
		} else {
			b.WriteByte('-')
		}
	}
	return b.String()
}

var numberedPatch = regexp.MustCompile(`^([0-9]{4})-(.+)$`)

func nextPatchPath(dir, preferred string) string {
	path := filepath.Join(dir, preferred)
	if !exists(path) {
		return path
	}
	number := 1
	tail := filepath.Base(preferred)
	if m := numberedPatch.FindStringSubmatch(tail); m != nil {
		number, _ = strconv.Atoi(m[1])
		number++
		tail = m[2]
	}
	for {
		candidate := filepath.Join(dir, fmt.Sprintf("%04d-%s", number, tail))
		if !exists(candidate) {
			return candidate
		}
		number++
	}
}

func configString(c Config, key, fallback string) string {
	v, err := c.String(key, fallback)
	if err != nil {
		return fallback
	}
	return v
}

func elementName(recipe Recipe) string {
	if recipe.elementID != "" {
		return recipe.elementID
	}
	return recipe.id
}
