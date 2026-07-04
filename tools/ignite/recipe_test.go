package main

import (
	"os"
	"path/filepath"
	"testing"
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
	got := appendSourcesToRecipeText(input, []string{"patches/pkg/0001-fix.patch", "patches/pkg/0002-next.patch"})
	want := "id: pkg\nversion: 1\nsources:\n  - https://example.invalid/pkg.tar.xz\n  - patches/pkg/0001-fix.patch\n  - patches/pkg/0002-next.patch\nscript: |\n  true\n"
	if got != want {
		t.Fatalf("unexpected recipe text:\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
}

func TestAppendSourcesToRecipeTextNewBlock(t *testing.T) {
	input := "id: pkg\nversion: 1\n"
	got := appendSourcesToRecipeText(input, []string{"patches/pkg/0001-fix.patch"})
	want := "id: pkg\nversion: 1\n\nsources:\n  - patches/pkg/0001-fix.patch\n"
	if got != want {
		t.Fatalf("unexpected recipe text:\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
}

func TestAppendSourcesToRecipeFileSkipsDuplicate(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "pkg.yml")
	input := "id: pkg\nversion: 1\nsources:\n  - patches/pkg/0001-fix.patch\n"
	if err := os.WriteFile(path, []byte(input), 0644); err != nil {
		t.Fatal(err)
	}
	if err := appendSourcesToRecipeFile(path, nil, []string{"patches/pkg/0001-fix.patch"}); err != nil {
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
