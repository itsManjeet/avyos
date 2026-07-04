package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
)

type JobState string

const (
	JobQueued    JobState = "queued"
	JobBlocked   JobState = "blocked"
	JobRunning   JobState = "running"
	JobSuccess   JobState = "success"
	JobFailed    JobState = "failed"
	JobCancelled JobState = "cancelled"
)

type BuildJob struct {
	ID            string   `json:"id"`
	Kind          string   `json:"kind"`
	GroupID       string   `json:"group_id,omitempty"`
	Recipes       []string `json:"recipes"`
	TargetRecipes []string `json:"target_recipes,omitempty"`
	CurrentRecipe string   `json:"current_recipe"`
	State         JobState `json:"state"`
	CreatedAt     int64    `json:"created_at"`
	StartedAt     int64    `json:"started_at"`
	FinishedAt    int64    `json:"finished_at"`
	ExitCode      int      `json:"exit_code"`
	LogPath       string   `json:"log_path"`
	Force         bool     `json:"force"`
	Push          bool     `json:"push"`
	Message       string   `json:"message"`
	pid           int
}

type Dashboard struct {
	ignite      *Ignite
	port        int
	bindHost    string
	projectPath string
	cachePath   string
	arch        string
	assetsPath  string
	logRoot     string

	watchRecipes           bool
	recipeFingerprint      string
	recipeReloadedAt       int64
	recipeReloadError      string
	recipeReloadGeneration atomic.Uint64
	reloadMu               sync.RWMutex

	mu       sync.Mutex
	cond     *sync.Cond
	queue    []*BuildJob
	history  []*BuildJob
	active   *BuildJob
	stopping bool
	counter  atomic.Uint64
}

func NewDashboard(ignite *Ignite, port int, bindHost, projectPath, cachePath, arch, assetsPath string) *Dashboard {
	d := &Dashboard{ignite: ignite, port: port, bindHost: bindHost, projectPath: projectPath, cachePath: cachePath, arch: arch, assetsPath: assetsPath, logRoot: filepath.Join(cachePath, "dashboard-logs")}
	d.cond = sync.NewCond(&d.mu)
	_ = os.MkdirAll(d.logRoot, 0755)
	return d
}

func (d *Dashboard) EnableRecipeWatcher(enable bool) {
	d.watchRecipes = enable
}

func (d *Dashboard) reloadRecipes() error {
	d.reloadMu.Lock()
	defer d.reloadMu.Unlock()
	if err := d.ignite.Load(); err != nil {
		d.recipeReloadError = err.Error()
		return err
	}
	d.recipeReloadError = ""
	d.recipeReloadedAt = time.Now().Unix()
	d.recipeReloadGeneration.Add(1)
	d.recipeFingerprint = d.scanRecipeFingerprint()
	return nil
}

func (d *Dashboard) watchRecipeLoop() {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	d.recipeFingerprint = d.scanRecipeFingerprint()
	d.recipeReloadedAt = time.Now().Unix()
	for range ticker.C {
		next := d.scanRecipeFingerprint()
		if next == "" || next == d.recipeFingerprint {
			continue
		}
		fmt.Println("ignite server: recipe change detected; reloading recipe graph")
		if err := d.reloadRecipes(); err != nil {
			fmt.Println("ignite server: recipe reload failed:", err)
		}
	}
}

func (d *Dashboard) scanRecipeFingerprint() string {
	var b strings.Builder
	for _, path := range []string{filepath.Join(d.projectPath, "checksum.lock")} {
		if info, err := os.Stat(path); err == nil {
			fmt.Fprintf(&b, "%s:%d:%d\n", filepath.ToSlash(path), info.Size(), info.ModTime().UnixNano())
		}
	}
	root := filepath.Join(d.projectPath, "elements")
	_ = filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() || filepath.Ext(path) != ".yml" {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return nil
		}
		rel, _ := filepath.Rel(d.projectPath, path)
		fmt.Fprintf(&b, "%s:%d:%d\n", filepath.ToSlash(rel), info.Size(), info.ModTime().UnixNano())
		return nil
	})
	return b.String()
}

func (d *Dashboard) newJobID() string {
	return fmt.Sprintf("%d-%d", time.Now().Unix(), d.counter.Add(1))
}

