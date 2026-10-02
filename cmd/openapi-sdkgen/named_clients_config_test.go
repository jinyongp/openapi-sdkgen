package main

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	compiler "openapi-sdkgen/internal/compiler"
	"openapi-sdkgen/internal/generator"
)

func TestNamedClientsConfigKeepsRootSelectionIndependent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sdk.toml")
	text := `[selection]
operations = ["root"]
[clients.orders.selection]
operations = ["listOrders", "listOrders"]
[clients.catalog.selection]
routes = ["GET /idless"]
`
	if err := os.WriteFile(path, []byte(text), 0600); err != nil {
		t.Fatal(err)
	}
	config, base, err := loadGenerateProjectConfig(path)
	if err != nil {
		t.Fatal(err)
	}
	registries, err := newCLIRegistries()
	if err != nil {
		t.Fatal(err)
	}
	flags, values := newGenerateFlagSet(registries)
	if err := flags.Flags.Parse([]string{"--operation", "override"}); err != nil {
		t.Fatal(err)
	}
	applyGenerateProjectConfig(config, base, values, visitedGenerateFlags(flags.Flags))
	clients, err := generator.CanonicalClients(values.clients)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(values.operations, rawStrings{"override"}) ||
		!reflect.DeepEqual(clients["orders"].Selection.Operations, []string{"listOrders"}) ||
		!reflect.DeepEqual(clients["catalog"].Selection.Routes, []string{"GET /idless"}) {
		t.Fatalf("root %v clients %#v", values.operations, clients)
	}
}

func TestNamedClientsCLICompilesAndPreparesOneSharedDocument(t *testing.T) {
	directory := t.TempDir()
	config := filepath.Join(directory, "sdk.toml")
	if err := os.WriteFile(filepath.Join(directory, "input.json"), []byte(selectionPublicationInput), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(config, []byte("source='./input.json'\ntarget='typescript'\noutput='./sdk'\n[selection]\noperations=['a']\n[clients.first.selection]\noperations=['b']\n[clients.second.selection]\nroutes=['GET /a','GET /b']\n"), 0600); err != nil {
		t.Fatal(err)
	}
	runtime := defaultGenerationRuntime
	compile, prepare := runtime.compile, runtime.prepare
	compiledCalls, preparedCalls := 0, 0
	runtime.compile = func(input string, options compiler.CompileOptions) (compiler.Result, error) {
		compiledCalls++
		return compile(input, options)
	}
	runtime.prepare = func(target generator.Target, result compiler.Result, options generator.Options) (generator.Preparation, error) {
		preparedCalls++
		if len(options.Clients) != 2 || options.Selection.Operations[0] != "a" {
			t.Fatalf("independent options not forwarded: %#v", options)
		}
		return prepare(target, result, options)
	}
	if err := generateWithRuntime([]string{"--config", config}, runtime); err != nil {
		t.Fatal(err)
	}
	if compiledCalls != 1 || preparedCalls != 1 {
		t.Fatalf("compile=%d prepare=%d", compiledCalls, preparedCalls)
	}
}

func TestNamedClientConfigRejectsNonSelectionProperties(t *testing.T) {
	for _, text := range []string{
		"[clients.orders]\noperations=[\"listOrders\"]",
		"[clients.orders]\nsource=\"other.json\"",
		"[clients.orders.defaults]\nbase_url=\"https://example.test\"",
		"[clients.orders.selection]\nunknown=true",
	} {
		path := filepath.Join(t.TempDir(), "sdk.toml")
		if err := os.WriteFile(path, []byte(text), 0600); err != nil {
			t.Fatal(err)
		}
		if _, _, err := loadGenerateProjectConfig(path); err == nil {
			t.Fatalf("invalid config accepted: %s", text)
		}
	}
}
