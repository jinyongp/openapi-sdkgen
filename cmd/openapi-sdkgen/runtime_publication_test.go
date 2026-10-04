package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestRuntimeFeaturePublicationTransitionsPreserveOwnership(t *testing.T) {
	dir := t.TempDir()
	input := filepath.Join(dir, "input.json")
	output := filepath.Join(dir, "sdk")
	fixture := `{"openapi":"3.2.0","info":{"title":"Runtime publication","version":"1"},"paths":{"/json":{"get":{"operationId":"json","responses":{"204":{"description":"ok"}}}},"/xml":{"get":{"operationId":"xml","responses":{"200":{"description":"ok","content":{"application/xml":{"schema":{"type":"string"}}}}}}}}}`
	if err := os.WriteFile(input, []byte(fixture), 0600); err != nil {
		t.Fatal(err)
	}
	base := []string{"--input", input, "--output", output, "--target", "typescript"}
	if err := generate(base); err != nil {
		t.Fatal(err)
	}
	xml := filepath.Join(output, "internal/runtime/media/xml/xml-codec.ts")
	original, err := os.ReadFile(xml)
	if err != nil {
		t.Fatal(err)
	}
	unmanaged := filepath.Join(output, "internal/runtime/local.ts")
	if err := os.WriteFile(unmanaged, []byte("user"), 0600); err != nil {
		t.Fatal(err)
	}
	selection := append(append([]string{}, base...), "--incremental", "--operation", "json")
	manifestPath := filepath.Join(output, artifactManifestName)
	before, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := generate(append(append([]string{}, selection...), "--check")); err == nil {
		t.Fatal("check accepted stale runtime artifacts")
	}
	after, _ := os.ReadFile(manifestPath)
	if !bytes.Equal(before, after) {
		t.Fatal("check modified the artifact manifest")
	}
	if err := os.WriteFile(xml, []byte("user edit"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := generate(selection); err == nil {
		t.Fatal("generation deleted an edited owned runtime")
	}
	after, _ = os.ReadFile(manifestPath)
	if !bytes.Equal(before, after) {
		t.Fatal("failed ownership check changed manifest")
	}
	if err := os.WriteFile(xml, original, 0600); err != nil {
		t.Fatal(err)
	}
	if err := generate(selection); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(xml); !os.IsNotExist(err) {
		t.Fatalf("stale XML runtime remains: %v", err)
	}
	if data, err := os.ReadFile(unmanaged); err != nil || string(data) != "user" {
		t.Fatal("unmanaged runtime file was changed")
	}
	if err := generate(append(append([]string{}, base...), "--incremental")); err != nil {
		t.Fatal(err)
	}
	if data, err := os.ReadFile(xml); err != nil || !bytes.Equal(data, original) {
		t.Fatal("full regeneration did not restore canonical XML runtime")
	}
	if err := generate(append(append([]string{}, base...), "--check")); err != nil {
		t.Fatal(err)
	}
}