func (d *Dashboard) enqueueJob(kind string, recipes []string, force, push bool, message string) *BuildJob {
	if kind == "" {
		kind = "build"
	}
	id := d.newJobID()
	job := &BuildJob{ID: id, Kind: kind, Recipes: recipes, State: JobQueued, CreatedAt: time.Now().Unix(), LogPath: filepath.Join(d.logRoot, id+".log"), Force: force, Push: push, Message: message, pid: -1}
	if len(recipes) == 1 {
		job.CurrentRecipe = recipes[0]
	}
	d.mu.Lock()
	d.queue = append(d.queue, job)
	d.mu.Unlock()
	d.cond.Broadcast()
	return job
}

func (d *Dashboard) normalizeRecipeArgs(recipes []string) ([]string, error) {
	out := make([]string, 0, len(recipes))
	for _, value := range recipes {
		recipe, err := findRecipe(d.ignite, value)
		if err != nil {
			return nil, err
		}
		out = append(out, elementName(recipe))
	}
	return out, nil
}

func (d *Dashboard) enqueueBuildPlan(recipes []string, force bool) (map[string]any, error) {
	if len(recipes) == 0 {
		return nil, fmt.Errorf("no recipes provided")
	}
	d.reloadMu.RLock()
	normalized, err := d.normalizeRecipeArgs(recipes)
	if err == nil {
		states, err := d.ignite.Resolve(normalized, true, true, true)
		d.reloadMu.RUnlock()
		if err != nil {
			return nil, err
		}
		return d.enqueueResolvedBuildPlan(normalized, states, force), nil
	}
	d.reloadMu.RUnlock()
	return nil, err
}

func (d *Dashboard) enqueueResolvedBuildPlan(recipes []string, states []State, force bool) map[string]any {
	groupID := d.newJobID()
	var jobs []*BuildJob
	for _, state := range states {
		if state.cached && !force {
			continue
		}
		job := d.enqueueJob("build", []string{state.id}, force, false, "")
		job.GroupID = groupID
		job.TargetRecipes = append([]string{}, recipes...)
		job.CurrentRecipe = state.id
		jobs = append(jobs, job)
	}
	message := "queued dependency-aware build plan"
	if len(jobs) == 0 {
		message = "all requested recipes are already cached"
	}
	return map[string]any{"group_id": groupID, "jobs": jobs, "count": len(jobs), "message": message}
}

func (d *Dashboard) findJob(id string) *BuildJob {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.active != nil && d.active.ID == id {
		return d.active
	}
	for _, job := range d.queue {
		if job.ID == id {
			return job
		}
	}
	for _, job := range d.history {
		if job.ID == id {
			return job
		}
	}
	return nil
}

func (d *Dashboard) workerLoop() {
	for {
		d.mu.Lock()
		for !d.stopping && len(d.queue) == 0 {
			d.cond.Wait()
		}
		if d.stopping {
			d.mu.Unlock()
			return
		}
		job := d.queue[0]
		d.queue = d.queue[1:]
		d.active = job
		job.State = JobRunning
		job.StartedAt = time.Now().Unix()
		d.mu.Unlock()

		d.runJob(job)

		d.mu.Lock()
		job.FinishedAt = time.Now().Unix()
		d.history = append(d.history, job)
		if len(d.history) > 100 {
			d.history = d.history[len(d.history)-100:]
		}
		d.active = nil
		d.mu.Unlock()
	}
}

func (d *Dashboard) runJob(job *BuildJob) {
	log, err := os.OpenFile(job.LogPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		job.State = JobFailed
		job.ExitCode = -1
		return
	}
	defer log.Close()
	_, _ = fmt.Fprintf(log, "==> ignite server job %s (%s)\n", job.ID, job.Kind)
	if job.GroupID != "" {
		_, _ = fmt.Fprintf(log, "==> group: %s\n", job.GroupID)
	}
	if len(job.Recipes) > 0 {
		_, _ = fmt.Fprintf(log, "==> recipes: %s\n", strings.Join(job.Recipes, ", "))
	}

	var runErr error
	switch job.Kind {
	case "build", "":
		runErr = d.runBuildJob(job, log)
	case "fetch":
		runErr = d.runFetchJob(job, log)
	case "workspace":
		runErr = d.runWorkspaceJob(job, log)
	case "workspace-finish":
		runErr = d.runWorkspaceFinishJob(job, log)
	case "status":
		runErr = d.runStatusJob(job, log)
	default:
		runErr = fmt.Errorf("unknown dashboard action %q", job.Kind)
	}

	if job.State == JobCancelled {
		return
	}
	if runErr != nil {
		job.ExitCode = 1
		job.State = JobFailed
		_, _ = fmt.Fprintf(log, "==> failed: %s\n", runErr)
		return
	}
	job.ExitCode = 0
	job.State = JobSuccess
	_, _ = fmt.Fprintln(log, "==> completed")
}

