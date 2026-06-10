package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
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
	JobRunning   JobState = "running"
	JobSuccess   JobState = "success"
	JobFailed    JobState = "failed"
	JobCancelled JobState = "cancelled"
)

type BuildJob struct {
	ID            string   `json:"id"`
	Recipes       []string `json:"recipes"`
	CurrentRecipe string   `json:"current_recipe"`
	State         JobState `json:"state"`
	StartedAt     int64    `json:"started_at"`
	FinishedAt    int64    `json:"finished_at"`
	ExitCode      int      `json:"exit_code"`
	LogPath       string   `json:"log_path"`
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

func (d *Dashboard) newJobID() string {
	return fmt.Sprintf("%d-%d", time.Now().Unix(), d.counter.Add(1))
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
	exe, err := os.Executable()
	if err != nil {
		exe = "ignite"
	}
	log, err := os.OpenFile(job.LogPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		job.State = JobFailed
		job.ExitCode = -1
		return
	}
	defer log.Close()
	overall := 0
	for _, recipeID := range job.Recipes {
		d.mu.Lock()
		job.CurrentRecipe = recipeID
		d.mu.Unlock()
		_, _ = fmt.Fprintf(log, "==> building %s\n", recipeID)
		cmd := exec.Command(exe, "-project-path", d.projectPath, "-cache-path", d.cachePath, "-arch", d.arch, "build", recipeID)
		cmd.Stdout = log
		cmd.Stderr = log
		cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
		if err := cmd.Start(); err != nil {
			overall = 1
			break
		}
		d.mu.Lock()
		job.pid = cmd.Process.Pid
		d.mu.Unlock()
		err := cmd.Wait()
		d.mu.Lock()
		job.pid = -1
		d.mu.Unlock()
		if err != nil {
			if exit, ok := err.(*exec.ExitError); ok {
				code := exit.ExitCode()
				if job.State == JobCancelled {
					break
				}
				overall = code
				_, _ = fmt.Fprintf(log, "==> build failed for %s (exit %d)\n", recipeID, code)
				break
			}
			overall = 1
			break
		}
		_, _ = fmt.Fprintf(log, "==> completed %s\n", recipeID)
	}
	job.ExitCode = overall
	if job.State != JobCancelled {
		if overall == 0 {
			job.State = JobSuccess
		} else {
			job.State = JobFailed
		}
	}
}

func (d *Dashboard) Run() int {
	go d.workerLoop()
	mux := http.NewServeMux()
	mux.HandleFunc("/api/status", d.handleStatus)
	mux.HandleFunc("/api/recipes", d.handleRecipes)
	mux.HandleFunc("/api/recipes/", d.handleRecipe)
	mux.HandleFunc("/api/builds", d.handleBuilds)
	mux.HandleFunc("/api/builds/", d.handleBuild)
	mux.HandleFunc("/api/fetch", d.handleFetch)
	mux.Handle("/", http.FileServer(http.Dir(d.assetsPath)))
	server := &http.Server{Addr: fmt.Sprintf("%s:%d", d.bindHost, d.port), Handler: cors(mux)}
	fmt.Printf("ignite dashboard listening on http://%s:%d\n", d.bindHost, d.port)
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
		next.ServeHTTP(w, r)
	})
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func (d *Dashboard) handleStatus(w http.ResponseWriter, r *http.Request) {
	total, cached, workspace, broken := 0, 0, 0, 0
	for _, recipe := range d.ignite.pool {
		total++
		copy := recipe
		hash, err := d.ignite.Hash(copy)
		if err != nil {
			broken++
			continue
		}
		copy.cache = hash
		if d.ignite.WorkspaceAvailable(copy) {
			workspace++
		} else if exists(d.ignite.CacheFile(copy)) {
			cached++
		}
	}
	writeJSON(w, 200, map[string]any{"total": total, "cached": cached, "workspace": workspace, "waiting": total - cached - workspace - broken, "broken": broken, "arch": d.arch, "project_path": d.projectPath, "cache_path": d.cachePath})
}

func (d *Dashboard) handleRecipes(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/api/recipes" {
		d.handleRecipe(w, r)
		return
	}
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
	limit := queryInt(r, "limit", 100)
	offset := queryInt(r, "offset", 0)

	var out []map[string]any
	for key, recipe := range d.ignite.pool {
		copy := recipe
		state := "unknown"
		note := ""
		if hash, err := d.ignite.Hash(copy); err != nil {
			note = err.Error()
		} else {
			copy.cache = hash
			if d.ignite.WorkspaceAvailable(copy) {
				state = "workspace"
			} else if exists(d.ignite.CacheFile(copy)) {
				state = "cached"
			} else {
				state = "waiting"
			}
		}
		if len(stateFilter) > 0 && !stateFilter[state] {
			continue
		}
		if query != "" && !strings.Contains(strings.ToLower(key), query) &&
			!strings.Contains(strings.ToLower(recipe.id), query) &&
			!strings.Contains(strings.ToLower(recipe.version), query) &&
			!strings.Contains(strings.ToLower(recipe.about), query) {
			continue
		}
		item := map[string]any{"id": key, "recipe_id": recipe.id, "version": recipe.version, "about": recipe.about, "state": state, "depends": recipe.depends}
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
	recipe, ok := d.ignite.pool[id]
	if !ok {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte("null"))
		return
	}
	copy := recipe
	state := "unknown"
	note := ""
	if hash, err := d.ignite.Hash(copy); err != nil {
		note = err.Error()
	} else {
		copy.cache = hash
	}
	_ = copy.ResolveSources(*d.ignite.config, nil)
	cached, workspace := false, false
	if copy.cache != "" {
		cached = exists(d.ignite.CacheFile(copy))
		workspace = d.ignite.WorkspaceAvailable(copy)
		if workspace {
			state = "workspace"
		} else if cached {
			state = "cached"
		} else {
			state = "waiting"
		}
	}
	out := map[string]any{"id": id, "recipe_id": copy.id, "element_id": copy.elementID, "version": copy.version, "about": copy.about, "cache": copy.cache, "state": state, "package_name": copy.PackageName(copy.elementID), "cache_file": d.ignite.CacheFile(copy), "depends": copy.depends, "build_time_depends": copy.buildTimeDepends, "sources": copy.sources, "backup": copy.backup, "integration": copy.integration}
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
			ai = out[i].FinishedAt
		}
		aj := out[j].StartedAt
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
		id := d.newJobID()
		job := &BuildJob{ID: id, Recipes: recipes, State: JobQueued, LogPath: filepath.Join(d.logRoot, id+".log"), pid: -1}
		d.mu.Lock()
		d.queue = append(d.queue, job)
		d.mu.Unlock()
		d.cond.Broadcast()
		writeJSON(w, 200, job)
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
	if err := d.ignite.FetchSources(recipes, false); err != nil {
		writeJSON(w, 500, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, 200, map[string]bool{"ok": true})
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
