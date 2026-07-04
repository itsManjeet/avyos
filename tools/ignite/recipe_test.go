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