func (d *Dashboard) runIgniteCLI(job *BuildJob, log io.Writer, current string, flags []string, command string, args []string) error {
	exe, err := os.Executable()
	if err != nil {
		exe = "ignite"
	}
	if current != "" {
		d.mu.Lock()
		job.CurrentRecipe = current
		d.mu.Unlock()
	}
	cmdArgs := []string{"-project-path", d.projectPath, "-cache-path", d.cachePath, "-workspace-path", d.ignite.workspacePath, "-arch", d.arch}
	cmdArgs = append(cmdArgs, flags...)
	cmdArgs = append(cmdArgs, command)
	cmdArgs = append(cmdArgs, args...)
	_, _ = fmt.Fprintf(log, "==> ignite %s\n", strings.Join(cmdArgs, " "))
	cmd := exec.Command(exe, cmdArgs...)
	cmd.Stdout = log
	cmd.Stderr = log
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := cmd.Start(); err != nil {
		return err
	}
	d.mu.Lock()
	job.pid = cmd.Process.Pid
	d.mu.Unlock()
	err = cmd.Wait()
	d.mu.Lock()
	job.pid = -1
	d.mu.Unlock()
	if err != nil {
		if exit, ok := err.(*exec.ExitError); ok {
			code := exit.ExitCode()
			if job.State == JobCancelled {
				return nil
			}
			job.ExitCode = code
			return fmt.Errorf("ignite %s failed (exit %d)", command, code)
		}
		return err
	}
	return nil
}

func (d *Dashboard) runBuildJob(job *BuildJob, log io.Writer) error {
	for _, recipeID := range job.Recipes {
		if err := d.runIgniteCLI(job, log, recipeID, nil, "build-one", []string{recipeID}); err != nil {
			return err
		}
		_, _ = fmt.Fprintf(log, "==> completed %s\n", recipeID)
	}
	return nil
}

func (d *Dashboard) runFetchJob(job *BuildJob, log io.Writer) error {
	flags := []string{}
	if job.Force {
		flags = append(flags, "-force")
	}
	return d.runIgniteCLI(job, log, "", flags, "fetch", job.Recipes)
}

func (d *Dashboard) runWorkspaceJob(job *BuildJob, log io.Writer) error {
	for _, recipeID := range job.Recipes {
		if err := d.runIgniteCLI(job, log, recipeID, nil, "workspace", []string{recipeID}); err != nil {
			return err
		}
	}
	return nil
}

func (d *Dashboard) runWorkspaceFinishJob(job *BuildJob, log io.Writer) error {
	flags := []string{}
	message := job.Message
	if message == "" {
		message = workspaceMessage
	}
	if message != "" {
		flags = append(flags, "-workspace-message", message)
	}
	if job.Push {
		flags = append(flags, "-workspace-push")
	}
	for _, recipeID := range job.Recipes {
		if err := d.runIgniteCLI(job, log, recipeID, flags, "workspace-finish", []string{recipeID}); err != nil {
			return err
		}
	}
	return nil
}

func (d *Dashboard) runStatusJob(job *BuildJob, log io.Writer) error {
	recipes := append([]string{}, job.Recipes...)
	if len(recipes) == 0 {
		for key := range d.ignite.pool {
			recipes = append(recipes, key)
		}
		sort.Strings(recipes)
	}
	return d.runIgniteCLI(job, log, "", nil, "status", recipes)
}

