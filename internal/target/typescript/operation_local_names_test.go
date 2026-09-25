package typescript

import (
	"strings"
	"testing"

	compiler "openapi-sdkgen/internal/compiler"
)

// Assertions about lexical bindings must select their owning module, never the
// first matching name in concatenated sources from several independent modules.
func operationArtifactSource(t testing.TB, artifacts []Artifact, route string) string {
	t.Helper()
	marker := "\nexport type RouteKey = " + quoteTS(route) + "\n"
	var source string
	for _, artifact := range artifacts {
		if !strings.HasPrefix(artifact.Path, "internal/operations/") || !strings.Contains(string(artifact.Data), marker) {
			continue
		}
		if source != "" {
			t.Fatalf("more than one operation artifact owns %q", route)
		}
		source = string(artifact.Data)
	}
	if source == "" {
		t.Fatalf("no operation artifact owns %q", route)
	}
	return source
}

func TestOperationPrivateTypesAreLocalAndPreservePublicExports(t *testing.T) {
	document, err := compiler.Compile([]byte(`{
  "openapi":"3.1.0", "info":{"title":"Local types","version":"1"},
  "paths":{
    "/alpha":{"post":{"operationId":"alpha","requestBody":{"required":true,"content":{"application/json":{"schema":{"type":"string"}}}},"responses":{"200":{"description":"ok","content":{"application/json":{"schema":{"type":"string"}}}}}}},
    "/beta":{"post":{"operationId":"beta","requestBody":{"required":true,"content":{"application/json":{"schema":{"type":"number"}}}},"responses":{"200":{"description":"ok","content":{"application/json":{"schema":{"type":"number"}}}}}}}
  }
}`))
	if err != nil {
		t.Fatal(err)
	}
	artifacts, err := SourceArtifacts(document)
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct{ route, value string }{{"POST /alpha", "string"}, {"POST /beta", "number"}} {
		source := operationArtifactSource(t, artifacts, test.route)
		for _, expected := range []string{
			"interface __sdkgen_Input {",
			"type __sdkgen_BodyInput = " + test.value,
			"type __sdkgen_Output = " + test.value,
			"interface __sdkgen_Call {",
			"interface __sdkgen_RawCall {",
			"interface __sdkgen_ResourceCall ",
			"interface __sdkgen_ResourceRawCall {",
			"export type Input = __sdkgen_Input",
			"export type Output = __sdkgen_Output",
			"export type Options = __sdkgen_Options",
			"export type BaseCall = __sdkgen_Call",
			"export type RawCall = __sdkgen_RawCall",
		} {
			if !strings.Contains(source, expected) {
				t.Fatalf("%s missing %q:\n%s", test.route, expected, source)
			}
		}
		if strings.Contains(source, "_r6f7065726174696f6e2d74797065_") || strings.Contains(source, "__sdkgen_opInput") {
			t.Fatalf("%s retained an operation-global or redundant type stem", test.route)
		}
	}
}
