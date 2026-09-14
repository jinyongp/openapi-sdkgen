package typescript

import (
	"strings"
	"testing"

	sdkgen "openapi-sdkgen/internal/compiler"
	"openapi-sdkgen/internal/compiler/ir"
)

func TestSchemaIsAlwaysDeprecatedConservatively(t *testing.T) {
	document := &ir.Document{ComponentSchemas: map[string]map[string]any{
		"Legacy":  {"type": "string", "deprecated": true},
		"Current": {"type": "string"},
		"CycleA":  {"$ref": "#/components/schemas/CycleB"},
		"CycleB":  {"$ref": "#/components/schemas/CycleA"},
	}}
	for _, test := range []struct {
		name  string
		value any
		want  bool
	}{
		{"direct", map[string]any{"deprecated": true}, true},
		{"reference", map[string]any{"$ref": "#/components/schemas/Legacy"}, true},
		{"allOf", map[string]any{"allOf": []any{map[string]any{"$ref": "#/components/schemas/Current"}, map[string]any{"$ref": "#/components/schemas/Legacy"}}}, true},
		{"oneOf all deprecated", map[string]any{"oneOf": []any{map[string]any{"const": "a", "deprecated": true}, map[string]any{"const": "b", "deprecated": true}}}, true},
		{"oneOf conditional", map[string]any{"oneOf": []any{map[string]any{"$ref": "#/components/schemas/Legacy"}, map[string]any{"$ref": "#/components/schemas/Current"}}}, false},
		{"anyOf conditional", map[string]any{"anyOf": []any{map[string]any{"$ref": "#/components/schemas/Legacy"}, map[string]any{"$ref": "#/components/schemas/Current"}}}, false},
		{"reference cycle", map[string]any{"$ref": "#/components/schemas/CycleA"}, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := schemaIsAlwaysDeprecated(document, test.value); got != test.want {
				t.Fatalf("schemaIsAlwaysDeprecated() = %v, want %v", got, test.want)
			}
		})
	}
}

func TestSourceArtifactsPropagateUnconditionalSchemaDeprecation(t *testing.T) {
	document, err := sdkgen.Compile([]byte(`{
  "openapi": "3.1.2",
  "info": {"title": "Schema deprecation", "version": "1"},
  "paths": {
    "/holder": {
      "get": {
        "operationId": "getHolder",
        "parameters": [
          {"name": "legacyParameter", "in": "query", "schema": {"$ref": "#/components/schemas/Legacy"}}
        ],
        "responses": {
          "200": {
            "description": "OK",
            "content": {"application/json": {"schema": {"$ref": "#/components/schemas/Holder"}}}
          }
        }
      }
    }
  },
  "components": {
    "schemas": {
      "Legacy": {"type": "string", "deprecated": true},
      "RefLegacy": {"$ref": "#/components/schemas/Legacy"},
      "AllLegacy": {"allOf": [{"$ref": "#/components/schemas/Legacy"}, {"type": "string"}]},
      "Conditional": {"oneOf": [{"$ref": "#/components/schemas/Legacy"}, {"type": "integer"}]},
      "Holder": {
        "type": "object",
        "properties": {
          "legacyRef": {"$ref": "#/components/schemas/Legacy"},
          "refLegacy": {"$ref": "#/components/schemas/RefLegacy"},
          "allLegacy": {"$ref": "#/components/schemas/AllLegacy"},
          "conditional": {"$ref": "#/components/schemas/Conditional"}
        }
      }
    }
  }
}`))
	if err != nil {
		t.Fatal(err)
	}
	artifacts, err := SourceArtifacts(document)
	if err != nil {
		t.Fatal(err)
	}

	schemaSource := schemaProjectionSource(artifacts)
	if got := strings.Count(schemaSource, "@deprecated This OpenAPI schema is deprecated."); got != 6 {
		t.Fatalf("deprecated schema projection tags = %d, want 6:\n%s", got, schemaSource)
	}
	for _, name := range []string{"legacyRef", "refLegacy", "allLegacy"} {
		if !jsDocForMarkerContains(schemaSource, "OpenAPI property `"+name+"`.", "@deprecated This OpenAPI value is deprecated.") {
			t.Fatalf("property %q did not inherit unconditional schema deprecation:\n%s", name, schemaSource)
		}
	}
	if jsDocForMarkerContains(schemaSource, "OpenAPI property `conditional`.", "@deprecated") {
		t.Fatalf("conditional schema deprecation became unconditional:\n%s", schemaSource)
	}

	clientSource := clientSemanticSource(artifacts)
	if !jsDocForMarkerContains(clientSource, "Query parameter `legacyParameter`.", "@deprecated This OpenAPI value is deprecated.") {
		t.Fatalf("parameter schema deprecation missing:\n%s", clientSource)
	}
	metadataSource := string(artifactByPath(t, artifacts, "metadata.ts"))
	if !strings.Contains(metadataSource, `["deprecated", true]`) {
		t.Fatalf("lossless metadata dropped schema deprecation:\n%s", metadataSource)
	}
}

