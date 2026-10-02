package sdkgen

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"openapi-sdkgen/internal/compiler/compatibility"
	"openapi-sdkgen/internal/diagnostic"
)

func TestInventoryLockedOfflineRemotePathItem(t *testing.T) {
	dir := t.TempDir()
	input := filepath.Join(dir, "openapi.json")
	remoteURL := "https://schemas.example.test/paths.json"
	remote := []byte(`{"Item":{"get":{"summary":"Remote"}},"ignored":{"$ref":"https://untrusted.example.test/schema"}}`)
	digest := sha256.Sum256(remote)
	encoded := hex.EncodeToString(digest[:])
	lockPath := defaultReferenceLockPath(input)
	if err := writeReferenceLock(lockPath, &referenceLock{Version: 1, References: map[string]string{remoteURL: encoded}, Extensions: map[string]string{}}); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(lockPath)
	if err != nil {
		t.Fatal(err)
	}
	cache := filepath.Join(dir, ".openapi-sdkgen-cache")
	if err := os.Mkdir(cache, 0700); err != nil {
		t.Fatal(err)
	}
	cachePath := filepath.Join(cache, encoded)
	if err := os.WriteFile(cachePath, remote, 0600); err != nil {
		t.Fatal(err)
	}
	body := fmt.Sprintf(`{"openapi":"3.2.1","paths":{"/a":{"$ref":%q},"/b":{"$ref":%q}}}`, remoteURL+"#/Item", remoteURL+"#/Item")
	if err := os.WriteFile(input, []byte(body), 0600); err != nil {
		t.Fatal(err)
	}
	metrics := &compilationMetrics{}
	options := CompileOptions{RemoteRefAllowlist: []string{"https://schemas.example.test"}, Offline: true, metrics: metrics}
	result, err := InspectInputResult(input, options)
	if err != nil || result.Inventory == nil || result.Inventory.DocumentsRead != 2 || metrics.RemoteReferenceSourceDecodes != 1 {
		t.Fatalf("result=%#v err=%v metrics=%#v", result, err, metrics)
	}
	assertFileContents(t, lockPath, before)
	if err := os.WriteFile(cachePath, []byte("changed"), 0600); err != nil {
		t.Fatal(err)
	}
	result, err = InspectInputResult(input, options)
	if err != nil || result.Inventory != nil || !diagnostic.HasErrors(result.Diagnostics) {
		t.Fatalf("tampered cache accepted: %#v %v", result, err)
	}
	assertFileContents(t, lockPath, before)
	options.RemoteRefAllowlist = nil
	result, err = InspectInputResult(input, options)
	if err != nil || result.Inventory != nil {
		t.Fatalf("untrusted remote accepted: %#v %v", result, err)
	}
}

func TestInventoryReadsDeclarationsWithoutSchemaAnalysis(t *testing.T) {
	for _, version := range []string{"3.0.4", "3.1.2", "3.2.1"} {
		t.Run(version, func(t *testing.T) {
			input := fmt.Sprintf(`{"openapi":%q,"info":{"title":"Catalog","version":"1"},"paths":{"/users/{id}":{"get":{"operationId":"getUser","tags":["Users"],"summary":"Find user","deprecated":true,"responses":{"200":{"content":{"application/json":{"schema":{"$ref":"missing-schema.json"}}}}}}},"/health":{"get":{}}},"webhooks":{"event":{"post":{"operationId":"event"}}},"components":{"schemas":{"Broken":{"type":123}}}}`, version)
			metrics := &compilationMetrics{}
			result, err := InspectInputResult("-", CompileOptions{InputReader: strings.NewReader(input), metrics: metrics})
			if err != nil || result.Inventory == nil || diagnostic.HasErrors(result.Diagnostics) {
				t.Fatalf("inventory=%#v err=%v", result, err)
			}
			inventory := result.Inventory
			if inventory.Title != "Catalog" || len(inventory.Operations) != 2 || inventory.DocumentsRead != 1 {
				t.Fatalf("inventory=%#v", inventory)
			}
			if inventory.Operations[0].OperationID != nil || inventory.Operations[1].Route != "GET /users/{id}" || !inventory.Operations[1].Deprecated {
				t.Fatalf("operations=%#v", inventory.Operations)
			}
			if metrics.SourceDecodes != 1 || metrics.ModelBuilds != 0 || metrics.Bundles != 0 || metrics.ReferenceSourceDecodes != 0 {
				t.Fatalf("metrics=%#v", metrics)
			}
		})
	}
}