func (d *Dashboard) Run() int {
	go d.workerLoop()
	if d.watchRecipes {
		go d.watchRecipeLoop()
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/api/status", d.handleStatus)
	mux.HandleFunc("/api/recipes", d.handleRecipes)
	mux.HandleFunc("/api/recipes/", d.handleRecipe)
	mux.HandleFunc("/api/sources", d.handleSources)
	mux.HandleFunc("/api/workspaces", d.handleWorkspaces)
	mux.HandleFunc("/api/actions", d.handleActions)
	mux.HandleFunc("/api/builds", d.handleBuilds)
	mux.HandleFunc("/api/builds/", d.handleBuild)
	mux.HandleFunc("/api/fetch", d.handleFetch)
	mux.Handle("/", http.FileServer(http.Dir(d.assetsPath)))
	server := &http.Server{Addr: fmt.Sprintf("%s:%d", d.bindHost, d.port), Handler: cors(mux)}
	fmt.Printf("ignite server listening on http://%s:%d\n", d.bindHost, d.port)
	if d.watchRecipes {
		fmt.Println("recipe watcher enabled")
	}
	fmt.Println("serving assets from", d.assetsPath)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return 1
	}
	d.mu.Lock()
	d.stopping = true
	d.cond.Broadcast()
	d.mu.Unlock()
	return 0
}

func cors(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		w.Header().Set("Access-Control-Allow-Methods", "GET,POST,OPTIONS")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func (d *Dashboard) handleStatus(w http.ResponseWriter, r *http.Request) {
	d.reloadMu.RLock()
	defer d.reloadMu.RUnlock()
	total, cached, workspace, broken := 0, 0, 0, 0
	container, gitSources, patchSources, localSources, remoteSources := 0, 0, 0, 0, 0
	for _, recipe := range d.ignite.pool {
		total++
		container++
		copy := recipe
		state, _, _ := d.recipeBuildState(copy)
		switch state {
		case "workspace":
			workspace++
		case "cached":
			cached++
		case "unknown":
			broken++
		}
		if err := copy.ResolveSources(*d.ignite.config, nil); err == nil {
			for _, source := range copy.sources {
				spec, err := parseSourceSpec(source)
				if err != nil {
					continue
				}
				switch sourceKind(spec) {
				case "git":
					gitSources++
				case "patch":
					patchSources++
				case "remote", "archive":
					remoteSources++
				default:
					localSources++
				}
			}
		}
	}
	channel := ""
	if variables := d.ignite.config.ScalarMap("variables"); variables != nil {
		channel = variables["channel"]
	}
	d.mu.Lock()
	active := d.active
	queueLen := len(d.queue)
	historyLen := len(d.history)
	d.mu.Unlock()
	writeJSON(w, 200, map[string]any{
		"total": total, "cached": cached, "workspace": workspace, "waiting": total - cached - workspace - broken, "broken": broken,
		"arch": d.arch, "project_path": d.projectPath, "cache_path": d.cachePath, "workspace_path": d.ignite.workspacePath, "source_path": filepath.Join(d.cachePath, "sources"), "log_path": d.logRoot,
		"local_conf": exists(filepath.Join(d.projectPath, "local.conf.yml")), "version": configString(*d.ignite.config, "version", ""), "channel": channel,
		"server": true, "watching": d.watchRecipes, "recipe_reloaded_at": d.recipeReloadedAt, "recipe_reload_generation": d.recipeReloadGeneration.Load(), "recipe_reload_error": d.recipeReloadError,
		"container": container, "git_sources": gitSources, "patch_sources": patchSources, "local_sources": localSources, "remote_sources": remoteSources,
		"queue": queueLen, "history": historyLen, "active": active,
	})
}

func (d *Dashboard) handleRecipes(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/api/recipes" {
		d.handleRecipe(w, r)
		return
	}
	d.reloadMu.RLock()
	defer d.reloadMu.RUnlock()
	query := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("q")))
	stateFilter := map[string]bool{}
	for _, value := range r.URL.Query()["state"] {
		for _, part := range strings.Split(value, ",") {
			part = strings.TrimSpace(part)
			if part != "" {
				stateFilter[part] = true
			}
		}
	}
	sourceFilter := strings.TrimSpace(r.URL.Query().Get("source"))
	limit := queryInt(r, "limit", 100)
	offset := queryInt(r, "offset", 0)
	dependents := d.dependentsMap()

	var out []map[string]any
	for key, recipe := range d.ignite.pool {
		copy := recipe
		state, note, hash := d.recipeBuildState(copy)
		if len(stateFilter) > 0 && !stateFilter[state] {
			continue
		}
		mode := "container"
		sourceStats := d.recipeSourceStats(copy)
		if sourceFilter != "" && sourceStats[sourceFilter] == 0 {
			continue
		}
		if query != "" && !strings.Contains(strings.ToLower(key), query) &&
			!strings.Contains(strings.ToLower(recipe.id), query) &&
			!strings.Contains(strings.ToLower(recipe.version), query) &&
			!strings.Contains(strings.ToLower(recipe.about), query) {
			continue
		}
		item := map[string]any{"id": key, "recipe_id": recipe.id, "version": recipe.version, "about": recipe.about, "state": state, "depends": recipe.depends, "build_time_depends": recipe.buildTimeDepends, "cache": hash, "mode": mode, "source_count": sourceStats["total"], "patch_count": sourceStats["patch"], "git_source_count": sourceStats["git"], "dependents": dependents[key]}
		if note != "" {
			item["note"] = note
		}
		out = append(out, item)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i]["id"].(string) < out[j]["id"].(string)
	})
	total := len(out)
	if offset < 0 {
		offset = 0
	}
	if limit <= 0 {
		limit = 100
	}
	if offset > len(out) {
		offset = len(out)
	}
	end := offset + limit
	if end > len(out) {
		end = len(out)
	}
	writeJSON(w, 200, map[string]any{"items": out[offset:end], "total": total, "offset": offset, "limit": limit})
}