func TestAnnotatedEnumValuesRecognitionIsConservative(t *testing.T) {
	valid := map[string]any{"oneOf": []any{
		map[string]any{"const": "ACTIVE", "title": "Active"},
		map[string]any{"const": "LEGACY", "description": "Legacy", "deprecated": true},
	}}
	for _, line := range []string{"3.1", "3.2"} {
		document := &ir.Document{OpenAPIVersionLine: line}
		values, docs, ok, err := annotatedEnumValues(document, valid)
		if err != nil || !ok || len(values) != 2 || values[0] != "ACTIVE" || values[1] != "LEGACY" {
			t.Fatalf("%s annotated enum = %#v, %#v, %v, %v", line, values, docs, ok, err)
		}
		if docs["ACTIVE"].title != "Active" || docs["LEGACY"].description != "Legacy" || !docs["LEGACY"].deprecated {
			t.Fatalf("%s annotated enum docs = %#v", line, docs)
		}
	}
	if _, _, ok, err := annotatedEnumValues(&ir.Document{OpenAPIVersionLine: "3.0"}, valid); err != nil || ok {
		t.Fatalf("OpenAPI 3.0 annotated enum promotion = %v, %v", ok, err)
	}
	for _, test := range []struct {
		name   string
		schema map[string]any
	}{
		{"duplicate const", map[string]any{"oneOf": []any{map[string]any{"const": "x"}, map[string]any{"const": "x", "deprecated": true}}}},
		{"missing const", map[string]any{"oneOf": []any{map[string]any{"const": "x"}, map[string]any{"title": "missing"}}}},
		{"reference branch", map[string]any{"oneOf": []any{map[string]any{"const": "x"}, map[string]any{"$ref": "#/components/schemas/Other"}}}},
		{"assertion branch", map[string]any{"oneOf": []any{map[string]any{"const": "x", "type": "string"}, map[string]any{"const": "y"}}}},
		{"both compositions", map[string]any{"oneOf": []any{map[string]any{"const": "x"}}, "anyOf": []any{map[string]any{"const": "y"}}}},
	} {
		t.Run(test.name, func(t *testing.T) {
			if _, _, ok, err := annotatedEnumValues(&ir.Document{OpenAPIVersionLine: "3.1"}, test.schema); err != nil || ok {
				t.Fatalf("annotated enum promotion = %v, %v", ok, err)
			}
		})
	}
}

