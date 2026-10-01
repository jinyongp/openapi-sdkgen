package main

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"openapi-sdkgen/internal/generator"
)

func TestSelectionConfigPresenceAndNamespaceOverrides(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "sdk.toml")
	registries, err := newCLIRegistries()
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		body    string
		flags   []string
		want    *generator.Selection
		invalid bool
	}{
		{body: "", want: nil},
		{body: "[selection]\n", invalid: true},
		{body: "[selection]\noperations=[]\nroutes=[]", invalid: true},
		{body: "[selection]\noperations=[\"a\"]\nroutes=[\"GET /b\"]", want: &generator.Selection{Operations: []string{"a"}, Routes: []string{"GET /b"}}},
		{body: "[selection]\noperations=[\"a\"]\nroutes=[\"GET /b\"]", flags: []string{"--operation", "c", "--operation", "c"}, want: &generator.Selection{Operations: []string{"c"}, Routes: []string{"GET /b"}}},
		{body: "[selection]\noperations=[\"a\"]\nroutes=[\"GET /b\"]", flags: []string{"--route", "GET /c"}, want: &generator.Selection{Operations: []string{"a"}, Routes: []string{"GET /c"}}},
	} {
		if err := os.WriteFile(configPath, []byte(tc.body), 0600); err != nil {
			t.Fatal(err)
		}
		config, base, err := loadGenerateProjectConfig(configPath)
		if err != nil {
			t.Fatal(err)
		}
		flags, values := newGenerateFlagSet(registries)
		if err := flags.Flags.Parse(tc.flags); err != nil {
			t.Fatal(err)
		}
		visited := visitedGenerateFlags(flags.Flags)
		applyGenerateProjectConfig(config, base, values, visited)
		var selection *generator.Selection
		if values.selectionExplicit || visited["operation"] || visited["route"] {
			selection = &generator.Selection{Operations: values.operations, Routes: values.routes}
		}
		actual, err := selection.Canonical()
		if (err != nil) != tc.invalid || (!tc.invalid && !reflect.DeepEqual(actual, tc.want)) {
			t.Fatalf("config %q flags %v: %#v, %v", tc.body, tc.flags, actual, err)
		}
	}
}

const selectionPublicationInput = `{"openapi":"3.2.1","info":{"title":"Selection publication","version":"1"},"paths":{
"/a":{"get":{"operationId":"a","responses":{"204":{"description":"OK"}}}},
"/b":{"get":{"operationId":"b","responses":{"204":{"description":"OK"}}}}
}}`

func TestSelectionPublicationTransitionsAndErrorsPreserveOutput(t *testing.T) {
	dir := t.TempDir()
	input := filepath.Join(dir, "input.json")
	output := filepath.Join(dir, "sdk")
	if err := os.WriteFile(input, []byte(selectionPublicationInput), 0600); err != nil {
		t.Fatal(err)
	}
	base := []string{"--input", input, "--target", "typescript", "--output", output}
	if err := generate(base); err != nil {
		t.Fatal(err)
	}
	custom := filepath.Join(output, "custom.txt")
	if err := os.WriteFile(custom, []byte("owned by user"), 0600); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		selectors []string
		routes    []string
	}{
		{[]string{"--operation", "a"}, []string{"GET /a"}},
		{[]string{"--route", "GET /b"}, []string{"GET /b"}},
		{nil, []string{"GET /a", "GET /b"}},
	} {
		args := append(append([]string(nil), base...), "--incremental")
		args = append(args, tc.selectors...)
		if err := generate(args); err != nil {
			t.Fatal(err)
		}
		manifest, err := readArtifactManifestRecord(output)
		if err != nil {
			t.Fatal(err)
		}
		for _, name := range []string{"a", "b"} {
			path := "internal/operations/" + name + "/get.ts"
			selected := len(tc.routes) == 2 || tc.routes[0] == "GET /"+name
			if _, exists := manifest.Files[path]; exists != selected {
				t.Fatalf("manifest %s selected=%t exists=%t", path, selected, exists)
			}
		}
		checkArgs := append(append([]string(nil), base...), "--check")
		checkArgs = append(checkArgs, tc.selectors...)
		if err := generate(checkArgs); err != nil {
			t.Fatalf("check: %v", err)
		}
		before, err := os.ReadFile(filepath.Join(output, artifactManifestName))
		if err != nil {
			t.Fatal(err)
		}
		if err := generate(args); err != nil {
			t.Fatal(err)
		}
		after, _ := os.ReadFile(filepath.Join(output, artifactManifestName))
		if !bytes.Equal(before, after) {
			t.Fatal("same selection changed manifest")
		}
		if data, err := os.ReadFile(custom); err != nil || string(data) != "owned by user" {
			t.Fatalf("custom file: %s, %v", data, err)
		}
	}
	before, _ := os.ReadFile(filepath.Join(output, artifactManifestName))
	for _, selector := range [][]string{{"--operation", "a", "--route", "GET /typo"}, {"--operation", ""}} {
		args := append(append([]string(nil), base...), "--incremental")
		args = append(args, selector...)
		if err := generate(args); err == nil {
			t.Fatalf("invalid selector %v accepted", selector)
		}
		after, _ := os.ReadFile(filepath.Join(output, artifactManifestName))
		if !bytes.Equal(before, after) {
			t.Fatal("invalid selection changed previous output")
		}
	}
	duplicate := bytes.Replace([]byte(selectionPublicationInput), []byte(`"operationId":"b"`), []byte(`"operationId":"a"`), 1)
	if err := os.WriteFile(input, duplicate, 0600); err != nil {
		t.Fatal(err)
	}
	for _, selector := range [][]string{nil, {"--operation", "a"}, {"--route", "GET /a"}} {
		args := append(append([]string(nil), base...), "--incremental")
		args = append(args, selector...)
		if err := generate(args); !errors.Is(err, errReportedDiagnostics) {
			t.Fatalf("duplicate ID error = %v", err)
		}
		after, _ := os.ReadFile(filepath.Join(output, artifactManifestName))
		if !bytes.Equal(before, after) {
			t.Fatal("compiler error changed output")
		}
	}
}
