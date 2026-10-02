package typescript

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	pathpkg "path"
	"reflect"
	"regexp"
	"strings"
	"testing"

	"openapi-sdkgen/internal/compiler"
	"openapi-sdkgen/internal/diagnostic"
)

func TestSelectiveStaticArtifactBoundsTheFinalPath(t *testing.T) {
	paths := make(map[string]bool)
	for _, tail := range []string{"ite", "item", "items", "other"} {
		route := "/" + strings.Repeat("segment/", 26) + tail
		module := operationModulePlan{routeKey: "GET " + route, path: operationArtifactBase(route, "GET")}
		unbounded := "selective/operations/" + strings.TrimPrefix(module.path, "internal/operations/")
		got := selectiveStaticArtifact(module)
		if err := validateArtifactPath(got); err != nil {
			t.Fatalf("final path %q (%d bytes): %v", got, len(got), err)
		}
		if len(unbounded) <= maxArtifactPathBytes && got != unbounded {
			t.Fatalf("existing portable path changed: %q -> %q", unbounded, got)
		}
		if len(unbounded) > maxArtifactPathBytes && !strings.HasPrefix(got, "selective/operations/route-") {
			t.Fatalf("overlong static path was not shortened: %q", got)
		}
		if got != selectiveStaticArtifact(module) || paths[got] {
			t.Fatalf("static path is unstable or collides: %q", got)
		}
		paths[got] = true
		if pathpkg.Base(got) != pathpkg.Base(module.path) {
			t.Fatalf("allocated filename changed: %q -> %q", module.path, got)
		}
	}
	module := operationModulePlan{routeKey: "GET /long", path: "internal/operations/" + strings.Repeat("segment/", 24) + "boundary/get-123456789abc.ts"}
	if got := selectiveStaticArtifact(module); !strings.HasPrefix(got, "selective/operations/route-") || pathpkg.Base(got) != "get-123456789abc.ts" {
		t.Fatalf("collision suffix was lost: %q", got)
	}
}

func TestSelectiveBoundaryArtifactsAndTheirImportsAreEmitted(t *testing.T) {
	paths := make(map[string]any)
	for _, tail := range []string{"ite", "item", "items", "other"} {
		paths["/"+strings.Repeat("segment/", 26)+tail] = map[string]any{
			"get": map[string]any{"operationId": tail, "responses": map[string]any{"204": map[string]any{"description": "ok"}}},
		}
	}
	longRoute := "/" + strings.Repeat("segment/", 26) + "items"
	fallbackDirectory := "route-" + shortArtifactHash("GET "+longRoute)
	for _, route := range []string{"/" + fallbackDirectory, "/" + fallbackDirectory + "-1"} {
		paths[route] = map[string]any{
			"get": map[string]any{"responses": map[string]any{"204": map[string]any{"description": "ok"}}},
		}
	}
	input, err := json.Marshal(map[string]any{"openapi": "3.0.3", "info": map[string]any{"title": "Boundary", "version": "1"}, "paths": paths})
	if err != nil {
		t.Fatal(err)
	}
	document, err := sdkgen.Compile(input)
	if err != nil {
		t.Fatal(err)
	}
	plan, diagnostics, err := prepareSourcePlan(document, false)
	if err != nil || diagnostic.HasErrors(diagnostics) {
		t.Fatalf("prepare: %v %v", err, diagnostics)
	}
	artifacts, err := emitSourcePlan(plan)
	if err != nil {
		t.Fatal(err)
	}
	byPath := make(map[string]string)
	for _, artifact := range artifacts {
		if err := validateArtifactPath(artifact.Path); err != nil {
			t.Fatalf("%s: %v", artifact.Path, err)
		}
		if _, exists := byPath[artifact.Path]; exists {
			t.Fatalf("duplicate emitted path: %s", artifact.Path)
		}
		byPath[artifact.Path] = string(artifact.Data)
	}
	for _, module := range plan.modules.operations {
		static := selectiveStaticArtifact(module)
		if !strings.Contains(byPath[static], "export const operation: OperationReference<") {
			t.Fatalf("static reference missing at planned path: %s", static)
		}
		if strings.HasPrefix(module.routeKey, "GET /"+fallbackDirectory) && static != "selective/operations/"+strings.TrimPrefix(module.path, "internal/operations/") {
			t.Fatalf("existing portable route moved: %s -> %s", module.routeKey, static)
		}
		if module.routeKey == "GET "+longRoute && static != "selective/operations/"+fallbackDirectory+"-2/get.ts" {
			t.Fatalf("shortened route did not avoid existing directories: %s", static)
		}
	}
	imports := regexp.MustCompile(`(?:\bfrom\s+|\bimport\s*\(?\s*)["'](\.[^"']+)["']`)
	for artifact, source := range byPath {
		for _, match := range imports.FindAllStringSubmatch(source, -1) {
			if !strings.HasSuffix(match[1], ".js") {
				continue
			}
			target := pathpkg.Join(pathpkg.Dir(artifact), strings.TrimSuffix(match[1], ".js")+".ts")
			if _, exists := byPath[target]; !exists {
				t.Fatalf("%s imports missing target %s", artifact, target)
			}
		}
	}
}

