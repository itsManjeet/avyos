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
	projectPath     string
	cachePath       string
	arch            = "x86_64"
	force           bool
	dashboardPort   = 8080
	dashboardHost   = "127.0.0.1"
	dashboardAssets string
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
  workspace-finish <recipe> Export quilt patches and close the workspace
  dashboard                 Start a web dashboard to trigger and monitor builds

Options:
  -project-path <path>      Specify project path
  -cache-path <path>        Specify cache path
  -arch <arch>              Specify target device architecture (default: x86_64)
  -force                    Force refetch/update for supported commands
  -port <port>              Dashboard listen port (default: 8080)
  -host <host>              Dashboard bind host (default: 127.0.0.1)
  -assets <path>            Path to dashboard static assets`)
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
	recipe, err := findElementRecipe(ignite, args[0])
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
	recipe, err := findElementRecipe(ignite, args[0])
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
	if err := ignite.WorkspaceFinish(recipe); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	return 0
}

func dashboard(ignite *Ignite, _ []string) int {
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
	return NewDashboard(ignite, dashboardPort, dashboardHost, projectPath, cachePath, arch, assets).Run()
}

func main() {
	var fn commandFunc
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
			case "-arch":
				arch = require(arg)
			case "-force":
				force = true
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
			case "dashboard":
				fn = dashboard
			default:
				fmt.Fprintln(os.Stderr, "Unknown option:", arg)
				os.Exit(1)
			}
		} else {
			args = append(args, arg)
		}
	}
	if cachePath == "" {
		cachePath = filepath.Join(projectPath, "build", arch)
	}
	config := NewConfig()
	ignite, err := NewIgnite(&config, projectPath, cachePath, arch)
	if err != nil {
		fmt.Fprintln(os.Stderr, "ERROR:", err)
		os.Exit(1)
	}
	if fn == nil {
		os.Exit(help(ignite, args))
	}
	if err := ignite.Load(); err != nil {
		fmt.Fprintln(os.Stderr, "ERROR:", err)
		os.Exit(1)
	}
	os.Exit(fn(ignite, args))
}
