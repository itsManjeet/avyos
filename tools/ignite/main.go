package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type commandFunc func(*Ignite, []string) int

var (
	projectPath      string
	cachePath        string
	arch             = "x86_64"
	force            bool
	dashboardPort    = 8080
	dashboardHost    = "127.0.0.1"
	dashboardAssets  string
	serverURL        string
	workspacePath    string
	workspacePush    bool
	workspaceMessage string

	cachePathSet        bool
	workspacePathSet    bool
	workspacePushSet    bool
	workspaceMessageSet bool
	serverURLSet        bool
	archSet             bool
	forceSet            bool
)

func help(_ *Ignite, _ []string) int {
	fmt.Println(`Usage: ignite <options> <command> <args...>
Commands:
  build <recipes...>        Build artifact of specified recipes
  status <recipe>           Print if artifact is cached or need to build
  pull <recipe>             Pull artifact cache from artifact-url:
  cache-path <recipe>       Print the cache path of recipe
  checkout <recipe> <path>  Checkout artifact at <path>
  fetch [recipes...]        Fetch sources and write checksum.lock
  workspace <recipe>        Create an editable source workspace
  workspace-finish <recipe> Export patches or commit git workspace and close it
  server                    Start the long-running Ignite server/API/dashboard
  dashboard                 Start the web dashboard (alias of server UI)
  client <command> [...]    Send commands to a running Ignite server

Options:
  -project-path <path>      Specify project path
  -cache-path <path>        Specify cache path
  -workspace-path <path>    Specify editable source workspace path
  -workspace-push           Push git workspace commits during workspace-finish
  -workspace-message <msg>  Commit message for git workspace-finish
  -arch <arch>              Specify target device architecture (default: x86_64)
  -force                    Force refetch/update for supported commands
  -port <port>              Dashboard listen port (default: 8080)
  -host <host>              Dashboard bind host (default: 127.0.0.1)
  -assets <path>            Path to dashboard static assets
  -server-url <url>         Ignite server URL for client mode

If local.conf.yml exists in the project root, Ignite reads defaults from it.
Command-line options always override local.conf.yml.`)
	return 1
}

func findRecipe(ignite *Ignite, component string) (Recipe, error) {
	candidates := []string{component}
	if !strings.HasSuffix(component, ".yml") {
		candidates = append(candidates, component+".yml")
	}
	if !strings.HasPrefix(component, "components/") {
		candidates = append(candidates, "components/"+component)
		if !strings.HasSuffix(component, ".yml") {
			candidates = append(candidates, "components/"+component+".yml")
		}
	}
	for _, candidate := range candidates {
		if recipe, ok := ignite.pool[candidate]; ok {
			return recipe, nil
		}
	}
	var found *Recipe
	for _, recipe := range ignite.pool {
		if recipe.id != component {
			continue
		}
		copy := recipe
		if found != nil {
			return Recipe{}, fmt.Errorf("multiple recipes found with id %q", component)
		}
		found = &copy
	}
	if found != nil {
		return *found, nil
	}
	return Recipe{}, fmt.Errorf("no recipe found with id %q", component)
}

func findElementRecipe(ignite *Ignite, component string) (Recipe, error) {
	if recipe, ok := ignite.pool[component]; ok {
		return recipe, nil
	}
	return Recipe{}, fmt.Errorf("no recipe found with element id %q", component)
}

func pull(ignite *Ignite, args []string) int {
	states, err := ignite.Resolve(args, true, true, true)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	artifactURL := configString(*ignite.config, "artifact-url", "https://repo.avyos.dev")
	for _, state := range states {
		recipe := state.recipe
		if ignite.WorkspaceAvailable(recipe) {
			fmt.Println("SKIP workspace active for", state.id)
			continue
		}
		if !state.cached {
			if err := recipe.ResolveSources(*ignite.config, nil); err != nil {
				fmt.Fprintln(os.Stderr, err)
				return 1
			}
			serverURL := artifactURL + "/cache/" + recipe.PackageName(recipe.elementID)
			cacheFilePath := ignite.CacheFile(recipe)
			fmt.Println("GET", serverURL)
			status := NewExecutor("/bin/curl").Arg("-C").Arg("-").Arg(serverURL).Arg("-o").Arg(cacheFilePath).Run()
			if status != 0 {
				fmt.Fprintln(os.Stderr, "Error:", status)
				return 1
			}
		}
	}
	return 0
}

