package main

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
)

func TestIgnoredAndInitializationFilesAreExcluded(t *testing.T) {
	root := t.TempDir()
	for _, test := range []struct{ name, source, reason string }{
		{"_ignored.go", "package api; func A() int { return 99 }", "go_ignored_filename"},
		{".ignored.go", "invalid source must never become active", "go_ignored_filename"},
		{"state.go", "package api; var A = 1", "package_variable"},
		{"init.go", "package api; func init() {}", "init_function"},
		{"api_linux.go", "package api; type A int", "test_or_platform_file"},
		{"line.go", "package api\n/*line fake.go:1*/\nfunc A() {}", "directive"},
		{"safe.go", "package api; type A struct { Value string `json:\"value\"` }; func (a A) Read() string { return a.Value }", ""},
	} {
		t.Run(test.name, func(t *testing.T) {
			path := filepath.Join(root, test.name)
			if err := os.WriteFile(path, []byte(test.source), 0600); err != nil {
				t.Fatal(err)
			}
			r, err := inspect(root, path, false)
			if err != nil {
				t.Fatal(err)
			}
			if test.reason != "" && !slices.Contains(r.Reasons, test.reason) {
				t.Fatalf("missing %s: %v", test.reason, r.Reasons)
			}
			if test.reason == "" && (len(r.Reasons) != 0 || len(r.Types) != 1 || len(r.Public) != 2) {
				t.Fatalf("safe declarations: %+v", r)
			}
		})
	}
}

func TestAuditCanCompleteAnExplicitlyRaisedParseBudget(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "api.go")
	if err := os.WriteFile(path, []byte("package api; type Public struct { Value string `json:\"value\"` }"), 0600); err != nil {
		t.Fatal(err)
	}
	limited, err := inspectWithLimit(root, path, false, 16)
	if err != nil || len(limited.Types) != 0 || len(limited.Reasons) != 1 {
		t.Fatalf("bounded audit must expose missing inventory: %+v %v", limited, err)
	}
	complete, err := inspectWithLimit(root, path, false, 0)
	if err != nil || len(complete.Reasons) != 0 || len(complete.Types) != 1 || complete.Public["Public"] == "" {
		t.Fatalf("explicit unlimited audit must recover the public contract: %+v %v", complete, err)
	}
}
