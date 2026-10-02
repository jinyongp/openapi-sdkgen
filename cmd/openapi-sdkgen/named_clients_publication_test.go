package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNamedClientsPublicationReassignmentCleanupAndReadOnlyCheck(t *testing.T) {
	dir := t.TempDir()
	input, config, output := filepath.Join(dir, "input.json"), filepath.Join(dir, "sdk.toml"), filepath.Join(dir, "sdk")
	if err := os.WriteFile(input, []byte(selectionPublicationInput), 0600); err != nil {
		t.Fatal(err)
	}
	previous := version
	version = "named-clients-test"
	t.Cleanup(func() { version = previous })
	writeConfig := func(blocks string) {
		t.Helper()
		text := "source='./input.json'\ntarget='typescript'\noutput='./sdk'\n" + blocks
		if err := os.WriteFile(config, []byte(text), 0600); err != nil {
			t.Fatal(err)
		}
	}
	writeConfig("[clients.first.selection]\noperations=['a']\n[clients.second.selection]\noperations=['b']\n")
	if err := generate([]string{"--config", config}); err != nil {
		t.Fatal(err)
	}
	readManifest := func() []byte {
		t.Helper()
		data, err := os.ReadFile(filepath.Join(output, artifactManifestName))
		if err != nil {
			t.Fatal(err)
		}
		return data
	}
	before := readManifest()
	custom := filepath.Join(output, "clients", "first", "custom.txt")
	if err := os.WriteFile(custom, []byte("user"), 0600); err != nil {
		t.Fatal(err)
	}
	writeConfig("[clients.first.selection]\noperations=['b']\n[clients.second.selection]\noperations=['a']\n")
	if err := generate([]string{"--config", config, "--check"}); err == nil {
		t.Fatal("check accepted stale client assignments")
	}
	if !bytes.Equal(before, readManifest()) {
		t.Fatal("check modified existing output")
	}
	if err := generate([]string{"--config", config, "--incremental"}); err != nil {
		t.Fatal(err)
	}
	manifest, err := readArtifactManifestRecord(output)
	if err != nil || len(manifest.Generation.Clients) != 2 || manifest.Generation.Clients[0].Name != "first" || manifest.Generation.Clients[0].Routes[0] != "GET /b" {
		t.Fatalf("assignment fingerprint = %#v, %v", manifest, err)
	}
	if bytes.Equal(before, readManifest()) {
		t.Fatal("same union reassignment did not update output")
	}
	before = readManifest()
	// Equivalent ID/route selections produce byte-identical output and identity.
	writeConfig("[clients.second.selection]\nroutes=['GET /a']\n[clients.first.selection]\nroutes=['GET /b','GET /b']\n")
	if err := generate([]string{"--config", config, "--incremental"}); err != nil || !bytes.Equal(before, readManifest()) {
		t.Fatalf("equivalent assignment changed output: %v", err)
	}
	writeConfig("[clients.renamed.selection]\noperations=['a']\n")
	if err := generate([]string{"--config", config, "--incremental"}); err != nil {
		t.Fatal(err)
	}
	for _, old := range []string{"first", "second"} {
		if _, err := os.Stat(filepath.Join(output, "clients", old, "index.ts")); !os.IsNotExist(err) {
			t.Fatalf("old client %s remains: %v", old, err)
		}
	}
	if data, err := os.ReadFile(custom); err != nil || string(data) != "user" {
		t.Fatalf("user file lost: %q, %v", data, err)
	}
	before = readManifest()
	for _, invalid := range []string{"[clients.invalid.selection]\noperations=['missing']\n", "[clients.invalid]\nsource='other.json'\n"} {
		writeConfig(invalid)
		if err := generate([]string{"--config", config, "--incremental"}); err == nil || !bytes.Equal(before, readManifest()) {
			t.Fatalf("invalid config changed output: %v", err)
		}
	}
	writeConfig("[clients.renamed.selection]\noperations=['a']\n")
	if err := generate([]string{"--config", config, "--check"}); err != nil {
		t.Fatal(err)
	}
	owned := filepath.Join(output, "clients", "renamed", "index.ts")
	ownedData, err := os.ReadFile(owned)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(owned, []byte("user edit"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := generate([]string{"--config", config, "--incremental"}); err == nil || !strings.Contains(err.Error(), "was edited") || !bytes.Equal(before, readManifest()) {
		t.Fatalf("edited named entry protection: %v", err)
	}
	if data, err := os.ReadFile(owned); err != nil || string(data) != "user edit" {
		t.Fatalf("edited entry changed: %q, %v", data, err)
	}
	if err := os.WriteFile(owned, ownedData, 0600); err != nil {
		t.Fatal(err)
	}
	assertIncrementalLockReusable(t, output)
}