func TestSelectiveLookupPathsPreserveExactKeysAndPortableSegments(t *testing.T) {
	for _, kind := range []string{"operation", "route"} {
		for _, key := range []string{"GET /todos/{id}", "then", "__proto__", "a\x00b", "../outside", "한글", "😀", "é", "e\u0301"} {
			path, err := selectiveLookupArtifact(kind, key)
			if err != nil {
				t.Fatal(err)
			}
			if err := validateArtifactPath(path); err != nil {
				t.Fatalf("%q: %v", path, err)
			}
			sum := sha256.Sum256([]byte(kind + "\x00" + key))
			encoded := hex.EncodeToString(sum[:])
			prefix := "o-"
			if kind == "route" {
				prefix = "r-"
			}
			if path != "selective/lookup/"+prefix+encoded[:16]+"/"+encoded[16:]+".ts" {
				t.Fatalf("wrong exact framing: %q", path)
			}
		}
	}
	if _, err := selectiveLookupArtifact("other", "valid"); err == nil {
		t.Fatal("invalid kind accepted")
	}
	if _, err := selectiveLookupArtifact("route", string([]byte{0xff})); err == nil {
		t.Fatal("invalid UTF-8 accepted")
	}
}

func TestSelectiveArtifactsUseFixedExportsAndPlanOwnedPaths(t *testing.T) {
	input, err := os.ReadFile("../../../test/typescript/fixtures/selection-public.openapi.json")
	if err != nil {
		t.Fatal(err)
	}
	document, err := sdkgen.Compile(input)
	if err != nil {
		t.Fatal(err)
	}
	plan, diagnostics, err := prepareSourcePlan(document, false)
	if err != nil || diagnostic.HasErrors(diagnostics) {
		t.Fatalf("prepare: %v %v", err, diagnostics)
	}
	artifacts, err := emitSourcePlan(plan)
	if err != nil {
		t.Fatal(err)
	}
	byPath := make(map[string]string)
	for _, artifact := range artifacts {
		byPath[artifact.Path] = string(artifact.Data)
	}
	for _, path := range plan.modules.selective {
		if _, exists := byPath[path.base]; !exists {
			t.Fatalf("planned selective artifact missing: %s", path.base)
		}
	}
	for _, item := range plan.manifest.Operations {
		for _, identity := range []struct{ kind, key string }{{"route", manifestRouteKey(item)}, {"operation", item.OperationID}} {
			if identity.key == "" {
				continue
			}
			path, err := selectiveLookupArtifact(identity.kind, identity.key)
			if err != nil {
				t.Fatal(err)
			}
			source, exists := byPath[path]
			if !exists {
				t.Fatalf("lookup absent: %s", path)
			}
			if !strings.Contains(source, "export const entry: OperationLookupEntry =") || strings.Contains(source, "export const then") {
				t.Fatalf("lookup has unsafe export ABI: %s", source)
			}
			if !strings.Contains(source, "key: "+quoteTS(identity.key)) {
				t.Fatalf("lookup lost identity %q", identity.key)
			}
		}
	}
	for _, operation := range plan.modules.operations {
		if strings.Contains(byPath["selective/index.ts"], quoteTS(operation.routeKey)) {
			t.Fatal("runtime bootstrap contains full route registry")
		}
	}
	if strings.Contains(byPath["selective/all.ts"], "/executions/") {
		t.Fatal("names-only enumeration imports execution code")
	}
	again, err := emitSourcePlan(plan)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(artifacts, again) {
		t.Fatal("selective emission is not repeatable")
	}
	plan.modules.selective = append(plan.modules.selective, plan.modules.selective[0])
	if plan.modules.validate() == nil {
		t.Fatal("selective path ownership collision was not checked")
	}
}

func TestSelectiveGenerationCoversOutputAndIsSharedByProviders(t *testing.T) {
	var previous string
	for _, title := range []string{"one", "two"} {
		document, err := sdkgen.Compile([]byte(`{"openapi":"3.1.1","info":{"title":"` + title + `","version":"1"},"paths":{"/one":{"get":{"operationId":"one","responses":{"204":{"description":"ok"}}}}}}`))
		if err != nil {
			t.Fatal(err)
		}
		artifacts, err := sourceArtifactsWithMetadata(document)
		if err != nil {
			t.Fatal(err)
		}
		pattern := regexp.MustCompile(`generation: "([a-f0-9]{64})"`)
		identity := ""
		for _, artifact := range artifacts {
			if artifact.Path == "selective/index.ts" {
				match := pattern.FindSubmatch(artifact.Data)
				if len(match) != 2 {
					t.Fatal("missing generated code identity")
				}
				identity = string(match[1])
			}
		}
		if identity == "" || identity == previous {
			t.Fatal("changed output retained a generation identity")
		}
		for _, artifact := range artifacts {
			if strings.HasPrefix(artifact.Path, "internal/executions/") || strings.HasPrefix(artifact.Path, "selective/lookup/") {
				if !strings.Contains(string(artifact.Data), `generation: "`+identity+`"`) {
					t.Fatalf("mixed code identity: %s", artifact.Path)
				}
			}
		}
		previous = identity
	}
}