func TestInventoryMountedPathItemClosureAndCache(t *testing.T) {
	dir := t.TempDir()
	entry := filepath.Join(dir, "openapi.yaml")
	if err := os.WriteFile(entry, []byte("openapi: 3.2.1\npaths:\n  /a:\n    $ref: './items.yaml#/x~1item'\n  /b:\n    $ref: './items.yaml#/x~1item'\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "items.yaml"), []byte("x/item:\n  get:\n    summary: Shared\n  additionalOperations:\n    CuStOm:\n      summary: Custom\nunused:\n  $ref: absent.yaml\n"), 0600); err != nil {
		t.Fatal(err)
	}
	metrics := &compilationMetrics{}
	result, err := InspectInputResult(entry, CompileOptions{metrics: metrics})
	if err != nil || result.Inventory == nil {
		t.Fatalf("inventory=%#v err=%v", result, err)
	}
	if result.Inventory.DocumentsRead != 2 || metrics.ReferenceSourceDecodes != 1 || len(result.Inventory.Operations) != 4 {
		t.Fatalf("inventory=%#v metrics=%#v", result.Inventory, metrics)
	}
	if result.Inventory.Operations[0].Route != "CuStOm /a" || result.Inventory.Operations[3].Route != "GET /b" {
		t.Fatalf("operations=%#v", result.Inventory.Operations)
	}
	if result.Inventory.Operations[0].Pointer != "#/paths/~1a/additionalOperations/CuStOm" {
		t.Fatalf("pointer=%q", result.Inventory.Operations[0].Pointer)
	}
}

func TestInventoryEntrySymlinkUsesCanonicalReferenceDirectory(t *testing.T) {
	dir := t.TempDir()
	spec := filepath.Join(dir, "spec")
	if err := os.Mkdir(spec, 0700); err != nil {
		t.Fatal(err)
	}
	entry := filepath.Join(spec, "openapi.json")
	if err := os.WriteFile(entry, []byte(`{"openapi":"3.2.1","info":{"title":"Symlink","version":"1"},"paths":{"/a":{"$ref":"./items.json#/Item"},"/b":{"$ref":"./items.json#/Item"}}}`), 0600); err != nil {
		t.Fatal(err)
	}
	for path, id := range map[string]string{filepath.Join(spec, "items.json"): "actual", filepath.Join(dir, "items.json"): "decoy"} {
		body := fmt.Sprintf(`{"Item":{"get":{"summary":%q,"responses":{"204":{"description":"OK"}}}}}`, id)
		if err := os.WriteFile(path, []byte(body), 0600); err != nil {
			t.Fatal(err)
		}
	}
	alias := filepath.Join(dir, "alias.json")
	if err := os.Symlink(entry, alias); err != nil {
		t.Fatal(err)
	}
	for _, input := range []string{entry, alias} {
		metrics := &compilationMetrics{}
		result, err := InspectInputResult(input, CompileOptions{metrics: metrics})
		if err != nil || result.Inventory == nil || len(result.Inventory.Operations) != 2 {
			t.Fatalf("input=%s result=%#v err=%v", input, result, err)
		}
		for _, operation := range result.Inventory.Operations {
			if operation.Summary != "actual" {
				t.Fatalf("input=%s read alias sibling: %#v", input, operation)
			}
		}
		if result.Inventory.DocumentsRead != 2 || metrics.ReferenceSourceDecodes != 1 {
			t.Fatalf("input=%s documents=%d metrics=%#v", input, result.Inventory.DocumentsRead, metrics)
		}
	}
}

func TestInventoryRejectsIncompleteAndAmbiguousLists(t *testing.T) {
	for _, body := range []string{
		`"/a":{"$ref":"#/paths/~1a"}`,
		`"/a":{"$ref":"#/paths/~1b","get":{}},"/b":{"get":{}}`,
		`"/a":{"$ref":"#/absent"}`,
		`"/a":{"get":false}`,
		`"/a":{"get":{"operationId":42}}`,
		`"/a":{"get":{"operationId":" "}}`,
		`"/a":{"get":{"tags":[false]}}`,
		`"/a":{"get":{"deprecated":"true"}}`,
		`"/a":{"get":{},"additionalOperations":{"GET":{}}}`,
		`"/a":{"additionalOperations":{"bad method":{}}}`,
	} {
		t.Run(body, func(t *testing.T) {
			result, err := InspectInputResult("-", CompileOptions{InputReader: strings.NewReader(`{"openapi":"3.2.1","paths":{` + body + `}}`)})
			if err != nil || result.Inventory != nil || !diagnostic.HasErrors(result.Diagnostics) {
				t.Fatalf("result=%#v err=%v", result, err)
			}
		})
	}
	result, err := InspectInputResult("-", CompileOptions{InputReader: strings.NewReader(duplicateOperationInput)})
	if err != nil || result.Inventory != nil {
		t.Fatalf("result=%#v err=%v", result, err)
	}
	found := false
	for _, value := range result.Diagnostics {
		if value.Rule == compatibility.RuleOperationIDUnique && value.Target == "" {
			found = true
		}
	}
	if !found {
		t.Fatalf("identity diagnostic missing: %#v", result.Diagnostics)
	}
}

func TestInventoryPreservesLocalContainmentAndReadOnlyLock(t *testing.T) {
	parent := t.TempDir()
	dir := filepath.Join(parent, "input")
	if err := os.Mkdir(dir, 0700); err != nil {
		t.Fatal(err)
	}
	outside := filepath.Join(parent, "outside.yaml")
	if err := os.WriteFile(outside, []byte("get: {}\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(dir, "escape.yaml")); err != nil {
		t.Fatal(err)
	}
	for _, ref := range []string{"../outside.yaml", "escape.yaml", "file://" + outside, outside} {
		body := fmt.Sprintf(`{"openapi":"3.2.1","paths":{"/a":{"$ref":%q}}}`, ref)
		entry := filepath.Join(dir, "input.json")
		if err := os.WriteFile(entry, []byte(body), 0600); err != nil {
			t.Fatal(err)
		}
		result, err := InspectInputResult(entry, CompileOptions{})
		if err != nil || result.Inventory != nil || !diagnostic.HasErrors(result.Diagnostics) {
			t.Fatalf("ref=%q result=%#v err=%v", ref, result, err)
		}
	}
	result, err := InspectInputResult("-", CompileOptions{InputReader: strings.NewReader(`{"openapi":"3.2.1","paths":{}}`), UpdateRefLock: true})
	if err != nil || result.Inventory != nil {
		t.Fatalf("lock update accepted: %#v %v", result, err)
	}
}
