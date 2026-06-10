package main

import "testing"

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