func (d *Dashboard) handleRecipe(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/api/recipes/")
	if strings.HasSuffix(id, "/builds") {
		id = strings.TrimSuffix(id, "/builds")
		d.handleRecipeBuilds(w, r, id)
		return
	}
	d.reloadMu.RLock()
	defer d.reloadMu.RUnlock()
	recipe, ok := d.ignite.pool[id]
	if !ok {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte("null"))
		return
	}
	copy := recipe
	state, note, hash := d.recipeBuildState(copy)
	copy.cache = hash
	_ = copy.ResolveSources(*d.ignite.config, nil)
	cached, workspace := false, false
	if copy.cache != "" {
		cached = exists(d.ignite.CacheFile(copy))
		workspace = d.ignite.WorkspaceAvailable(copy)
	}
	mode := "container"
	dependents := d.dependentsMap()[id]
	out := map[string]any{
		"id": id, "recipe_id": copy.id, "element_id": copy.elementID, "version": copy.version, "about": copy.about, "cache": copy.cache, "state": state, "package_name": copy.PackageName(copy.elementID), "cache_file": d.ignite.CacheFile(copy),
		"depends": copy.depends, "build_time_depends": copy.buildTimeDepends, "dependents": dependents, "sources": copy.sources, "source_details": d.recipeSourceDetails(copy), "backup": copy.backup, "integration": copy.integration,
		"cached": cached, "workspace": workspace, "workspace_path": d.ignite.WorkspacePath(copy), "mode": mode, "container": true,
	}
	if note != "" {
		out["note"] = note
	}
	writeJSON(w, 200, out)
}

func (d *Dashboard) handleRecipeBuilds(w http.ResponseWriter, r *http.Request, id string) {
	d.mu.Lock()
	items := append([]*BuildJob{}, d.history...)
	if d.active != nil {
		items = append(items, d.active)
	}
	items = append(items, d.queue...)
	d.mu.Unlock()

	out := []*BuildJob{}
	for _, job := range items {
		if buildIncludesRecipe(job, id) {
			out = append(out, job)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		ai := out[i].StartedAt
		if ai == 0 {
			ai = out[i].CreatedAt
		}
		if ai == 0 {
			ai = out[i].FinishedAt
		}
		aj := out[j].StartedAt
		if aj == 0 {
			aj = out[j].CreatedAt
		}
		if aj == 0 {
			aj = out[j].FinishedAt
		}
		return ai > aj
	})
	limit := queryInt(r, "limit", 12)
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	writeJSON(w, 200, map[string]any{"items": out, "total": len(out)})
}

