package main

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestNoCheckConfigAndCLIOverride(t *testing.T) {
	registries, err := newCLIRegistries()
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		config         string
		flags          []string
		want, explicit bool
	}{
		{"", nil, true, false},
		{"[typescript]\nnocheck = true", nil, true, true},
		{"[typescript]\nnocheck = false", nil, false, true},
		{"[typescript]\nnocheck = false", []string{"--ts-nocheck=true"}, true, true},
		{"[typescript]\nnocheck = true", []string{"--ts-nocheck=false"}, false, true},
	} {
		path := filepath.Join(t.TempDir(), "sdk.toml")
		if err := os.WriteFile(path, []byte(tc.config), 0600); err != nil {
			t.Fatal(err)
		}
		config, base, err := loadGenerateProjectConfig(path)
		if err != nil {
			t.Fatal(err)
		}
		flags, values := newGenerateFlagSet(registries)
		if err := flags.Flags.Parse(tc.flags); err != nil {
			t.Fatal(err)
		}
		visited := visitedGenerateFlags(flags.Flags)
		applyGenerateProjectConfig(config, base, values, visited)
		if *values.tsNoCheck != tc.want || (values.tsNoCheckExplicit || visited["ts-nocheck"]) != tc.explicit {
			t.Fatalf("config %q flags %v: wrong policy", tc.config, tc.flags)
		}
	}
	for _, text := range []string{"[typescript]\nnocheck = 'false'", "[typescript]\nno_check = false", "[clients.a.typescript]\nnocheck = false"} {
		path := filepath.Join(t.TempDir(), "sdk.toml")
		if err := os.WriteFile(path, []byte(text), 0600); err != nil {
			t.Fatal(err)
		}
		if _, _, err := loadGenerateProjectConfig(path); err == nil {
			t.Fatalf("invalid setting accepted: %s", text)
		}
	}
}

func TestNoCheckTransitionsPreserveManagedOutput(t *testing.T) {
	directory := t.TempDir()
	input, output := filepath.Join(directory, "openapi.json"), filepath.Join(directory, "sdk")
	if err := os.WriteFile(input, []byte(metadataAddonInput), 0600); err != nil {
		t.Fatal(err)
	}
	previous := version
	version = "10.0.0-test"
	t.Cleanup(func() { version = previous })
	base := []string{"--input", input, "--output", output, "--target", "typescript"}
	if err := generate(base); err != nil {
		t.Fatal(err)
	}
	user := filepath.Join(output, "user.ts")
	if err := os.WriteFile(user, []byte("export const user = true;\n"), 0600); err != nil {
		t.Fatal(err)
	}
	for _, policy := range []bool{false, true, false} {
		before := snapshotGeneratedDirectory(t, output)
		flag := "--ts-nocheck=false"
		if policy {
			flag = "--ts-nocheck=true"
		}
		args := append(append([]string{}, base...), flag)
		if err := generate(append(args, "--check")); err == nil {
			t.Fatal("check missed header policy drift")
		}
		if !reflect.DeepEqual(before, snapshotGeneratedDirectory(t, output)) {
			t.Fatal("check changed files")
		}
		if err := generate(append(args, "--incremental")); err != nil {
			t.Fatal(err)
		}
		after := snapshotGeneratedDirectory(t, output)
		if before["user.ts"] != after["user.ts"] {
			t.Fatal("user file changed")
		}
		for path, data := range after {
			if path == "user.ts" || !strings.HasSuffix(path, ".ts") {
				continue
			}
			if strings.Contains(data, "// @ts-nocheck\n") != policy {
				t.Fatalf("wrong policy: %s", path)
			}
			if strings.ReplaceAll(before[path], "// @ts-nocheck\n", "") != strings.ReplaceAll(data, "// @ts-nocheck\n", "") {
				t.Fatalf("source changed with header: %s", path)
			}
		}
		manifest, err := readArtifactManifestRecord(output)
		if err != nil {
			t.Fatal(err)
		}
		got := manifest.Generation.TypeScriptNoCheck == nil || *manifest.Generation.TypeScriptNoCheck
		if got != policy {
			t.Fatal("publication identity missed header policy")
		}
		if err := generate(append(args, "--incremental")); err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(after, snapshotGeneratedDirectory(t, output)) {
			t.Fatal("matching generation changed files")
		}
		if err := generate(append(args, "--check")); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(output, "index.ts"), []byte("user edit\n"), 0600); err != nil {
		t.Fatal(err)
	}
	edited := snapshotGeneratedDirectory(t, output)
	if err := generate(append(append([]string{}, base...), "--ts-nocheck=true", "--incremental")); err == nil {
		t.Fatal("edited managed file was replaced")
	}
	if !reflect.DeepEqual(edited, snapshotGeneratedDirectory(t, output)) {
		t.Fatal("failed transition changed files")
	}
}
