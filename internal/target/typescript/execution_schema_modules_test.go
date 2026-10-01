package typescript

import (
	"fmt"
	pathpkg "path"
	"reflect"
	"regexp"
	"strings"
	"testing"

	"openapi-sdkgen/internal/compiler"
	"openapi-sdkgen/internal/diagnostic"
)

func TestRepeatedExecutionClosuresShareOnlyTheirProjectedSchemas(t *testing.T) {
	input := []byte(`{
  "openapi":"3.1.0","info":{"title":"Shared closures","version":"1"},
  "paths":{
    "/a":{"post":{"operationId":"a","requestBody":{"content":{"application/json":{"schema":{"$ref":"#/components/schemas/Root"}}}},"responses":{"200":{"description":"ok","content":{"application/json":{"schema":{"$ref":"#/components/schemas/Root"}}}}}}},
    "/b":{"post":{"operationId":"b","requestBody":{"content":{"application/json":{"schema":{"$ref":"#/components/schemas/Root"}}}},"responses":{"200":{"description":"ok","content":{"application/json":{"schema":{"$ref":"#/components/schemas/Root"}}}}}}}
  },
  "components":{"schemas":{
    "Root":{"type":"object","properties":{"shared":{"$ref":"#/components/schemas/Shared"},"input":{"writeOnly":true,"$ref":"#/components/schemas/InputOnly"},"output":{"readOnly":true,"$ref":"#/components/schemas/OutputOnly"}}},
    "Shared":{"type":"string"},"InputOnly":{"type":"string"},"OutputOnly":{"type":"string"},"Unrelated":{"type":"boolean"}
  }}
}`)
	document, err := sdkgen.Compile(input)
	if err != nil {
		t.Fatal(err)
	}
	plan, diagnostics, err := prepareSourcePlan(document, false)
	if err != nil || diagnostic.HasErrors(diagnostics) {
		t.Fatalf("prepare: %v %v", err, diagnostics)
	}
	if len(plan.executionSchemas) != 2 {
		t.Fatalf("shared projected closures = %d, want 2", len(plan.executionSchemas))
	}
	artifacts, err := emitSourcePlan(plan)
	if err != nil {
		t.Fatal(err)
	}
	again, err := emitSourcePlan(plan)
	if err != nil || !reflect.DeepEqual(artifacts, again) {
		t.Fatalf("emission is not stable: %v", err)
	}
	byPath := make(map[string]string)
	for _, artifact := range artifacts {
		byPath[artifact.Path] = string(artifact.Data)
	}
	for _, bundle := range plan.executionSchemas {
		want := []string{"InputOnly", "Root", "Shared"}
		excluded := "OutputOnly"
		if bundle.projection == "output" {
			want, excluded = []string{"OutputOnly", "Root", "Shared"}, "InputOnly"
		}
		if !reflect.DeepEqual(bundle.names, want) || strings.Contains(byPath[bundle.path], excluded) || strings.Contains(byPath[bundle.path], "Unrelated") {
			t.Fatalf("projection dependencies widened: %#v\n%s", bundle, byPath[bundle.path])
		}
		for index, name := range want {
			entry := fmt.Sprintf("[%s, schema%d]", quoteTS(name), index)
			if !strings.Contains(byPath[bundle.path], entry) || !strings.Contains(byPath[bundle.path], bundle.projection+"WireSchema") {
				t.Fatalf("projected schema binding missing: %s", entry)
			}
		}
	}
	for _, module := range plan.modules.operations {
		execution := plan.executions[module.routeKey]
		if execution.inputBundle == "" || execution.outputBundle == "" || execution.inputBundle == execution.outputBundle {
			t.Fatalf("missing independent projection modules: %#v", execution)
		}
		provider := byPath[operationExecutionArtifactPath(module)]
		if !strings.Contains(provider, "bindBase(request, inputSchemas, outputSchemas)") || strings.Contains(provider, "Object.fromEntries") {
			t.Fatalf("provider did not use shared maps: %s", provider)
		}
	}
	imports := regexp.MustCompile(`(?:\bfrom\s+|\bimport\s*\(?\s*)["'](\.[^"']+)["']`)
	for artifact, source := range byPath {
		for _, match := range imports.FindAllStringSubmatch(source, -1) {
			if strings.HasSuffix(match[1], ".js") {
				target := pathpkg.Join(pathpkg.Dir(artifact), strings.TrimSuffix(match[1], ".js")+".ts")
				if _, exists := byPath[target]; !exists {
					t.Fatalf("%s imports missing %s", artifact, target)
				}
			}
		}
	}
}
