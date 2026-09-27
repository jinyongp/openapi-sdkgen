package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestHarvestInventoryIsDeterministicAcrossManifestAndCheckoutRoots(t *testing.T) {
	makeCheckout := func(root string) string {
		if err := os.MkdirAll(filepath.Join(root, "specs"), 0o755); err != nil {
			t.Fatal(err)
		}
		spec := []byte(`{
  "openapi":"3.1.1",
  "info":{"title":"Harvest","version":"1"},
  "paths":{
    "/a":{"get":{"operationId":"same","responses":{"204":{"description":"OK"}}}},
    "/b":{"get":{"operationId":"same","responses":{"204":{"description":"OK"}}}}
  }
}`)
		path := filepath.Join(root, "specs", "openapi.json")
		if err := os.WriteFile(path, spec, 0o644); err != nil {
			t.Fatal(err)
		}
		return path
	}

	firstRoot := filepath.Join(t.TempDir(), "checkout-a")
	secondRoot := filepath.Join(t.TempDir(), "checkout-b")
	makeCheckout(firstRoot)
	makeCheckout(secondRoot)

	writeManifest := func(root string, reverse bool) string {
		providers := []provider{{ID: "alpha", Input: "specs/openapi.json"}, {ID: "beta", Input: "specs/openapi.json"}}
		if reverse {
			providers[0], providers[1] = providers[1], providers[0]
		}
		data, err := json.Marshal(manifest{Providers: providers})
		if err != nil {
			t.Fatal(err)
		}
		path := filepath.Join(root, "manifest.json")
		if err := os.WriteFile(path, data, 0o644); err != nil {
			t.Fatal(err)
		}
		return path
	}

	firstOutput := filepath.Join(t.TempDir(), "first.json")
	secondOutput := filepath.Join(t.TempDir(), "second.json")
	if err := run(writeManifest(firstRoot, false), firstOutput); err != nil {
		t.Fatal(err)
	}
	if err := run(writeManifest(secondRoot, true), secondOutput); err != nil {
		t.Fatal(err)
	}
	var first, second inventory
	for path, target := range map[string]*inventory{firstOutput: &first, secondOutput: &second} {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal(data, target); err != nil {
			t.Fatal(err)
		}
	}
	if !reflect.DeepEqual(first, second) {
		t.Fatalf("cross-checkout inventory differs:\nfirst=%#v\nsecond=%#v", first, second)
	}
	if len(first.Providers) != 2 || first.Providers[0].ID != "alpha" || first.Providers[1].ID != "beta" {
		t.Fatalf("provider ordering = %#v", first.Providers)
	}
	if first.Providers[0].Complete {
		t.Fatalf("blocking target inventory unexpectedly complete: %#v", first.Providers[0])
	}
	if len(first.Providers[0].Diagnostics) == 0 || first.Providers[0].Diagnostics[0].ID == "" {
		t.Fatalf("missing stable issue identity: %#v", first.Providers[0].Diagnostics)
	}
}

func TestHarvestTreatsFullyAnalyzedBlockingTargetAsCompleteDiscovery(t *testing.T) {
	root := t.TempDir()
	input := filepath.Join(root, "openapi.json")
	if err := os.WriteFile(input, []byte(`{
  "openapi":"3.1.1",
  "info":{"title":"No entry surface","version":"1"},
  "paths":{}
}`), 0o644); err != nil {
		t.Fatal(err)
	}

	result, err := harvestProvider("empty", input, "")
	if err != nil {
		t.Fatal(err)
	}
	if result.Counts.Errors == 0 {
		t.Fatalf("inventory = %#v, want blocking target diagnostic", result)
	}
	for _, coverage := range result.Coverage {
		if coverage.Status != "complete" {
			t.Fatalf("coverage = %#v, want all discovery analyzers complete", result.Coverage)
		}
	}
	if !result.Complete {
		t.Fatalf("fully analyzed blocking inventory marked partial: %#v", result)
	}
}

func TestHarvestRejectsPinnedHashMismatch(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "openapi.json"), []byte(`{"openapi":"3.1.1","info":{"title":"x","version":"1"},"paths":{}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	data, _ := json.Marshal(manifest{Providers: []provider{{ID: "pinned", Input: "openapi.json", SHA256: "deadbeef"}}})
	manifestPath := filepath.Join(root, "manifest.json")
	if err := os.WriteFile(manifestPath, data, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := run(manifestPath, filepath.Join(root, "out.json")); err == nil {
		t.Fatal("expected pinned hash mismatch")
	}
}