func cachepath(ignite *Ignite, args []string) int {
	if len(args) != 1 {
		fmt.Fprintln(os.Stderr, "require exactly one argument")
		return 1
	}
	recipe, err := findRecipe(ignite, args[0])
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	recipe.cache, err = ignite.Hash(recipe)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	fmt.Println(ignite.CacheFile(recipe))
	return 0
}

func checkout(ignite *Ignite, args []string) int {
	if len(args) != 2 {
		fmt.Fprintln(os.Stderr, "require exactly two arguments: <recipe> <path>")
		return 1
	}
	recipe, ok := ignite.pool[args[0]]
	if !ok {
		fmt.Fprintf(os.Stderr, "no recipe found with id %q\n", args[0])
		return 1
	}
	var err error
	recipe.cache, err = ignite.Hash(recipe)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	_ = os.MkdirAll(args[1], 0755)
	return NewExecutor("/bin/tar").Arg("-xf").Arg(ignite.CacheFile(recipe)).Arg("-C").Arg(args[1]).Run()
}

func fetch(ignite *Ignite, args []string) int {
	if err := ignite.FetchSources(args, force); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	return 0
}

func build(ignite *Ignite, args []string) int {
	states, err := ignite.Resolve(args, true, true, true)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	for _, state := range states {
		if state.cached {
			continue
		}
		recipe := state.recipe
		if err := recipe.ResolveSources(*ignite.config, nil); err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		fmt.Println("building", state.id)
		if err := ignite.Build(recipe); err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
	}
	return 0
}

func buildOne(ignite *Ignite, args []string) int {
	if len(args) != 1 {
		fmt.Fprintln(os.Stderr, "require exactly one argument: <recipe>")
		return 1
	}
	recipe, err := findRecipe(ignite, args[0])
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	recipe.cache, err = ignite.Hash(recipe)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	if err := recipe.ResolveSources(*ignite.config, nil); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	fmt.Println("building", elementName(recipe))
	if err := ignite.Build(recipe); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	return 0
}

func status(ignite *Ignite, args []string) int {
	states, err := ignite.Resolve(args, true, true, true)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	totalCached := 0
	for _, state := range states {
		label := "WAITING  "
		if ignite.WorkspaceAvailable(state.recipe) {
			label = "WORKSPACE"
		} else if state.cached {
			label = "CACHED   "
		}
		fmt.Printf("  %s  %s\n", label, state.id)
		if state.cached {
			totalCached++
		}
	}
	fmt.Printf("\n  TOTAL COMPONENTS : %d\n  TOTAL CACHED     : %d\n  NEED TO BUILD    : %d\n", len(states), totalCached, len(states)-totalCached)
	return 0
}

func workspace(ignite *Ignite, args []string) int {
	if len(args) != 1 {
		fmt.Fprintln(os.Stderr, "require exactly one argument: <recipe>")
		return 1
	}
	recipe, err := findRecipe(ignite, args[0])
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	recipe.cache, err = ignite.Hash(recipe)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	if err := recipe.ResolveSources(*ignite.config, nil); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	if err := ignite.WorkspaceInit(recipe); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	return 0
}

func workspaceFinish(ignite *Ignite, args []string) int {
	if len(args) != 1 {
		fmt.Fprintln(os.Stderr, "require exactly one argument: <recipe>")
		return 1
	}
	recipe, err := findRecipe(ignite, args[0])
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	recipe.cache, err = ignite.Hash(recipe)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	if err := recipe.ResolveSources(*ignite.config, nil); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	if err := ignite.WorkspaceFinish(recipe, WorkspaceFinishOptions{Message: workspaceMessage, Push: workspacePush}); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	return 0
}

func loadLocalConfig(projectPath string) (Config, bool, error) {
	path := filepath.Join(projectPath, "local.conf.yml")
	if !exists(path) {
		return NewConfig(), false, nil
	}
	config := NewConfig()
	if err := config.UpdateFromFile(path); err != nil {
		return config, false, err
	}
	return config, true, nil
}

func localString(config Config, key, fallback string) string {
	value, err := config.String(key, fallback)
	if err != nil {
		return fallback
	}
	return value
}

func dashboardAssetsPath() string {
	assets := dashboardAssets
	if assets == "" {
		if exe, err := os.Executable(); err == nil {
			exeDir := filepath.Dir(exe)
			for _, candidate := range []string{
				filepath.Join(exeDir, "dashboard"),
				filepath.Join(exeDir, "..", "share", "ignite", "dashboard"),
				filepath.Join(projectPath, "tools", "ignite", "dashboard"),
			} {
				if exists(filepath.Join(candidate, "index.html")) {
					assets, _ = filepath.Abs(candidate)
					break
				}
			}
		}
		if assets == "" {
			assets = filepath.Join(projectPath, "tools", "ignite", "dashboard")
		}
	}
	return assets
}

