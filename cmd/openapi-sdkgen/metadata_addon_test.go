package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

const metadataAddonInput = `{"openapi":"3.2.1","info":{"title":"Original metadata sentinel","version":"1"},"paths":{"/health":{"get":{"operationId":"health","responses":{"204":{"description":"OK"}}}}}}`

func TestGenerateMetadataAddonFromCLIAndConfig(t *testing.T) {
	for _, test := range []struct {
		name     string
		config   string
		args     []string
		document bool
		server   bool
	}{
		{name: "default"},
		{name: "cli", args: []string{"--with", "metadata"}, document: true},
		{name: "config", config: `addons = ["metadata"]`, document: true},
		{name: "both", args: []string{"--with", "server", "--with", "metadata"}, document: true, server: true},
		{name: "config-both", config: `addons = ["server", "metadata"]`, document: true, server: true},
		{name: "cli-replaces-config", config: `addons = ["server", "metadata"]`, args: []string{"--with", "server"}, server: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			directory := t.TempDir()
			input := filepath.Join(directory, "openapi.json")
			if err := os.WriteFile(input, []byte(metadataAddonInput), 0600); err != nil {
				t.Fatal(err)
			}
			config := filepath.Join(directory, "openapi-sdkgen.toml")
			if err := os.WriteFile(config, []byte(test.config), 0600); err != nil {
				t.Fatal(err)
			}
			output := filepath.Join(directory, "sdk")
			args := append([]string{"generate", "--input", input, "--output", output, "--target", "typescript", "--config", config}, test.args...)
			if err := run(args); err != nil {
				t.Fatal(err)
			}
			metadata, err := os.ReadFile(filepath.Join(output, "metadata.ts"))
			if err != nil {
				t.Fatal(err)
			}
			if bytes.Contains(metadata, []byte("Original metadata sentinel")) != test.document || bytes.Contains(metadata, []byte("document:")) != test.document {
				t.Fatalf("document option ignored: %s", metadata)
			}
			_, err = os.Stat(filepath.Join(output, "server", "runtime.ts"))
			if (err == nil) != test.server {
				t.Fatalf("server presence = %v, want %v", err == nil, test.server)
			}
		})
	}
}
