package main

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestGoSelectionClientsMetadataPublication(t *testing.T) {
	root := t.TempDir()
	input := filepath.Join(root, "input.json")
	config := filepath.Join(root, "sdk.toml")
	output := filepath.Join(root, "api")
	if err := os.WriteFile(input, []byte(selectionPublicationInput), 0600); err != nil {
		t.Fatal(err)
	}
	base := "source='input.json'\ntarget='golang'\noutput='api'\n[golang]\nimport_path='example.test/consumer/api'\n"
	write := func(extra string) {
		t.Helper()
		if err := os.WriteFile(config, []byte(extra+base), 0600); err != nil {
			t.Fatal(err)
		}
	}
	write("")
	args := []string{"--config", config}
	if err := generate(args); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"go.mod", "go.sum", "user.go"} {
		if err := os.WriteFile(filepath.Join(output, name), []byte("consumer-owned\n"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	before := snapshotGeneratedDirectory(t, output)
	stages := []struct {
		extra                 string
		metadata, left, right bool
	}{
		{"addons=['metadata']\n[selection]\noperations=['a']\n", true, false, false},
		{"addons=['metadata']\n[selection]\noperations=['a']\n[clients.left.selection]\noperations=['a']\n[clients.right.selection]\noperations=['b']\n", true, true, true},
		{"[selection]\noperations=['b']\n[clients.right.selection]\noperations=['b']\n", false, false, true},
	}
	for _, stage := range stages {
		// Keep root settings before TOML tables; table order is intentional.
		body := "source='input.json'\ntarget='golang'\noutput='api'\n" + stage.extra + "\n[golang]\nimport_path='example.test/consumer/api'\n"
		if err := os.WriteFile(config, []byte(body), 0600); err != nil {
			t.Fatal(err)
		}
		if err := generate(append(append([]string{}, args...), "--check")); err == nil {
			t.Fatal("check missed selection/client/addon drift")
		}
		if err := generate(append(append([]string{}, args...), "--incremental")); err != nil {
			t.Fatal(err)
		}
		manifest, err := readArtifactManifestRecord(output)
		if err != nil {
			t.Fatal(err)
		}
		for name, present := range map[string]bool{"metadata/metadata.go": stage.metadata, "clients/left/client.go": stage.left, "clients/right/client.go": stage.right} {
			_, exists := manifest.Files[name]
			if exists != present {
				t.Fatalf("managed file %s=%t want %t", name, exists, present)
			}
		}
		for _, name := range []string{"go.mod", "go.sum", "user.go"} {
			data, err := os.ReadFile(filepath.Join(output, name))
			if err != nil || string(data) != "consumer-owned\n" {
				t.Fatal("consumer file changed", name, err)
			}
		}
		if err := generate(append(append([]string{}, args...), "--check")); err != nil {
			t.Fatal(err)
		}
		if stage.left {
			if err := os.WriteFile(filepath.Join(output, "clients", "left", "custom.txt"), []byte("keep"), 0600); err != nil {
				t.Fatal(err)
			}
		}
	}
	if data, err := os.ReadFile(filepath.Join(output, "clients", "left", "custom.txt")); err != nil || string(data) != "keep" {
		t.Fatal("client removal deleted consumer file", err)
	}
	current := snapshotGeneratedDirectory(t, output)
	if reflect.DeepEqual(before, current) {
		t.Fatal("transitions did not change generation")
	}
	data, err := os.ReadFile(config)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(config, []byte(strings.Replace(string(data), "operations=['b']", "operations=['missing']", 1)), 0600); err != nil {
		t.Fatal(err)
	}
	if err := generate(append(append([]string{}, args...), "--incremental")); err == nil {
		t.Fatal("invalid selection accepted")
	}
	if !reflect.DeepEqual(current, snapshotGeneratedDirectory(t, output)) {
		t.Fatal("failed selection changed publication")
	}
}
