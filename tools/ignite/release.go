package main

import (
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
)

var recipeReleaseLine = regexp.MustCompile(`(?m)^release:[^\r\n]*$`)
var recipeVersionLine = regexp.MustCompile(`(?m)^version:[^\r\n]*(?:\r?\n|$)`)

// bumpReleaseForRebuild creates a new distribution revision before replacing
// an existing non-workspace artifact. The revision is stored in the recipe so
// published package names remain stable and usable by remote installers.
func (i *Ignite) bumpReleaseForRebuild(recipe Recipe) (Recipe, error) {
	if i.WorkspaceAvailable(recipe) || !exists(i.CacheFile(recipe)) {
		return recipe, nil
	}
	current, err := strconv.Atoi(recipe.release)
	if err != nil || current < 1 {
		return Recipe{}, fmt.Errorf("cannot automatically bump non-numeric release %q for %s", recipe.release, recipe.elementID)
	}
	next := strconv.Itoa(current + 1)
	if recipe.file == "" {
		return Recipe{}, fmt.Errorf("cannot update release for %s because its recipe file is unknown", recipe.elementID)
	}
	if err := updateRecipeRelease(recipe.file, next); err != nil {
		return Recipe{}, err
	}

	i.hashMu.Lock()
	delete(i.hashCache, recipe.elementID)
	i.hashMu.Unlock()

	updated, err := LoadRecipe(recipe.file, i.projectPath, i.virtualMergeFiles())
	if err != nil {
		return Recipe{}, err
	}
	updated.arch = i.arch
	updated.cache, err = i.Hash(updated)
	if err != nil {
		return Recipe{}, err
	}
	i.pool[updated.elementID] = updated
	return updated, nil
}

func updateRecipeRelease(path, release string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	text := string(data)
	if recipeReleaseLine.MatchString(text) {
		text = recipeReleaseLine.ReplaceAllString(text, "release: "+release)
	} else if location := recipeVersionLine.FindStringIndex(text); location != nil {
		text = text[:location[1]] + "release: " + release + "\n" + text[location[1]:]
	} else {
		text = strings.TrimRight(text, "\r\n") + "\nrelease: " + release + "\n"
	}
	return os.WriteFile(path, []byte(text), 0644)
}
