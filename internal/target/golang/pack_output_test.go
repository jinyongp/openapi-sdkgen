package golang

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	compiler "openapi-sdkgen/internal/compiler"
	"openapi-sdkgen/internal/generator"
	"openapi-sdkgen/internal/output"
)

func TestPackIncrementalPublicationPreservesOwnership(t *testing.T) {
	plan := func(first, second string) generator.Plan {
		document, err := compiler.Compile([]byte(fmt.Sprintf(`{"openapi":"3.1.0","info":{"title":"Pack output","version":"1"},"paths":{
"/%s":{"get":{"operationId":"%s","responses":{"204":{"description":"Done"}}}},
"/%s":{"get":{"operationId":"%s","responses":{"204":{"description":"Done"}}}}}}`, first, first, second, second)))
		if err != nil {
			t.Fatal(err)
		}
		prepared, diagnostics, err := (Generator{}).Prepare(document, generator.Options{Golang: &generator.GoOptions{ImportPath: "example.test/consumer/api"}})
		if err != nil || len(diagnostics) != 0 {
			t.Fatal(err, diagnostics)
		}
		return prepared
	}
	root := filepath.Join(t.TempDir(), "sdk")
	first, second := plan("a", "b"), plan("b", "c")
	emit := func(prepared generator.Plan, incremental bool) error {
		return output.StreamArtifacts(root, incremental, nil, func(sink generator.ArtifactSink) error { return (Generator{}).EmitTo(prepared, sink) })
	}
	if err := emit(first, false); err != nil {
		t.Fatal(err)
	}
	manifestPath := filepath.Join(root, output.ManifestName)
	before, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatal(err)
	}
	var manifest output.Manifest
	if err := json.Unmarshal(before, &manifest); err != nil {
		t.Fatal(err)
	}
	packed := ""
	for name := range manifest.Files {
		if strings.Contains(name, "/packed_") {
			packed = name
			break
		}
	}
	if packed == "" {
		t.Fatal("fixture did not use native pack output")
	}
	if err := output.CheckManaged(root, nil, func(sink generator.ArtifactSink) error { return (Generator{}).EmitTo(first, sink) }); err != nil {
		t.Fatal(err)
	}
	after, err := os.ReadFile(manifestPath)
	if err != nil || !bytes.Equal(before, after) {
		t.Fatal("no-op check changed manifest", err)
	}
	owned := filepath.Join(root, filepath.FromSlash(packed))
	original, err := os.ReadFile(owned)
	if err != nil {
		t.Fatal(err)
	}
	edited := append(append([]byte(nil), original...), []byte("\n// User change\n")...)
	if err := os.WriteFile(owned, edited, 0644); err != nil {
		t.Fatal(err)
	}
	if err := emit(second, true); err == nil {
		t.Fatal("edited packed file was overwritten")
	}
	after, err = os.ReadFile(manifestPath)
	if err != nil || !bytes.Equal(before, after) {
		t.Fatal("failed publication changed manifest", err)
	}
	actual, err := os.ReadFile(owned)
	if err != nil || !bytes.Equal(edited, actual) {
		t.Fatal("failed publication changed user edit", err)
	}
	if err := os.WriteFile(owned, original, 0644); err != nil {
		t.Fatal(err)
	}
	user := filepath.Join(root, "notes.txt")
	if err := os.WriteFile(user, []byte("user-owned"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := emit(second, true); err != nil {
		t.Fatal(err)
	}
	if err := output.CheckManaged(root, nil, func(sink generator.ArtifactSink) error { return (Generator{}).EmitTo(second, sink) }); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatal(err)
	}
	var current output.Manifest
	if err := json.Unmarshal(data, &current); err != nil {
		t.Fatal(err)
	}
	removed := 0
	for name := range manifest.Files {
		if _, exists := current.Files[name]; !exists {
			removed++
			if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(name))); !os.IsNotExist(err) {
				t.Fatal("stale artifact survived", name, err)
			}
		}
	}
	if removed == 0 {
		t.Fatal("fixture did not replace/remove artifacts")
	}
	if data, err := os.ReadFile(user); err != nil || string(data) != "user-owned" {
		t.Fatal("unmanaged file changed", err)
	}
}
