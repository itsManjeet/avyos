package main

import (
	"archive/tar"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestResolveTemplateVersionForms(t *testing.T) {
	vars := map[string]string{"version": "1.2.3", "name": "pkg"}
	got, err := resolveTemplate("%{name}-%{version:2}-%{version:_}", vars)
	if err != nil {
		t.Fatal(err)
	}
	if got != "pkg-1.2-1_2_3" {
		t.Fatalf("unexpected expansion: %q", got)
	}
}

func TestParseSourceSpec(t *testing.T) {
	spec, err := parseSourceSpec("archive.tar.xz::noextract::https://example.invalid/src.tar.xz")
	if err != nil {
		t.Fatal(err)
	}
	if spec.filename != "archive.tar.xz" || spec.url != "https://example.invalid/src.tar.xz" || !spec.noextract {
		t.Fatalf("unexpected source spec: %#v", spec)
	}
}

func TestPackageNameUsesVersionReleaseAndArchitecture(t *testing.T) {
	recipe := Recipe{id: "demo", elementID: "demo:docs", version: "1.2.3", release: "4", arch: "x86_64"}
	if got, want := recipe.PackageName(), "demo-docs-1.2.3-4-x86_64.pkg"; got != want {
		t.Fatalf("unexpected package name: got %q, want %q", got, want)
	}
	if got, want := recipe.PackageName("demo"), "demo-1.2.3-4-x86_64.pkg"; got != want {
		t.Fatalf("unexpected package name: got %q, want %q", got, want)
	}
}

func TestABIFingerprintChangesWithSONAME(t *testing.T) {
	left := abiFingerprint([]string{"libalpha.so.1", "libbeta.so.2"})
	right := abiFingerprint([]string{"libalpha.so.1", "libbeta.so.2"})
	if left != right {
		t.Fatalf("ABI fingerprint must be stable: %q != %q", left, right)
	}
	if left == abiFingerprint([]string{"libalpha.so.2", "libbeta.so.2"}) {
		t.Fatal("SONAME change did not change ABI fingerprint")
	}
}

func TestPackageCacheOnlyInvalidatesDependentsForABIChanges(t *testing.T) {
	tmp := t.TempDir()
	ignite := &Ignite{config: &Config{}, projectPath: tmp, cachePath: tmp, workspacePath: filepath.Join(tmp, "workspaces"), pool: map[string]Recipe{}, hashCache: map[string]string{}}
	dependency := Recipe{id: "libdemo", version: "1", release: "1", arch: "x86_64", elementID: "libdemo", config: NewConfig()}
	consumer := Recipe{id: "demo", version: "1", release: "1", arch: "x86_64", elementID: "demo", cache: "consumer-build", depends: []string{"libdemo"}, config: NewConfig()}
	ignite.pool[dependency.elementID] = dependency
	if err := writeTestPackageInfo(ignite.CacheFile(dependency), PackageInfo{Format: 1, ID: dependency.id, Element: dependency.elementID, Version: dependency.version, Release: dependency.release, Arch: dependency.arch, ABI: "abi-v1", BuildID: "dependency-build-v1"}); err != nil {
		t.Fatal(err)
	}
	consumerInfo := PackageInfo{Format: 1, ID: consumer.id, Element: consumer.elementID, Version: consumer.version, Release: consumer.release, Arch: consumer.arch, ABI: "consumer-abi", BuildID: consumer.cache, DependencyABI: map[string]string{dependency.elementID: "abi-v1"}}
	if err := writeTestPackageInfo(ignite.CacheFile(consumer), consumerInfo); err != nil {
		t.Fatal(err)
	}
	if cached, err := ignite.PackageCached(consumer); err != nil || !cached {
		t.Fatalf("expected compatible dependency to retain cached package: cached=%t err=%v", cached, err)
	}
	if err := writeTestPackageInfo(ignite.CacheFile(dependency), PackageInfo{Format: 1, ID: dependency.id, Element: dependency.elementID, Version: dependency.version, Release: dependency.release, Arch: dependency.arch, ABI: "abi-v1", BuildID: "dependency-build-v2"}); err != nil {
		t.Fatal(err)
	}
	if cached, err := ignite.PackageCached(consumer); err != nil || !cached {
		t.Fatalf("dependency rebuild with stable ABI must not invalidate consumer: cached=%t err=%v", cached, err)
	}
	if err := writeTestPackageInfo(ignite.CacheFile(dependency), PackageInfo{Format: 1, ID: dependency.id, Element: dependency.elementID, Version: dependency.version, Release: dependency.release, Arch: dependency.arch, ABI: "abi-v2", BuildID: "dependency-build-v3"}); err != nil {
		t.Fatal(err)
	}
	state, err := ignite.PackageCacheState(consumer)
	if err != nil || state.Cached {
		t.Fatalf("dependency ABI change must invalidate consumer: state=%#v err=%v", state, err)
	}
	if want := `dependency "libdemo" ABI changed`; state.Reason != want {
		t.Fatalf("unexpected ABI invalidation reason: got %q, want %q", state.Reason, want)
	}
}

func TestBumpReleaseForRebuildPersistsNewPackageRevision(t *testing.T) {
	project := t.TempDir()
	path := filepath.Join(project, "external", "demo", "recipe.yml")
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("id: demo\nversion: 1.0\nrelease: 3\n"), 0644); err != nil {
		t.Fatal(err)
	}
	config := NewConfig()
	ignite := &Ignite{config: &config, projectPath: project, cachePath: project, workspacePath: filepath.Join(project, "workspaces"), arch: "x86_64", pool: map[string]Recipe{}, hashCache: map[string]string{}}
	recipe, err := LoadRecipe(path, project, nil)
	if err != nil {
		t.Fatal(err)
	}
	recipe.arch = ignite.arch
	recipe.cache, err = ignite.Hash(recipe)
	if err != nil {
		t.Fatal(err)
	}
	if err := writeTestPackageInfo(ignite.CacheFile(recipe), PackageInfo{Format: 1, ID: recipe.id, Element: recipe.elementID, Version: recipe.version, Release: recipe.release, Arch: recipe.arch, BuildID: recipe.cache}); err != nil {
		t.Fatal(err)
	}
	updated, err := ignite.bumpReleaseForRebuild(recipe)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := updated.release, "4"; got != want {
		t.Fatalf("unexpected release: got %q, want %q", got, want)
	}
	if got, want := updated.PackageName(), "demo-1.0-4-x86_64.pkg"; got != want {
		t.Fatalf("unexpected package name: got %q, want %q", got, want)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "id: demo\nversion: 1.0\nrelease: 4\n" {
		t.Fatalf("unexpected updated recipe:\n%s", data)
	}
}

func TestUpdateRecipeReleaseAddsMissingRelease(t *testing.T) {
	path := filepath.Join(t.TempDir(), "recipe.yml")
	if err := os.WriteFile(path, []byte("id: demo\nversion: 1.0\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := updateRecipeRelease(path, "2"); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "id: demo\nversion: 1.0\nrelease: 2\n" {
		t.Fatalf("unexpected updated recipe:\n%s", data)
	}
}

func TestFetchSourcesContinuesAfterSourceFailure(t *testing.T) {
	project := t.TempDir()
	if err := os.WriteFile(filepath.Join(project, "available.txt"), []byte("available"), 0644); err != nil {
		t.Fatal(err)
	}
	config := NewConfig()
	ignite := &Ignite{
		config:      &config,
		projectPath: project,
		cachePath:   filepath.Join(project, "cache"),
		pool: map[string]Recipe{
			"broken": {id: "broken", elementID: "broken", sources: []string{"missing.txt"}, config: NewConfig()},
			"valid":  {id: "valid", elementID: "valid", sources: []string{"available.txt"}, config: NewConfig()},
		},
	}
	err := ignite.FetchSources(nil, false)
	fetchErr, ok := err.(*FetchError)
	if !ok {
		t.Fatalf("expected FetchError, got %T (%v)", err, err)
	}
	if len(fetchErr.Failures) != 1 || fetchErr.Failures[0].Recipe != "broken" || fetchErr.Failures[0].Source != "missing.txt" {
		t.Fatalf("unexpected fetch failures: %#v", fetchErr.Failures)
	}
	if !strings.Contains(fetchErr.Error(), "broken: missing.txt:") {
		t.Fatalf("fetch summary does not include the failed source: %s", fetchErr)
	}
	if !exists(filepath.Join(ignite.cachePath, "sources", "available.txt")) {
		t.Fatal("valid source was not fetched after another source failed")
	}
	checksums, err := readChecksumLock(filepath.Join(project, "checksum.lock"))
	if err != nil {
		t.Fatal(err)
	}
	if checksums["available.txt"] == "" {
		t.Fatal("successful source was not checkpointed in checksum.lock")
	}
}

func writeTestPackageInfo(path string, info PackageInfo) error {
	data, err := yaml.Marshal(info)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()
	archive := tar.NewWriter(file)
	defer archive.Close()
	if err := archive.WriteHeader(&tar.Header{Name: "./INFO", Mode: 0644, Size: int64(len(data))}); err != nil {
		return err
	}
	_, err = archive.Write(data)
	return err
}

func TestLoadExternalRecipe(t *testing.T) {
	project := t.TempDir()
	path := filepath.Join(project, "external", "pkg", "recipe.yml")
	variantPath := filepath.Join(project, "external", "pkg", "recipe.docs.yml")
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("id: pkg\nversion: 1\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(variantPath, []byte("id: pkg-docs\nversion: 1\n"), 0644); err != nil {
		t.Fatal(err)
	}
	ignite := &Ignite{config: &Config{}, projectPath: project, pool: map[string]Recipe{}, hashCache: map[string]string{}}
	if err := ignite.Load(); err != nil {
		t.Fatal(err)
	}
	recipe, ok := ignite.pool["pkg"]
	if !ok {
		t.Fatal("external recipe was not loaded")
	}
	if recipe.elementID != "pkg" {
		t.Fatalf("unexpected external element id: %q", recipe.elementID)
	}
	if found, err := findRecipe(ignite, "pkg"); err != nil || found.file != path {
		t.Fatalf("failed to find external recipe: recipe=%#v err=%v", found, err)
	}
	if found, err := findRecipe(ignite, "pkg/recipe.docs.yml"); err != nil || found.file != variantPath || found.elementID != "pkg:docs" {
		t.Fatalf("failed to find external recipe variant: recipe=%#v err=%v", found, err)
	}
	if found, err := findRecipe(ignite, "pkg:docs"); err != nil || found.file != variantPath {
		t.Fatalf("failed to find canonical external recipe variant: recipe=%#v err=%v", found, err)
	}
}

func TestApplyPatchFileHandlesBundledSequentialPatches(t *testing.T) {
	tmp := t.TempDir()
	root := filepath.Join(tmp, "root")
	nested := filepath.Join(root, "pkg-1.0")
	if err := os.MkdirAll(nested, 0755); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(nested, "hello.txt")
	if err := os.WriteFile(file, []byte("one\n"), 0644); err != nil {
		t.Fatal(err)
	}
	patch := filepath.Join(tmp, "bundle.patch")
	data := `From 1111111111111111111111111111111111111111 Mon Sep 17 00:00:00 2001
Subject: [PATCH 1/2] one to two
---
 hello.txt | 2 +-
 1 file changed, 1 insertion(+), 1 deletion(-)

diff --git a/hello.txt b/hello.txt
--- a/hello.txt
+++ b/hello.txt
@@ -1 +1 @@
-one
+two

From 2222222222222222222222222222222222222222 Mon Sep 17 00:00:00 2001
Subject: [PATCH 2/2] two to three
---
 hello.txt | 2 +-
 1 file changed, 1 insertion(+), 1 deletion(-)

diff --git a/hello.txt b/hello.txt
--- a/hello.txt
+++ b/hello.txt
@@ -1 +1 @@
-two
+three
`
	if err := os.WriteFile(patch, []byte(data), 0644); err != nil {
		t.Fatal(err)
	}
	if err := applyPatchFile(patch, root); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "three\n" {
		t.Fatalf("patch bundle was not applied sequentially: %q", got)
	}
}

func TestAppendSourcesToRecipeTextExistingBlock(t *testing.T) {
	input := "id: pkg\nversion: 1\nsources:\n  - https://example.invalid/pkg.tar.xz\nscript: |\n  true\n"
	got := appendSourcesToRecipeText(input, []string{"external/pkg/0001-fix.patch", "external/pkg/0002-next.patch"})
	want := "id: pkg\nversion: 1\nsources:\n  - https://example.invalid/pkg.tar.xz\n  - external/pkg/0001-fix.patch\n  - external/pkg/0002-next.patch\nscript: |\n  true\n"
	if got != want {
		t.Fatalf("unexpected recipe text:\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
}

func TestAppendSourcesToRecipeTextNewBlock(t *testing.T) {
	input := "id: pkg\nversion: 1\n"
	got := appendSourcesToRecipeText(input, []string{"external/pkg/0001-fix.patch"})
	want := "id: pkg\nversion: 1\n\nsources:\n  - external/pkg/0001-fix.patch\n"
	if got != want {
		t.Fatalf("unexpected recipe text:\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
}

func TestAppendSourcesToRecipeFileSkipsDuplicate(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "pkg.yml")
	input := "id: pkg\nversion: 1\nsources:\n  - external/pkg/0001-fix.patch\n"
	if err := os.WriteFile(path, []byte(input), 0644); err != nil {
		t.Fatal(err)
	}
	if err := appendSourcesToRecipeFile(path, nil, []string{"external/pkg/0001-fix.patch"}); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != input {
		t.Fatalf("duplicate was appended:\n%s", got)
	}
}

func TestParseGitSourceSpec(t *testing.T) {
	spec, err := parseSourceSpec("demo::git+https://example.invalid/org/demo.git#main")
	if err != nil {
		t.Fatal(err)
	}
	if !spec.IsGit() || spec.filename != "demo" || spec.GitRef() != "main" {
		t.Fatalf("unexpected git source spec: %#v ref=%q", spec, spec.GitRef())
	}
	remote, err := spec.GitRemote()
	if err != nil {
		t.Fatal(err)
	}
	if remote != "https://example.invalid/org/demo.git" {
		t.Fatalf("unexpected git remote: %q", remote)
	}
}

func TestGitSourceSpecInfersName(t *testing.T) {
	spec, err := parseSourceSpec("git+https://example.invalid/org/demo.git?ref=main")
	if err != nil {
		t.Fatal(err)
	}
	if spec.filename != "demo" || spec.GitRef() != "main" {
		t.Fatalf("unexpected inferred git source spec: %#v ref=%q", spec, spec.GitRef())
	}
}

func TestExecutorContainerAlwaysWrapsCommand(t *testing.T) {
	container := Container{
		hostRoot: "/tmp/ignite-root",
		environ:  []string{"HOME=/"},
	}
	ex := NewExecutor("/bin/true").Path("/build-root").Container(&container)
	if len(ex.args) == 0 || ex.args[0] != "/bin/bwrap" {
		t.Fatalf("expected containerized command, got args %#v", ex.args)
	}
	if ex.path != "" {
		t.Fatalf("containerized command should not keep a host working directory: %q", ex.path)
	}
	foundChdir := false
	for idx, arg := range ex.args {
		if arg == "--chdir" && idx+1 < len(ex.args) && ex.args[idx+1] == "/build-root" {
			foundChdir = true
			break
		}
	}
	if !foundChdir {
		t.Fatalf("expected runtime chdir in container args: %#v", ex.args)
	}
}

func TestContainerRuntimePathMapsHostRoot(t *testing.T) {
	container := Container{hostRoot: "/tmp/ignite-root"}
	got := container.RuntimePath("/tmp/ignite-root/install-root/pkg/bin/tool")
	if got != "/install-root/pkg/bin/tool" {
		t.Fatalf("unexpected runtime path: %q", got)
	}
}
