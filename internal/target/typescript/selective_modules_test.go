package typescript

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"reflect"
	"regexp"
	"strings"
	"testing"

	"openapi-sdkgen/internal/compiler"
	"openapi-sdkgen/internal/diagnostic"
)

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
			if !strings.Contains(source, "export const entry =") || strings.Contains(source, "export const then") {
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
		artifacts, err := SourceArtifacts(document)
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