func TestSourceArtifactsEmitAnnotatedEnumDeprecationDocumentation(t *testing.T) {
	for _, test := range []struct {
		version     string
		composition string
	}{
		{"3.1.2", "oneOf"},
		{"3.2.0", "anyOf"},
	} {
		t.Run(test.version, func(t *testing.T) {
			document, err := sdkgen.Compile([]byte(`{
  "openapi": "` + test.version + `",
  "info": {"title": "Annotated enums", "version": "1"},
  "paths": {
    "/status": {
      "get": {
        "operationId": "getStatus",
        "responses": {"200": {"description": "OK", "content": {"application/json": {"schema": {"$ref": "#/components/schemas/Status"}}}}}
      }
    },
    "/legacy": {
      "get": {
        "operationId": "getLegacyStatus",
        "responses": {"200": {"description": "OK", "content": {"application/json": {"schema": {"$ref": "#/components/schemas/LegacyStatus"}}}}}
      }
    }
  },
  "components": {
    "schemas": {
      "Status": {"` + test.composition + `": [
        {"const": "ACTIVE", "title": "Active", "description": "Current status."},
        {"const": "LEGACY", "title": "Legacy", "description": "Previous status.", "deprecated": true},
        {"const": 7, "description": "Numeric status.", "deprecated": true}
      ]},
      "LegacyStatus": {"type": "string", "enum": ["OLD", "OLDER"], "deprecated": true}
    }
  }
}`))
			if err != nil {
				t.Fatal(err)
			}
			compileTypeScriptArtifacts(t, document)
			artifacts, err := SourceArtifacts(document)
			if err != nil {
				t.Fatal(err)
			}
			enumsSource := string(artifactByPath(t, artifacts, "internal/enums.ts"))
			for _, wanted := range []string{
				`["ACTIVE", "LEGACY", 7] as const`,
				"* Active\n     *\n     * Current status.",
				"* Legacy\n     *\n     * Previous status.\n     *\n     * @deprecated This OpenAPI enum value is deprecated.",
			} {
				if !strings.Contains(enumsSource, wanted) {
					t.Fatalf("%s enums source missing %q:\n%s", test.version, wanted, enumsSource)
				}
			}
			if strings.Contains(enumsSource, `readonly "7":`) {
				t.Fatalf("%s invented a member property for a non-string enum value:\n%s", test.version, enumsSource)
			}
			if !jsDocBeforeMarkerContains(enumsSource, `readonly "LegacyStatus": {`, "@deprecated This OpenAPI enum is deprecated.") {
				t.Fatalf("%s plain deprecated enum component lacks deprecation JSDoc:\n%s", test.version, enumsSource)
			}
			if jsDocBeforeMarkerContains(enumsSource, `readonly "OLD": "OLD"`, "@deprecated This OpenAPI enum value is deprecated.") {
				t.Fatalf("%s schema deprecation incorrectly became member deprecation:\n%s", test.version, enumsSource)
			}
			schemaSource := schemaProjectionSource(artifacts)
			if !strings.Contains(schemaSource, `"ACTIVE" | "LEGACY" | 7`) {
				t.Fatalf("%s annotated enum value union missing:\n%s", test.version, schemaSource)
			}
		})
	}
}

func jsDocBeforeMarkerContains(source, marker, wanted string) bool {
	markerIndex := strings.Index(source, marker)
	if markerIndex < 0 {
		return false
	}
	start := strings.LastIndex(source[:markerIndex], "/**")
	if start < 0 {
		return false
	}
	endOffset := strings.Index(source[start:markerIndex], "*/")
	if endOffset < 0 {
		return false
	}
	end := start + endOffset + len("*/")
	return strings.Contains(source[start:end], wanted)
}

func jsDocForMarkerContains(source, marker, wanted string) bool {
	markerIndex := strings.Index(source, marker)
	if markerIndex < 0 {
		return false
	}
	start := strings.LastIndex(source[:markerIndex], "/**")
	if start < 0 {
		return false
	}
	endOffset := strings.Index(source[markerIndex:], "*/")
	if endOffset < 0 {
		return false
	}
	end := markerIndex + endOffset + len("*/")
	return strings.Contains(source[start:end], wanted)
}
