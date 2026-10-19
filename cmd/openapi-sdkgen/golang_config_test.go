package main

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestGoConfigAndCLIOverride(t *testing.T) {
	registries, err := newCLIRegistries()
	if err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(t.TempDir(), "sdk.toml")
	if err := os.WriteFile(file, []byte("[golang]\npackage='custom'\nimport_path='example.test/custom'\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	config, base, err := loadGenerateProjectConfig(file)
	if err != nil {
		t.Fatal(err)
	}
	for _, override := range []bool{false, true} {
		flags, values := newGenerateFlagSet(registries)
		args := []string{}
		if override {
			args = []string{"--go-package", "override", "--go-import-path", "example.test/override"}
		}
		if err := flags.Flags.Parse(args); err != nil {
			t.Fatal(err)
		}
		applyGenerateProjectConfig(config, base, values, visitedGenerateFlags(flags.Flags))
		wantPackage, wantPath := "custom", "example.test/custom"
		if override {
			wantPackage, wantPath = "override", "example.test/override"
		}
		if !values.goExplicit || *values.goPackage != wantPackage || *values.goImportPath != wantPath {
			t.Fatalf("wrong Go configuration: %s %s", *values.goPackage, *values.goImportPath)
		}
	}
}

func TestEmptyGoSectionRemainsExplicit(t *testing.T) {
	registries, err := newCLIRegistries()
	if err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(t.TempDir(), "sdk.toml")
	if err := os.WriteFile(file, []byte("[golang]\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	config, base, err := loadGenerateProjectConfig(file)
	if err != nil {
		t.Fatal(err)
	}
	flags, values := newGenerateFlagSet(registries)
	applyGenerateProjectConfig(config, base, values, visitedGenerateFlags(flags.Flags))
	if !values.goExplicit {
		t.Fatal("empty Go configuration section lost its target ownership")
	}
}

const goHealthInput = `{"openapi":"3.1.0","info":{"title":"Health","version":"1"},"paths":{"/health":{"get":{"operationId":"getHealth","responses":{"200":{"description":"Healthy","content":{"application/json":{"schema":{"type":"boolean"}}}}}}}}}`

func TestGoGenerationFingerprintAndOwnership(t *testing.T) {
	root := t.TempDir()
	input, output := filepath.Join(root, "input.json"), filepath.Join(root, "api")
	if err := os.WriteFile(input, []byte(goHealthInput), 0o600); err != nil {
		t.Fatal(err)
	}
	previous := version
	version = "10.0.0-go-test"
	t.Cleanup(func() { version = previous })
	args := []string{"--input", input, "--output", output, "--target", "golang", "--go-import-path", "example.test/consumer/api"}
	if err := generate(args); err != nil {
		t.Fatal(err)
	}
	for _, file := range []string{"user.go", "go.mod", "go.sum"} {
		if err := os.WriteFile(filepath.Join(output, file), []byte("user-owned\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	before := snapshotGeneratedDirectory(t, output)
	changed := append(append([]string{}, args...), "--go-package", "custom")
	if err := generate(append(changed, "--check")); err == nil {
		t.Fatal("check missed package drift")
	}
	if !reflect.DeepEqual(before, snapshotGeneratedDirectory(t, output)) {
		t.Fatal("check mutated output")
	}
	if err := generate(append(changed, "--incremental")); err != nil {
		t.Fatal(err)
	}
	after := snapshotGeneratedDirectory(t, output)
	for _, file := range []string{"user.go", "go.mod", "go.sum"} {
		if before[file] != after[file] {
			t.Fatalf("changed consumer-owned %s", file)
		}
	}
	manifest, err := readArtifactManifestRecord(output)
	if err != nil {
		t.Fatal(err)
	}
	if manifest.Generation.Golang == nil || manifest.Generation.Golang.Package != "custom" {
		t.Fatal("Go options missing from fingerprint")
	}
	if err := generate(append(changed, "--check")); err != nil {
		t.Fatal(err)
	}
	if err := generate(append(changed, "--incremental")); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(after, snapshotGeneratedDirectory(t, output)) {
		t.Fatal("identical incremental generation changed output")
	}
	for _, extra := range [][]string{{"--typecheck=false"}, {"--go-import-path", ""}, {"--go-package", "main"}, {"--target", "typescript"}} {
		if err := generate(append(append([]string{}, changed...), extra...)); err == nil {
			t.Fatalf("accepted invalid options: %v", extra)
		}
	}
}
