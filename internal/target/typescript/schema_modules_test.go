package typescript

import (
	"strings"
	"testing"

	sdkgen "openapi-sdkgen/internal/compiler"
	"openapi-sdkgen/internal/compiler/ir"
)

func TestSchemaModulesUseInlineAndSharedTypeReferencesWithoutRuntimeEdges(t *testing.T) {
	document := &ir.Document{ComponentSchemas: map[string]map[string]any{
		"Shared": {"type": "string"},
		"Single": {"type": "object", "properties": map[string]any{
			"value": map[string]any{"$ref": "#/components/schemas/Shared"},
		}},
		"Repeated": {"type": "object", "properties": map[string]any{
			"first":  map[string]any{"$ref": "#/components/schemas/Shared"},
			"second": map[string]any{"$ref": "#/components/schemas/Shared"},
		}},
		"Node": {"type": "object", "properties": map[string]any{
			"next": map[string]any{"$ref": "#/components/schemas/Node"},
		}},
		"MutualA": {"type": "object", "properties": map[string]any{
			"other": map[string]any{"$ref": "#/components/schemas/MutualB"},
		}},
		"MutualB": {"type": "object", "properties": map[string]any{
			"other": map[string]any{"$ref": "#/components/schemas/MutualA"},
		}},
	}}
	artifacts, err := SourceArtifacts(document)
	if err != nil {
		t.Fatal(err)
	}

	single := string(artifactByPath(t, artifacts, "internal/schemas/single.ts"))
	for _, expected := range []string{
		`readonly "value"?: import("./shared.js").Input`,
		`readonly "value"?: import("./shared.js").Output`,
	} {
		if !strings.Contains(single, expected) {
			t.Fatalf("single schema reference was not rendered inline as %q:\n%s", expected, single)
		}
	}
	if strings.Contains(single, "import type {") {
		t.Fatalf("single-use schema reference created an alias import:\n%s", single)
	}

	repeated := string(artifactByPath(t, artifacts, "internal/schemas/repeated.ts"))
	for _, expected := range []string{
		"import type { Input as ",
		"import type { Output as ",
		`from "./shared.js"`,
	} {
		if !strings.Contains(repeated, expected) {
			t.Fatalf("repeated schema reference is missing %q:\n%s", expected, repeated)
		}
	}
	if strings.Contains(repeated, `import("./shared.js")`) {
		t.Fatalf("repeated schema reference did not share an alias:\n%s", repeated)
	}

	node := string(artifactByPath(t, artifacts, "internal/schemas/node.ts"))
	if strings.Contains(node, `from "./node.js"`) || !strings.Contains(node, `readonly "next"?: Input`) || !strings.Contains(node, `readonly "next"?: Output`) {
		t.Fatalf("self-recursive schema did not stay local:\n%s", node)
	}

	index := string(artifactByPath(t, artifacts, "internal/schemas/index.ts"))
	if strings.Contains(index, "import ") || strings.Contains(index, "export const ") || !strings.Contains(index, `readonly "Single": {`) || !strings.Contains(index, `import("./single.js").Input`) {
		t.Fatalf("schema type registry is not a thin inline-import registry:\n%s", index)
	}
	for _, artifact := range artifacts {
		if artifact.Path == "internal/types.ts" {
			t.Fatal("legacy internal/types.ts artifact remains")
		}
	}
	_ = compileTypeScriptArtifacts(t, document)
}

func TestSchemaModulesKeepWireOnlyComponentsOutOfPublicRegistry(t *testing.T) {
	document := &ir.Document{
		ComponentSchemas: map[string]map[string]any{
			"WireOnly": {"type": "object", "properties": map[string]any{"id": map[string]any{"type": "string"}}},
		},
		Operations: []ir.Operation{
			{
				OperationID: "visible", Method: "GET", Path: "/visible",
				Raw: map[string]any{"responses": map[string]any{"200": map[string]any{"content": map[string]any{"application/json": map[string]any{"schema": map[string]any{"type": "string"}}}}}},
			},
			{
				OperationID: "hidden", Method: "GET", Path: "/hidden", Visibility: "hidden",
				Raw: map[string]any{"x-sdk-visibility": "hidden", "responses": map[string]any{"200": map[string]any{"content": map[string]any{"application/json": map[string]any{"schema": map[string]any{"$ref": "#/components/schemas/WireOnly"}}}}}},
			},
		},
	}
	artifacts, err := SourceArtifacts(document)
	if err != nil {
		t.Fatal(err)
	}
	for _, artifact := range artifacts {
		if artifact.Path == "internal/schemas/wire-only.ts" {
			t.Fatalf("hidden-only schema leaf was emitted:\n%s", artifact.Data)
		}
	}
	index := string(artifactByPath(t, artifacts, "internal/schemas/index.ts"))
	if strings.Contains(index, `readonly "WireOnly":`) {
		t.Fatalf("wire-only component leaked into public Components:\n%s", index)
	}
	wire := string(artifactByPath(t, artifacts, "internal/schemas/wire.ts"))
	if strings.Contains(wire, `outputWireSchema as `) || strings.Contains(wire, `from "./wire-only.js"`) {
		t.Fatalf("hidden-only component leaked into the wire registry:\n%s", wire)
	}
}

func TestSchemaModulesPlanNormalizedDynamicReferencesWithoutOpaqueLiteralReferences(t *testing.T) {
	document, err := sdkgen.Compile([]byte(`{
  "openapi": "3.1.0",
  "info": {"title": "Dynamic reachability", "version": "1"},
  "paths": {
    "/node": {"get": {
      "operationId": "getNode",
      "responses": {"200": {
        "description": "OK",
        "content": {"application/json": {"schema": {
          "$dynamicRef": "https://schemas.example.test/node#node",
          "example": {"$ref": "#/components/schemas/LiteralOnly"}
        }}}
      }}
    }}
  },
  "components": {"schemas": {
    "Node": {
      "$id": "https://schemas.example.test/node",
      "$dynamicAnchor": "node",
      "type": "object",
      "properties": {"id": {"type": "string"}}
    },
    "LiteralOnly": {"type": "boolean"}
  }}
}`))
	if err != nil {
		t.Fatal(err)
	}
	artifacts, err := SourceArtifacts(document)
	if err != nil {
		t.Fatal(err)
	}
	_ = artifactByPath(t, artifacts, "internal/schemas/node.ts")
	for _, artifact := range artifacts {
		if artifact.Path == "internal/schemas/literal-only.ts" {
			t.Fatalf("opaque literal reference emitted a schema module:\n%s", artifact.Data)
		}
	}
}
