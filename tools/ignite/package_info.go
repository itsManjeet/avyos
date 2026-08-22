package main

import (
	"debug/elf"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// PackageInfo is the distribution metadata embedded as /INFO in every
// runtime package. BuildID identifies the exact recipe inputs used to build
// the package, while ABI identifies the exported shared-library interface.
type PackageInfo struct {
	Format        int               `yaml:"format"`
	ID            string            `yaml:"id"`
	Element       string            `yaml:"element"`
	Version       string            `yaml:"version"`
	Release       string            `yaml:"release"`
	Arch          string            `yaml:"arch"`
	About         string            `yaml:"about,omitempty"`
	Integration   string            `yaml:"integration,omitempty"`
	Depends       []string          `yaml:"depends,omitempty"`
	BuildDepends  []string          `yaml:"build-depends,omitempty"`
	Backup        []string          `yaml:"backup,omitempty"`
	Provides      []string          `yaml:"provides,omitempty"`
	Requires      []string          `yaml:"requires,omitempty"`
	ABI           string            `yaml:"abi"`
	BuildID       string            `yaml:"build-id"`
	DependencyABI map[string]string `yaml:"dependency-abi,omitempty"`
}

// CacheState explains whether a package can be reused without rebuilding.
// Reason is suitable for display in the CLI.
type CacheState struct {
	Cached bool
	Reason string
}

func (i *Ignite) PackageCacheState(recipe Recipe) (CacheState, error) {
	if i.WorkspaceAvailable(recipe) {
		return CacheState{Reason: "local workspace is active"}, nil
	}
	if !exists(i.CacheFile(recipe)) {
		return CacheState{Reason: "package artifact is missing"}, nil
	}
	info, err := readPackageInfo(i.CacheFile(recipe))
	if err != nil {
		return CacheState{Reason: "package metadata is missing or invalid"}, nil
	}
	if info.Format != 1 {
		return CacheState{Reason: "package metadata format is unsupported"}, nil
	}
	if info.ID != recipe.id || info.Element != recipe.elementID {
		return CacheState{Reason: "package identity changed"}, nil
	}
	if info.Version != recipe.version || info.Release != recipe.release || info.Arch != recipe.arch {
		return CacheState{Reason: "package version, release, or architecture changed"}, nil
	}
	if info.BuildID != recipe.cache {
		return CacheState{Reason: "recipe, merge, or source inputs changed"}, nil
	}
	for dependency, expectedABI := range info.DependencyABI {
		dependency = canonicalRecipeReference(dependency)
		depRecipe, ok := i.pool[dependency]
		if !ok {
			return CacheState{}, fmt.Errorf("package %q references missing dependency %q", recipe.id, dependency)
		}
		depRecipe.cache, err = i.Hash(depRecipe)
		if err != nil {
			return CacheState{}, err
		}
		if !exists(i.CacheFile(depRecipe)) {
			return CacheState{Reason: fmt.Sprintf("dependency %q package is missing", dependency)}, nil
		}
		depInfo, err := readPackageInfo(i.CacheFile(depRecipe))
		if err != nil {
			return CacheState{Reason: fmt.Sprintf("dependency %q metadata is missing or invalid", dependency)}, nil
		}
		if depInfo.ABI != expectedABI {
			return CacheState{Reason: fmt.Sprintf("dependency %q ABI changed", dependency)}, nil
		}
	}
	return CacheState{Cached: true, Reason: "package inputs and dependency ABIs are compatible"}, nil
}

func (i *Ignite) PackageCached(recipe Recipe) (bool, error) {
	state, err := i.PackageCacheState(recipe)
	return state.Cached, err
}

func (i *Ignite) packageInfo(recipe Recipe, root string) (PackageInfo, error) {
	provides, requires, err := elfMetadata(root)
	if err != nil {
		return PackageInfo{}, err
	}
	dependencyABI := map[string]string{}
	depends := make([]string, 0, len(recipe.depends))
	for _, dependency := range recipe.depends {
		dependency = canonicalRecipeReference(dependency)
		depends = append(depends, dependency)
		depRecipe, ok := i.pool[dependency]
		if !ok {
			return PackageInfo{}, fmt.Errorf("missing runtime dependency %q for %s", dependency, recipe.id)
		}
		depRecipe.cache, err = i.Hash(depRecipe)
		if err != nil {
			return PackageInfo{}, err
		}
		depInfo, err := readPackageInfo(i.CacheFile(depRecipe))
		if err != nil {
			return PackageInfo{}, fmt.Errorf("failed to read dependency metadata for %q: %w", dependency, err)
		}
		dependencyABI[dependency] = depInfo.ABI
	}
	buildDepends := make([]string, 0, len(recipe.buildTimeDepends))
	for _, dependency := range recipe.buildTimeDepends {
		buildDepends = append(buildDepends, canonicalRecipeReference(dependency))
	}
	integration := recipe.integration
	if integration != "" {
		var err error
		integration, err = recipe.ResolveValue(integration, *i.config, nil)
		if err != nil {
			return PackageInfo{}, err
		}
	}
	return PackageInfo{
		Format:        1,
		ID:            recipe.id,
		Element:       recipe.elementID,
		Version:       recipe.version,
		Release:       recipe.release,
		Arch:          recipe.arch,
		About:         recipe.about,
		Integration:   integration,
		Depends:       depends,
		BuildDepends:  buildDepends,
		Backup:        append([]string(nil), recipe.backup...),
		Provides:      provides,
		Requires:      requires,
		ABI:           abiFingerprint(provides),
		BuildID:       recipe.cache,
		DependencyABI: dependencyABI,
	}, nil
}

func writePackageInfo(path string, info PackageInfo) error {
	data, err := yaml.Marshal(info)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

func readPackageInfo(packagePath string) (PackageInfo, error) {
	status, out := NewExecutor("/bin/tar").Arg("-xOf").Arg(packagePath).Arg("./INFO").Output()
	if status != 0 {
		return PackageInfo{}, fmt.Errorf("failed to read INFO from %q", packagePath)
	}
	var info PackageInfo
	if err := yaml.Unmarshal([]byte(out), &info); err != nil {
		return PackageInfo{}, fmt.Errorf("failed to parse INFO from %q: %w", packagePath, err)
	}
	return info, nil
}

func elfMetadata(root string) ([]string, []string, error) {
	provides := map[string]bool{}
	requires := map[string]bool{}
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return err
		}
		file, err := elf.Open(path)
		if err != nil {
			return nil
		}
		defer file.Close()
		if names, err := file.DynString(elf.DT_SONAME); err == nil {
			for _, name := range names {
				provides[name] = true
			}
		}
		if names, err := file.DynString(elf.DT_NEEDED); err == nil {
			for _, name := range names {
				requires[name] = true
			}
		}
		return nil
	})
	if err != nil {
		return nil, nil, err
	}
	return sortedSet(provides), sortedSet(requires), nil
}

func abiFingerprint(provides []string) string {
	return sha256Hex([]byte(strings.Join(provides, "\n")))
}

func sortedSet(values map[string]bool) []string {
	items := make([]string, 0, len(values))
	for value := range values {
		items = append(items, value)
	}
	sort.Strings(items)
	return items
}