func (d *Dashboard) handleSources(w http.ResponseWriter, r *http.Request) {
	d.reloadMu.RLock()
	defer d.reloadMu.RUnlock()
	query := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("q")))
	typeFilter := strings.TrimSpace(r.URL.Query().Get("type"))
	items := []map[string]any{}
	for key, recipe := range d.ignite.pool {
		copy := recipe
		if err := copy.ResolveSources(*d.ignite.config, nil); err != nil {
			items = append(items, map[string]any{"recipe": key, "error": err.Error()})
			continue
		}
		for idx, source := range copy.sources {
			spec, err := parseSourceSpec(source)
			if err != nil {
				items = append(items, map[string]any{"recipe": key, "source": source, "index": idx, "error": err.Error()})
				continue
			}
			kind := sourceKind(spec)
			if typeFilter != "" && typeFilter != kind {
				continue
			}
			needle := strings.ToLower(source + " " + spec.filename + " " + key + " " + copy.id)
			if query != "" && !strings.Contains(needle, query) {
				continue
			}
			locked, hasLock, lockErr := d.ignite.LockedSourceChecksum(spec.filename)
			cachedPath := filepath.Join(d.cachePath, "sources", spec.filename)
			localPath := ""
			localExists := false
			if !spec.IsGit() && !strings.HasPrefix(spec.url, "http://") && !strings.HasPrefix(spec.url, "https://") {
				localPath = filepath.Join(d.projectPath, spec.url)
				localExists = exists(localPath)
			}
			item := map[string]any{"recipe": key, "recipe_id": copy.id, "index": idx, "source": source, "name": spec.filename, "url": spec.url, "type": kind, "noextract": spec.noextract, "locked": hasLock, "checksum": locked, "cached": exists(cachedPath), "cache_path": cachedPath, "local_path": localPath, "local_exists": localExists}
			if spec.IsGit() {
				if remote, err := spec.GitRemote(); err == nil {
					item["remote"] = remote
				}
				item["ref"] = spec.GitRef()
			}
			if lockErr != nil {
				item["lock_error"] = lockErr.Error()
			}
			items = append(items, item)
		}
	}
	sort.Slice(items, func(i, j int) bool {
		ai := fmt.Sprint(items[i]["name"])
		aj := fmt.Sprint(items[j]["name"])
		if ai == aj {
			return fmt.Sprint(items[i]["recipe"]) < fmt.Sprint(items[j]["recipe"])
		}
		return ai < aj
	})
	writeJSON(w, 200, map[string]any{"items": items, "total": len(items)})
}

func (d *Dashboard) handleWorkspaces(w http.ResponseWriter, r *http.Request) {
	d.reloadMu.RLock()
	defer d.reloadMu.RUnlock()
	items := []map[string]any{}
	for key, recipe := range d.ignite.pool {
		copy := recipe
		if hash, err := d.ignite.Hash(copy); err == nil {
			copy.cache = hash
		}
		if !d.ignite.WorkspaceAvailable(copy) {
			continue
		}
		path := d.ignite.WorkspacePath(copy)
		meta := filepath.Join(path, ".ignite-workspace")
		metadata := readWorkspaceMetadata(meta)
		status := ""
		dirty := false
		if isDir(filepath.Join(path, ".git")) {
			if out, err := gitStatusPorcelain(path); err == nil {
				status = strings.TrimSpace(out)
				dirty = status != ""
			}
		} else {
			status = workspaceDiffSummary(path, filepath.Join(meta, "original"))
			dirty = status != ""
		}
		items = append(items, map[string]any{"id": key, "recipe_id": recipe.id, "version": recipe.version, "path": path, "metadata": metadata, "git": metadata["git"] == "true", "branch": metadata["git-branch"], "dirty": dirty, "status": status})
	}
	sort.Slice(items, func(i, j int) bool { return items[i]["id"].(string) < items[j]["id"].(string) })
	writeJSON(w, 200, map[string]any{"items": items, "total": len(items), "workspace_path": d.ignite.workspacePath})
}

