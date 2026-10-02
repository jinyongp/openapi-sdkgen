package main

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

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
