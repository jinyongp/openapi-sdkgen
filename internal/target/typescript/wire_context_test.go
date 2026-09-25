package typescript

import (
	"os"
	"strings"
	"testing"

	compiler "openapi-sdkgen/internal/compiler"
)

func TestWireRenderContextPropagatesNestedPropertyNeeds(t *testing.T) {
	child := map[string]any{"type": "object", "properties": map[string]any{"__proto__": map[string]any{"type": "string"}}}
	cases := map[string]any{
		"direct":           child,
		"ref siblings":     map[string]any{"$ref": "#/components/schemas/Parent", "allOf": []any{child}},
		"dynamic siblings": map[string]any{"x-sdkgen-dynamic-reference": map[string]any{"anchor": "node", "reference": "#/components/schemas/Parent"}, "allOf": []any{child}},
	}
	for _, key := range []string{"contentSchema", "propertyNames", "items", "contains", "additionalProperties", "unevaluatedProperties", "unevaluatedItems", "not", "if", "then", "else"} {
		cases[key] = map[string]any{key: child}
	}
	for _, key := range []string{"allOf", "oneOf", "anyOf", "prefixItems"} {
		cases[key] = map[string]any{key: []any{child}}
	}
	for _, key := range []string{"patternProperties", "dependentSchemas"} {
		cases[key] = map[string]any{key: map[string]any{"nested": child}}
	}
	for name, schema := range cases {
		t.Run(name, func(t *testing.T) {
			for _, mode := range []wirePropertiesMode{wirePropertiesLiteral, wirePropertiesConstructed} {
				wire := newWireRenderContext(mode)
				source, err := wire.wireSchemaDescriptor(schema, projectionInput)
				if err != nil || !wire.usesProperties {
					t.Fatalf("mode %d needs = %t, error = %v", mode, wire.usesProperties, err)
				}
				literal := strings.Contains(source, "Object.fromEntries<WireProperty>")
				constructed := strings.Contains(source, "__sdkgen_Properties(")
				if literal != (mode == wirePropertiesLiteral) || constructed != (mode == wirePropertiesConstructed) {
					t.Fatalf("mode %d crossed owner policy: %s", mode, source)
				}
			}
		})
	}
}

func TestWireRenderContextDoesNotInferNeedsFromOpaqueDataOrFilteredProperties(t *testing.T) {
	for _, schema := range []any{
		false, map[string]any{}, map[string]any{"$ref": "#/components/schemas/Parent"},
		map[string]any{"const": map[string]any{"properties": map[string]any{"x": "__sdkgen_Properties"}}},
		map[string]any{"properties": map[string]any{"hidden": map[string]any{"readOnly": true}}},
	} {
		wire := newWireRenderContext(wirePropertiesConstructed)
		if _, err := wire.wireSchemaDescriptor(schema, projectionInput); err != nil {
			t.Fatal(err)
		}
		if wire.usesProperties {
			t.Fatalf("schema incorrectly requires helper: %#v", schema)
		}
	}
	wire := newWireRenderContext(wirePropertiesMode(255))
	if _, err := wire.propertyExpression([]runtimeProperty{{key: "x", value: "{}"}}); err == nil || wire.usesProperties {
		t.Fatal("invalid owner mode did not fail without publishing needs")
	}
}

func TestWirePropertyImportsFollowActualEmissionOwner(t *testing.T) {
	for _, fixture := range []string{"representation", "lifecycle", "baseline-oas31"} {
		t.Run(fixture, func(t *testing.T) {
			input, err := os.ReadFile("../../../test/typescript/fixtures/" + fixture + ".openapi.json")
			if err != nil {
				t.Fatal(err)
			}
			document, err := compiler.Compile(input)
			if err != nil {
				t.Fatal(err)
			}
			artifacts, err := sourceArtifacts(document, true)
			if err != nil {
				t.Fatal(err)
			}
			for _, artifact := range artifacts {
				source := string(artifact.Data)
				switch {
				case strings.HasPrefix(artifact.Path, "internal/schemas/"):
					if strings.Contains(source, "wire-properties.js") || strings.Contains(source, "createWireProperties") {
						t.Fatalf("named schema gained a runtime helper dependency: %s", artifact.Path)
					}
					if strings.Contains(source, "Object.fromEntries<WireProperty>") && !strings.Contains(source, "import type { WireSchema, WireProperty }") {
						t.Fatalf("missing literal property type import: %s", artifact.Path)
					}
				case strings.HasPrefix(artifact.Path, "internal/operations/"):
					needs := strings.Contains(source, "/* @__PURE__ */ __sdkgen_Properties(")
					if needs != strings.Contains(source, "createWireProperties as __sdkgen_Properties") {
						t.Fatalf("operation helper import/usage mismatch: %s", artifact.Path)
					}
					if strings.Contains(source, "wire-properties.js") {
						t.Fatalf("extra operation helper edge: %s", artifact.Path)
					}
				case artifact.Path == "server/callbacks.ts" || artifact.Path == "server/webhooks.ts":
					needs := strings.Contains(source, "/* @__PURE__ */ __sdkgen_Properties(")
					if needs != strings.Contains(source, "import { wireProperties as __sdkgen_Properties }") {
						t.Fatalf("server helper import/usage mismatch: %s", artifact.Path)
					}
				case artifact.Path == "internal/index.ts":
					if strings.Contains(source, "wireProperties") || strings.Contains(source, "createWireProperties") {
						t.Fatal("helper leaked into public root")
					}
				}
			}
		})
	}
}

func TestWirePropertyNeedsIncludeMultipartHeaderSchemas(t *testing.T) {
	wire := newWireRenderContext(wirePropertiesConstructed)
	source, err := wire.requestBodyWireEncodings(nil, map[string]any{
		"encoding": map[string]any{"part": map[string]any{"headers": map[string]any{"X-Metadata": map[string]any{
			"schema": map[string]any{"type": "object", "properties": map[string]any{"value": map[string]any{"type": "string"}}},
		}}}},
	})
	if err != nil || !wire.usesProperties || !strings.Contains(source, "__sdkgen_Properties(") {
		t.Fatalf("multipart header lost owner needs: %s, %v", source, err)
	}
}
