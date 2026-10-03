package main

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	compiler "openapi-sdkgen/internal/compiler"
)

func TestMetadataAddonTransitionsPreserveManagedOutput(t *testing.T) {
	directory := t.TempDir()
	input, output := filepath.Join(directory, "openapi.json"), filepath.Join(directory, "sdk")
	if err := os.WriteFile(input, []byte(metadataAddonInput), 0600); err != nil {
		t.Fatal(err)
	}
	previousVersion := version
	version = "10.0.0-test"
	t.Cleanup(func() { version = previousVersion })
	base := []string{"--input", input, "--output", output, "--target", "typescript", "--operation", "health"}
	args := func(metadata bool, mode string) []string {
		values := append([]string(nil), base...)
		if metadata {
			values = append(values, "--with", "metadata")
		}
		if mode != "" {
			values = append(values, mode)
		}
		return values
	}
	if err := generate(args(false, "")); err != nil {
		t.Fatal(err)
	}
	userFile := filepath.Join(output, "user.ts")
	if err := os.WriteFile(userFile, []byte("export const ownedByUser = true;\n"), 0600); err != nil {
		t.Fatal(err)
	}
	var defaultIdentity string
	for _, metadata := range []bool{false, true, false} {
		before := snapshotGeneratedDirectory(t, output)
		manifestBefore, err := readArtifactManifestRecord(output)
		if err != nil {
			t.Fatal(err)
		}
		changing := (len(manifestBefore.Generation.Addons) > 0) != metadata
		err = generate(args(metadata, "--check"))
		if (err != nil) != changing {
			t.Fatalf("check setting drift = %v, changing=%v", err, changing)
		}
		if !reflect.DeepEqual(before, snapshotGeneratedDirectory(t, output)) {
			t.Fatal("check wrote files")
		}
		if err := generate(args(metadata, "--incremental")); err != nil {
			t.Fatal(err)
		}
		after := snapshotGeneratedDirectory(t, output)
		if after["user.ts"] != before["user.ts"] {
			t.Fatal("user file changed")
		}
		if strings.Contains(after["metadata.ts"], "document:") != metadata || strings.Contains(after["metadata.ts"], "generationSelection") != metadata {
			t.Fatal("metadata content or selection lost")
		}
		identity := after["selective/index.ts"]
		if metadata && identity == defaultIdentity {
			t.Fatal("metadata change retained selective fingerprint")
		}
		if !metadata {
			if defaultIdentity != "" && identity != defaultIdentity {
				t.Fatal("return to default changed fingerprint")
			}
			defaultIdentity = identity
		}
		manifest, err := readArtifactManifestRecord(output)
		if err != nil {
			t.Fatal(err)
		}
		if (len(manifest.Generation.Addons) > 0) != metadata {
			t.Fatal("addon identity not recorded")
		}
		if err := generate(args(metadata, "--incremental")); err != nil {
			t.Fatalf("matching settings failed: %v", err)
		}
		if !reflect.DeepEqual(after, snapshotGeneratedDirectory(t, output)) {
			t.Fatal("no-op changed output")
		}
		if err := generate(args(metadata, "--check")); err != nil {
			t.Fatal(err)
		}
	}
	metadataPath := filepath.Join(output, "metadata.ts")
	generatedMetadata, err := os.ReadFile(metadataPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(metadataPath, []byte("user edit\n"), 0600); err != nil {
		t.Fatal(err)
	}
	edited := snapshotGeneratedDirectory(t, output)
	err = generate(args(true, "--incremental"))
	if err == nil || !strings.Contains(err.Error(), "was edited") {
		t.Fatalf("edited metadata replaced: %v", err)
	}
	if !reflect.DeepEqual(edited, snapshotGeneratedDirectory(t, output)) {
		t.Fatal("failed transition changed output")
	}
	// Restore the test's own edit before testing lock acquisition, which also
	// checks ownership and correctly refuses edited managed files.
	if err := os.WriteFile(metadataPath, generatedMetadata, 0600); err != nil {
		t.Fatal(err)
	}
	assertIncrementalLockReusable(t, output)
}

func TestMetadataAddonFullNoOpSkipsCompiler(t *testing.T) {
	previousVersion := version
	version = "10.0.0-test"
	t.Cleanup(func() { version = previousVersion })
	for _, metadata := range []bool{false, true} {
		directory := t.TempDir()
		input, output := filepath.Join(directory, "openapi.json"), filepath.Join(directory, "sdk")
		if err := os.WriteFile(input, []byte(metadataAddonInput), 0600); err != nil {
			t.Fatal(err)
		}
		args := []string{"--input", input, "--target", "typescript", "--output", output}
		if metadata {
			args = append(args, "--with", "metadata")
		}
		if err := generate(args); err != nil {
			t.Fatal(err)
		}
		compiled := false
		runtime := generationRuntime{compile: func(string, compiler.CompileOptions) (compiler.Result, error) {
			compiled = true
			return compiler.Result{}, errors.New("unexpected compile")
		}}
		if err := generateWithRuntime(append(args, "--incremental"), runtime); err != nil || compiled {
			t.Fatalf("full no-op recompiled: %v", err)
		}
	}
}

func TestMetadataAddonBlockedInputPreservesOutput(t *testing.T) {
	directory := t.TempDir()
	input, output := filepath.Join(directory, "openapi.json"), filepath.Join(directory, "sdk")
	if err := os.WriteFile(input, []byte(metadataAddonInput), 0600); err != nil {
		t.Fatal(err)
	}
	base := []string{"--input", input, "--target", "typescript", "--output", output}
	if err := generate(base); err != nil {
		t.Fatal(err)
	}
	before := snapshotGeneratedDirectory(t, output)
	invalid := bytes.ReplaceAll([]byte(metadataAddonInput), []byte(`"openapi":"3.2.1"`), []byte(`"openapi":"4.0.0"`))
	if err := os.WriteFile(input, invalid, 0600); err != nil {
		t.Fatal(err)
	}
	if err := generate(append(base, "--incremental", "--with", "metadata")); err == nil {
		t.Fatal("invalid input accepted")
	}
	if !reflect.DeepEqual(before, snapshotGeneratedDirectory(t, output)) {
		t.Fatal("blocked metadata transition changed output")
	}
	assertIncrementalLockReusable(t, output)
}