func (d *Dashboard) handleActions(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, 405, map[string]string{"error": "method not allowed"})
		return
	}
	var body struct {
		Action  string   `json:"action"`
		Recipes []string `json:"recipes"`
		Force   bool     `json:"force"`
		Push    bool     `json:"push"`
		Message string   `json:"message"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, 400, map[string]string{"error": err.Error()})
		return
	}
	body.Action = strings.TrimSpace(body.Action)
	allowed := map[string]bool{"build": true, "fetch": true, "workspace": true, "workspace-finish": true, "status": true}
	if !allowed[body.Action] {
		writeJSON(w, 400, map[string]string{"error": "unknown action"})
		return
	}
	if (body.Action == "build" || body.Action == "workspace" || body.Action == "workspace-finish") && len(body.Recipes) == 0 {
		writeJSON(w, 400, map[string]string{"error": "no recipes provided"})
		return
	}
	if body.Action == "build" {
		plan, err := d.enqueueBuildPlan(body.Recipes, body.Force)
		if err != nil {
			writeJSON(w, 400, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, 200, plan)
		return
	}
	job := d.enqueueJob(body.Action, body.Recipes, body.Force, body.Push, body.Message)
	writeJSON(w, 200, job)
}

func (d *Dashboard) handleBuilds(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		var body struct {
			Recipes []string `json:"recipes"`
		}
		var recipes []string
		data, _ := io.ReadAll(r.Body)
		if err := json.Unmarshal(data, &body); err == nil && len(body.Recipes) > 0 {
			recipes = body.Recipes
		} else {
			_ = json.Unmarshal(data, &recipes)
		}
		if len(recipes) == 0 {
			writeJSON(w, 400, map[string]string{"error": "no recipes provided"})
			return
		}
		plan, err := d.enqueueBuildPlan(recipes, false)
		if err != nil {
			writeJSON(w, 400, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, 200, plan)
		return
	}
	d.mu.Lock()
	out := map[string]any{"active": d.active, "queue": d.queue, "history": reverseJobs(d.history)}
	d.mu.Unlock()
	writeJSON(w, 200, out)
}

func (d *Dashboard) handleBuild(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/builds/")
	if strings.HasSuffix(path, "/cancel") && r.Method == http.MethodPost {
		id := strings.TrimSuffix(path, "/cancel")
		job := d.findJob(id)
		if job == nil {
			writeJSON(w, 404, map[string]string{"error": "not found"})
			return
		}
		d.mu.Lock()
		if job.State == JobQueued {
			job.State = JobCancelled
			job.FinishedAt = time.Now().Unix()
			for idx, queued := range d.queue {
				if queued.ID == job.ID {
					d.queue = append(d.queue[:idx], d.queue[idx+1:]...)
					break
				}
			}
			d.history = append(d.history, job)
		} else if job.State == JobRunning {
			job.State = JobCancelled
			if job.pid > 0 {
				_ = syscall.Kill(-job.pid, syscall.SIGTERM)
			}
		}
		d.mu.Unlock()
		writeJSON(w, 200, job)
		return
	}
	if strings.HasSuffix(path, "/logs") {
		id := strings.TrimSuffix(path, "/logs")
		job := d.findJob(id)
		if job == nil {
			writeJSON(w, 404, map[string]string{"error": "not found"})
			return
		}
		d.streamLogs(w, job)
		return
	}
	job := d.findJob(path)
	if job == nil {
		writeJSON(w, 404, map[string]string{"error": "not found"})
		return
	}
	writeJSON(w, 200, job)
}

func (d *Dashboard) streamLogs(w http.ResponseWriter, job *BuildJob) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	flusher, _ := w.(http.Flusher)
	file, err := waitOpen(job.LogPath, 5*time.Second)
	if err != nil {
		return
	}
	defer file.Close()
	reader := bufio.NewReader(file)
	for {
		line, err := reader.ReadString('\n')
		if line != "" {
			payload, _ := json.Marshal(strings.TrimSuffix(line, "\n"))
			_, _ = fmt.Fprintf(w, "data: %s\n\n", payload)
			if flusher != nil {
				flusher.Flush()
			}
		}
		if err == nil {
			continue
		}
		if job.State != JobRunning && job.State != JobQueued {
			_, _ = fmt.Fprintf(w, "event: end\ndata: %s\n\n", job.State)
			if flusher != nil {
				flusher.Flush()
			}
			return
		}
		time.Sleep(400 * time.Millisecond)
	}
}

func (d *Dashboard) handleFetch(w http.ResponseWriter, r *http.Request) {
	var recipes []string
	_ = json.NewDecoder(r.Body).Decode(&recipes)
	job := d.enqueueJob("fetch", recipes, force, false, "")
	writeJSON(w, 200, job)
}

func (d *Dashboard) recipeBuildState(recipe Recipe) (string, string, string) {
	hash, err := d.ignite.Hash(recipe)
	if err != nil {
		return "unknown", err.Error(), ""
	}
	recipe.cache = hash
	if d.ignite.WorkspaceAvailable(recipe) {
		return "workspace", "", hash
	}
	if exists(d.ignite.CacheFile(recipe)) {
		return "cached", "", hash
	}
	return "waiting", "", hash
}

func (d *Dashboard) recipeSourceStats(recipe Recipe) map[string]int {
	stats := map[string]int{"total": 0, "git": 0, "patch": 0, "local": 0, "remote": 0, "archive": 0}
	copy := recipe
	if err := copy.ResolveSources(*d.ignite.config, nil); err != nil {
		return stats
	}
	for _, source := range copy.sources {
		spec, err := parseSourceSpec(source)
		if err != nil {
			continue
		}
		kind := sourceKind(spec)
		stats["total"]++
		stats[kind]++
	}
	return stats
}

func (d *Dashboard) recipeSourceDetails(recipe Recipe) []map[string]any {
	var out []map[string]any
	for idx, source := range recipe.sources {
		spec, err := parseSourceSpec(source)
		if err != nil {
			out = append(out, map[string]any{"source": source, "index": idx, "error": err.Error()})
			continue
		}
		locked, hasLock, _ := d.ignite.LockedSourceChecksum(spec.filename)
		item := map[string]any{"source": source, "index": idx, "name": spec.filename, "url": spec.url, "type": sourceKind(spec), "noextract": spec.noextract, "locked": hasLock, "checksum": locked, "cached": exists(filepath.Join(d.cachePath, "sources", spec.filename))}
		if spec.IsGit() {
			if remote, err := spec.GitRemote(); err == nil {
				item["remote"] = remote
			}
			item["ref"] = spec.GitRef()
		}
		out = append(out, item)
	}
	return out
}

func (d *Dashboard) dependentsMap() map[string][]string {
	out := map[string][]string{}
	for key, recipe := range d.ignite.pool {
		for _, dep := range recipe.depends {
			out[dep] = append(out[dep], key)
		}
	}
	for key := range out {
		sort.Strings(out[key])
	}
	return out
}

func sourceKind(spec SourceSpec) string {
	if spec.IsGit() {
		return "git"
	}
	if isPatchFile(spec.filename) || isPatchFile(spec.url) {
		return "patch"
	}
	if strings.HasPrefix(spec.url, "http://") || strings.HasPrefix(spec.url, "https://") {
		return "remote"
	}
	if isArchiveSource(spec.filename) {
		return "archive"
	}
	return "local"
}

func isArchiveSource(name string) bool {
	name = strings.ToLower(name)
	for _, suffix := range []string{".tar", ".tar.gz", ".tgz", ".tar.xz", ".txz", ".tar.bz2", ".tbz2", ".zip", ".tar.zst"} {
		if strings.HasSuffix(name, suffix) {
			return true
		}
	}
	return false
}

func workspaceDiffSummary(workspace, original string) string {
	if !isDir(original) {
		return ""
	}
	diffBin, err := findBinary("diff", "")
	if err != nil {
		return ""
	}
	status, out := NewExecutor(diffBin).Arg("-qr").Arg(original).Arg(workspace).Output()
	if status == 0 {
		return ""
	}
	lines := []string{}
	for _, line := range strings.Split(out, "\n") {
		if line == "" || strings.Contains(line, ".ignite-workspace") {
			continue
		}
		lines = append(lines, line)
		if len(lines) >= 20 {
			break
		}
	}
	return strings.Join(lines, "\n")
}

func reverseJobs(in []*BuildJob) []*BuildJob {
	out := make([]*BuildJob, len(in))
	for idx := range in {
		out[idx] = in[len(in)-1-idx]
	}
	return out
}

func buildIncludesRecipe(job *BuildJob, id string) bool {
	if job.CurrentRecipe == id {
		return true
	}
	for _, recipe := range job.Recipes {
		if recipe == id {
			return true
		}
	}
	return false
}

func queryInt(r *http.Request, key string, fallback int) int {
	value := r.URL.Query().Get(key)
	if value == "" {
		return fallback
	}
	n, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	return n
}

func waitOpen(path string, timeout time.Duration) (*os.File, error) {
	deadline := time.Now().Add(timeout)
	for {
		file, err := os.Open(path)
		if err == nil {
			return file, nil
		}
		if time.Now().After(deadline) {
			return nil, err
		}
		time.Sleep(200 * time.Millisecond)
	}
}