func dashboard(ignite *Ignite, _ []string) int {
	d := NewDashboard(ignite, dashboardPort, dashboardHost, projectPath, cachePath, arch, dashboardAssetsPath())
	return d.Run()
}

func server(ignite *Ignite, _ []string) int {
	d := NewDashboard(ignite, dashboardPort, dashboardHost, projectPath, cachePath, arch, dashboardAssetsPath())
	d.EnableRecipeWatcher(true)
	return d.Run()
}

func main() {
	var fn commandFunc
	skipRecipeLoad := false
	args := []string{}
	var err error
	projectPath, err = os.Getwd()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	for idx := 1; idx < len(os.Args); idx++ {
		arg := os.Args[idx]
		if strings.HasPrefix(arg, "-") {
			require := func(opt string) string {
				if idx+1 >= len(os.Args) {
					fmt.Fprintln(os.Stderr, "Option", opt, "requires an argument")
					os.Exit(1)
				}
				idx++
				return os.Args[idx]
			}
			switch arg {
			case "-project-path":
				projectPath = require(arg)
			case "-cache-path":
				cachePath = require(arg)
				cachePathSet = true
			case "-workspace-path":
				workspacePath = require(arg)
				workspacePathSet = true
			case "-workspace-push":
				workspacePush = true
				workspacePushSet = true
			case "-workspace-message":
				workspaceMessage = require(arg)
				workspaceMessageSet = true
			case "-arch":
				arch = require(arg)
				archSet = true
			case "-force":
				force = true
				forceSet = true
			case "-port":
				dashboardPort, err = strconv.Atoi(require(arg))
				if err != nil {
					fmt.Fprintln(os.Stderr, err)
					os.Exit(1)
				}
			case "-host":
				dashboardHost = require(arg)
			case "-assets":
				dashboardAssets = require(arg)
			case "-server-url":
				serverURL = require(arg)
				serverURLSet = true
			default:
				fmt.Fprintln(os.Stderr, "Unknown option:", arg)
				os.Exit(1)
			}
			continue
		}
		if fn == nil {
			switch arg {
			case "build":
				fn = build
			case "build-one":
				fn = buildOne
			case "help":
				fn = help
			case "status":
				fn = status
			case "pull":
				fn = pull
			case "cache-path":
				fn = cachepath
			case "checkout":
				fn = checkout
			case "fetch":
				fn = fetch
			case "workspace":
				fn = workspace
			case "workspace-finish":
				fn = workspaceFinish
			case "server":
				fn = server
			case "dashboard":
				fn = dashboard
			case "client":
				fn = client
				skipRecipeLoad = true
			default:
				fmt.Fprintln(os.Stderr, "Unknown option:", arg)
				os.Exit(1)
			}
		} else {
			args = append(args, arg)
		}
	}
	localConfig, localLoaded, err := loadLocalConfig(projectPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "ERROR:", err)
		os.Exit(1)
	}
	if localLoaded {
		if !archSet {
			arch = localString(localConfig, "arch", arch)
		}
		if !cachePathSet {
			cachePath = localString(localConfig, "cache-path", cachePath)
		}
		if !workspacePathSet {
			workspacePath = localString(localConfig, "workspace-path", workspacePath)
		}
		if !workspacePushSet {
			workspacePush = localConfig.Bool("workspace-push", workspacePush)
		}
		if !workspaceMessageSet {
			workspaceMessage = localString(localConfig, "workspace-message", workspaceMessage)
		}
		if !serverURLSet {
			serverURL = localString(localConfig, "server-url", serverURL)
		}
		if !forceSet {
			force = localConfig.Bool("force", force)
		}
	}
	if cachePath == "" {
		cachePath = filepath.Join(projectPath, "build", arch)
	}
	if serverURL == "" {
		serverURL = fmt.Sprintf("http://%s:%d", dashboardHost, dashboardPort)
	}
	config := NewConfig()
	if localLoaded {
		config = localConfig
	}
	ignite, err := NewIgnite(&config, projectPath, cachePath, workspacePath, arch)
	if err != nil {
		fmt.Fprintln(os.Stderr, "ERROR:", err)
		os.Exit(1)
	}
	if fn == nil {
		os.Exit(help(ignite, args))
	}
	if !skipRecipeLoad {
		if err := ignite.Load(); err != nil {
			fmt.Fprintln(os.Stderr, "ERROR:", err)
			os.Exit(1)
		}
	}
	os.Exit(fn(ignite, args))
}
