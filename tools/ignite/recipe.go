package main

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

type Recipe struct {
	file                        string
	id, version, release, about string
	integration, cache          string
	depends, backup             []string
	config                      Config
	buildTimeDepends            []string
	sources                     []string
	elementID                   string
	arch                        string
}

func LoadRecipe(path, projectPath string, virtualFiles map[string][]byte) (Recipe, error) {
	r := Recipe{file: path, config: NewConfig()}
	r.config.searchPath = append(r.config.searchPath, projectPath)
	r.config.virtualFiles = virtualFiles
	r.config.SetString("cache", "none")
	if err := r.UpdateFromFile(path); err != nil {
		return r, err
	}
	r.buildTimeDepends = append(r.buildTimeDepends, r.config.StringSlice("build-depends")...)
	r.sources = append(r.sources, r.config.StringSlice("sources")...)
	if externalID, ok := externalRecipeID(path, projectPath); ok {
		r.elementID = externalID
	}
	return r, nil
}

// externalRecipeID returns the canonical recipe ID for a recipe stored in
// external/. recipe.yml is addressed as <name>; recipe.<sub>.yml as
// <name>:<sub>.
func externalRecipeID(file, projectPath string) (string, bool) {
	rel, err := filepath.Rel(filepath.Join(projectPath, "external"), file)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", false
	}
	dir := filepath.ToSlash(filepath.Dir(rel))
	if dir == "." {
		return "", false
	}
	base := filepath.Base(rel)
	if base == "recipe.yml" {
		return dir, true
	}
	if strings.HasPrefix(base, "recipe.") && strings.HasSuffix(base, ".yml") {
		sub := strings.TrimSuffix(strings.TrimPrefix(base, "recipe."), ".yml")
		if sub != "" {
			return dir + ":" + sub, true
		}
	}
	return "", false
}

// canonicalRecipeReference accepts canonical IDs as well as legacy external/
// IDs and on-disk recipe-file paths.
func canonicalRecipeReference(reference string) string {
	reference = strings.TrimPrefix(reference, "external/")
	slash := strings.LastIndexByte(reference, '/')
	if slash < 0 {
		return reference
	}
	dir, base := reference[:slash], reference[slash+1:]
	if base == "recipe.yml" {
		return dir
	}
	if strings.HasPrefix(base, "recipe.") && strings.HasSuffix(base, ".yml") {
		sub := strings.TrimSuffix(strings.TrimPrefix(base, "recipe."), ".yml")
		if sub != "" {
			return dir + ":" + sub
		}
	}
	return reference
}

func (r *Recipe) UpdateFromFile(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("failed to read file %q", path)
	}
	return r.UpdateFromData(data, path)
}

func (r *Recipe) UpdateFromData(data []byte, path string) error {
	if err := r.config.UpdateFrom(data, path); err != nil {
		return err
	}
	var err error
	if r.id, err = r.config.String("id"); err != nil {
		return err
	}
	if r.version, err = r.config.String("version"); err != nil {
		return err
	}
	r.about, _ = r.config.String("about", "")
	r.release, _ = r.config.String("release", "1")
	r.cache, _ = r.config.String("cache", "")
	r.depends = append(r.depends, r.config.StringSlice("depends")...)
	r.backup = append(r.backup, r.config.StringSlice("backup")...)
	r.integration, _ = r.config.String("integration", "")
	return nil
}

func (r Recipe) Name() string {
	return recipePathName(r.id)
}

func (r Recipe) String() string {
	var b strings.Builder
	fmt.Fprintf(&b, "id: %s\nversion: %s\nrelease: %s\nabout: %s\nbuild-id: %s\n", r.id, r.version, r.release, r.about, r.cache)
	if len(r.depends) > 0 {
		b.WriteString("depends:\n")
		for _, dep := range r.depends {
			b.WriteString("- " + strings.TrimSuffix(dep, filepath.Ext(dep)) + "\n")
		}
	}
	if len(r.backup) > 0 {
		b.WriteString("backup:\n")
		for _, item := range r.backup {
			b.WriteString("- " + item + "\n")
		}
	}
	if r.integration != "" {
		b.WriteString("script: |-\n")
		for _, line := range strings.Split(r.integration, "\n") {
			b.WriteString("  " + line + "\n")
		}
		b.WriteByte('\n')
	}
	return b.String()
}

func (r Recipe) PackageName(eid ...string) string {
	name := r.elementID
	if name == "" {
		name = r.id
	}
	if len(eid) > 0 && eid[0] != "" {
		name = eid[0]
	}
	name = packageNameIdentity(name)
	arch := r.arch
	if arch == "" {
		arch = "any"
	}
	return name + "-" + r.version + "-" + r.release + "-" + arch + ".pkg"
}

func recipePathName(name string) string {
	name = strings.ReplaceAll(name, "/", "-")
	name = strings.ReplaceAll(name, "\\", "-")
	return strings.ReplaceAll(name, ":", "-")
}

func packageNameIdentity(name string) string {
	return recipePathName(strings.TrimPrefix(name, "external/"))
}

func (r *Recipe) ResolveSources(global Config, extra map[string]string) error {
	for i, source := range r.sources {
		value, err := r.ResolveValue(source, global, extra)
		if err != nil {
			return err
		}
		r.sources[i] = value
	}
	return nil
}

func (r Recipe) ResolveValue(value string, global Config, extra map[string]string) (string, error) {
	vars := map[string]string{}
	for k, v := range global.ScalarMap("variables") {
		vars[k] = v
	}
	if r.config.node != nil && r.config.node.Kind == 4 {
		for i := 0; i+1 < len(r.config.node.Content); i += 2 {
			if r.config.node.Content[i+1].Kind == 8 {
				vars[r.config.node.Content[i].Value] = scalarString(r.config.node.Content[i+1])
			}
		}
	}
	for k, v := range r.config.ScalarMap("variables") {
		vars[k] = v
	}
	vars["build-dir"] = "_pkgupd_build_dir"
	for k, v := range extra {
		vars[k] = v
	}
	return resolveTemplate(value, vars)
}

var templatePattern = regexp.MustCompile(`%\{([^}]+)\}`)

func resolveTemplate(value string, vars map[string]string) (string, error) {
	result := value
	for {
		match := templatePattern.FindStringSubmatchIndex(result)
		if match == nil {
			return result, nil
		}
		name := result[match[2]:match[3]]
		repl, ok := vars[name]
		if !ok && strings.HasPrefix(name, "version:") {
			version, exists := vars["version"]
			if !exists {
				return "", fmt.Errorf("version variable not defined for %q", name)
			}
			spec := strings.TrimPrefix(name, "version:")
			if n, err := strconv.Atoi(spec); err == nil {
				pos := 0
				for count := 0; count <= n-1; count++ {
					next := strings.IndexByte(version[pos+1:], '.')
					if next < 0 {
						return "", fmt.Errorf("invalid variable value spliting for %dth position", n-1)
					}
					pos += next + 1
				}
				repl, ok = version[:pos], true
			} else {
				if spec == "" {
					return "", fmt.Errorf("empty version: specifier in %q", name)
				}
				repl, ok = strings.ReplaceAll(version, ".", spec[:1]), true
			}
		}
		if !ok {
			return "", fmt.Errorf("undefined variable %q", name)
		}
		var b bytes.Buffer
		b.WriteString(result[:match[0]])
		b.WriteString(repl)
		b.WriteString(result[match[1]:])
		result = b.String()
	}
}
