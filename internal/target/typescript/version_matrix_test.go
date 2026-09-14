package typescript

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	sdkgen "openapi-sdkgen/internal/compiler"
)

func TestVersionFeatureFixturesGenerateForTypeScript(t *testing.T) {
	for _, test := range []struct {
		fixture     string
		version     string
		want        string
		operationID string
		route       string
	}{
		{"oas30-sdk.json", "3.0.3", "string | null", "listWidgets", "GET /widgets"},
		{"oas31-sdk.json", "3.1.1", "string | null", "getWidget", "GET /widgets"},
		{"oas32-sdk.json", "3.2.0", `method: "QUERY"`, "queryWidgets", "QUERY /widgets"},
	} {
		t.Run(test.version, func(t *testing.T) {
			data, err := os.ReadFile(filepath.Join("..", "..", "compiler", "openapi", "testdata", test.fixture))
			if err != nil {
				t.Fatal(err)
			}
			document, err := sdkgen.Compile(data)
			if err != nil {
				t.Fatal(err)
			}
			if document.OpenAPIVersion != test.version {
				t.Fatalf("version = %q, want %q", document.OpenAPIVersion, test.version)
			}
			typescriptArtifacts, err := SourceArtifacts(document)
			if err != nil {
				t.Fatal(err)
			}
			if source := schemaProjectionSource(typescriptArtifacts) + clientSemanticSource(typescriptArtifacts); !strings.Contains(source, test.want) {
				t.Fatalf("TypeScript source missing %q:\n%s", test.want, source)
			}
			probe := fmt.Sprintf(`
import type { Client, OperationInput, OperationOutput, RouteInput, RouteOutput } from "./index.js"
type Equal<Left, Right> = (<Value>() => Value extends Left ? 1 : 2) extends (<Value>() => Value extends Right ? 1 : 2) ? true : false
type Expect<Value extends true> = Value
type Method = Client["$operations"][%q]
type Assertions = [
  Expect<Equal<OperationInput<%q>, OperationInput<Method>>>,
  Expect<Equal<OperationInput<%q>, RouteInput<%q>>>,
  Expect<Equal<OperationOutput<%q>, RouteOutput<%q>>>,
]
void (null as unknown as Assertions)
`, test.operationID, test.operationID, test.operationID, test.route, test.operationID, test.route)
			compileTypeScriptArtifactsWithProbe(t, document, "operation-types.probe.ts", probe)
		})
	}
}

func TestSchemaDeprecationEmitsAcrossSupportedVersionLines(t *testing.T) {
	for _, version := range []string{"3.0.4", "3.1.2", "3.2.0"} {
		t.Run(version, func(t *testing.T) {
			document, err := sdkgen.Compile([]byte(fmt.Sprintf(`{
  "openapi": %q,
  "info": {"title": "Deprecated schemas", "version": "1"},
  "paths": {
    "/legacy": {
      "get": {
        "operationId": "getDeprecatedSchema",
        "responses": {"200": {"description": "OK", "content": {"application/json": {"schema": {"$ref": "#/components/schemas/Legacy"}}}}}
      }
    }
  },
  "components": {"schemas": {"Legacy": {"type": "string", "deprecated": true}}}
}`, version)))
			if err != nil {
				t.Fatal(err)
			}
			artifacts, err := SourceArtifacts(document)
			if err != nil {
				t.Fatal(err)
			}
			schemaSource := schemaProjectionSource(artifacts)
			if !strings.Contains(schemaSource, "@deprecated This OpenAPI schema is deprecated.") || !strings.Contains(schemaSource, "@deprecated This OpenAPI value is deprecated.") {
				t.Fatalf("%s schema deprecation missing from generated source:\n%s", version, schemaSource)
			}
			metadataSource := string(artifactByPath(t, artifacts, "metadata.ts"))
			if !strings.Contains(metadataSource, `["deprecated", true]`) {
				t.Fatalf("%s schema deprecation missing from metadata:\n%s", version, metadataSource)
			}
		})
	}
}

func TestDeprecatedEnumComponentsEmitAcrossSupportedVersionLines(t *testing.T) {
	for _, version := range []string{"3.0.4", "3.1.2", "3.2.0"} {
		t.Run(version, func(t *testing.T) {
			document, err := sdkgen.Compile([]byte(fmt.Sprintf(`{
  "openapi": %q,
  "info": {"title": "Deprecated enums", "version": "1"},
  "paths": {
    "/status": {
      "get": {
        "operationId": "getLegacyStatus",
        "responses": {"200": {"description": "OK", "content": {"application/json": {"schema": {"$ref": "#/components/schemas/LegacyStatus"}}}}}
      }
    }
  },
  "components": {"schemas": {"LegacyStatus": {"type": "string", "enum": ["OLD", "OLDER"], "deprecated": true}}}
}`, version)))
			if err != nil {
				t.Fatal(err)
			}
			artifacts, err := SourceArtifacts(document)
			if err != nil {
				t.Fatal(err)
			}
			enumsSource := string(artifactByPath(t, artifacts, "internal/enums.ts"))
			if !jsDocBeforeMarkerContains(enumsSource, `readonly "LegacyStatus": {`, "@deprecated This OpenAPI enum is deprecated.") {
				t.Fatalf("%s enum component lacks deprecation JSDoc:\n%s", version, enumsSource)
			}
			if jsDocBeforeMarkerContains(enumsSource, `readonly "OLD": "OLD"`, "@deprecated This OpenAPI enum value is deprecated.") {
				t.Fatalf("%s enum schema deprecation incorrectly became member deprecation:\n%s", version, enumsSource)
			}
		})
	}
}

func TestOperationAndParameterDeprecationEmitAcrossSupportedVersionLines(t *testing.T) {
	for _, version := range []string{"3.0.4", "3.1.2", "3.2.0"} {
		t.Run(version, func(t *testing.T) {
			extraPaths := ""
			security := ""
			if strings.HasPrefix(version, "3.2") {
				extraPaths = `,
    "/search": {
      "query": {
        "operationId": "queryLegacy", "deprecated": true,
        "parameters": [{"name": "queryLegacyParam", "in": "query", "deprecated": true, "schema": {"type": "string"}}],
        "responses": {"204": {"description": "OK"}}
      },
      "additionalOperations": {
        "PURGE": {
          "operationId": "purgeLegacy", "deprecated": true,
          "parameters": [{"name": "purgeLegacyParam", "in": "query", "deprecated": true, "schema": {"type": "string"}}],
          "responses": {"204": {"description": "OK"}}
        }
      }
    }`
				security = `,
  "security": [{"LegacyKey": []}],
  "components": {"securitySchemes": {"LegacyKey": {"type": "apiKey", "in": "header", "name": "x-api-key", "deprecated": true}}}`
			}
			document, err := sdkgen.Compile([]byte(fmt.Sprintf(`{
  "openapi": %q,
  "info": {"title": "Deprecation", "version": "1"},
  "paths": {
    "/legacy": {
      "get": {
        "operationId": "getLegacy", "deprecated": true,
        "parameters": [{"name": "legacyParam", "in": "query", "deprecated": true, "schema": {"type": "string"}}],
        "responses": {"204": {"description": "OK"}}
      }
    }%s
  }%s
}`, version, extraPaths, security)))
			if err != nil {
				t.Fatal(err)
			}
			artifacts, err := SourceArtifacts(document)
			if err != nil {
				t.Fatal(err)
			}
			source := clientSemanticSource(artifacts)
			for _, operationID := range append([]string{"getLegacy"}, func() []string {
				if strings.HasPrefix(version, "3.2") {
					return []string{"queryLegacy", "purgeLegacy"}
				}
				return nil
			}()...) {
				if !jsDocForMarkerContains(source, "Operation ID: `"+operationID+"`.", "@deprecated This operation is deprecated.") {
					t.Fatalf("%s operation %s lacks deprecation JSDoc:\n%s", version, operationID, source)
				}
			}
			for _, parameter := range append([]string{"legacyParam"}, func() []string {
				if strings.HasPrefix(version, "3.2") {
					return []string{"queryLegacyParam", "purgeLegacyParam"}
				}
				return nil
			}()...) {
				if !jsDocForMarkerContains(source, "Query parameter `"+parameter+"`.", "@deprecated This OpenAPI value is deprecated.") {
					t.Fatalf("%s parameter %s lacks deprecation JSDoc:\n%s", version, parameter, source)
				}
			}
			if strings.HasPrefix(version, "3.2") && !strings.Contains(source, `name: "LegacyKey", type: "apiKey", location: "header", parameterName: "x-api-key", deprecated: true`) {
				t.Fatalf("%s generated security definition lost deprecation metadata:\n%s", version, source)
			}
		})
	}
}

func TestResponseHeaderDeprecationEmitsAcrossSupportedVersionLines(t *testing.T) {
	for _, version := range []string{"3.0.4", "3.1.2", "3.2.0"} {
		t.Run(version, func(t *testing.T) {
			document, err := sdkgen.Compile([]byte(fmt.Sprintf(`{
  "openapi": %q,
  "info": {"title": "Deprecated response headers", "version": "1"},
  "paths": {
    "/headers": {
      "get": {
        "operationId": "getHeaders",
        "responses": {
          "200": {
            "description": "OK",
            "headers": {
              "X-Inline-Legacy": {"deprecated": true, "schema": {"type": "string"}},
              "X-Ref-Legacy": {"$ref": "#/components/headers/Legacy"},
              "X-Current": {"deprecated": false, "schema": {"type": "string"}}
            },
            "content": {"application/json": {"schema": {"type": "string"}}}
          }
        }
      }
    }
  },
  "components": {
    "headers": {
      "Legacy": {"deprecated": true, "schema": {"type": "string"}}
    }
  }
}`, version)))
			if err != nil {
				t.Fatal(err)
			}
			artifacts, err := SourceArtifacts(document)
			if err != nil {
				t.Fatal(err)
			}
			source := clientSemanticSource(artifacts)
			for _, name := range []string{"X-Inline-Legacy", "X-Ref-Legacy"} {
				expected := `/** @deprecated This OpenAPI response header is deprecated. */ readonly "` + name + `"?: string`
				if !strings.Contains(source, expected) {
					t.Fatalf("%s source missing %q:\n%s", version, expected, source)
				}
			}
			if forbidden := `@deprecated This OpenAPI response header is deprecated. */ readonly "X-Current"`; strings.Contains(source, forbidden) {
				t.Fatalf("%s source deprecated a current header:\n%s", version, source)
			}
		})
	}
}
