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

func TestSchemaVariableNamesInOpenAPILiteralsRemainUnused(t *testing.T) {
	document, err := compiler.Compile([]byte(`{"openapi":"3.1.1","info":{"title":"Literal schema names","version":"1"},"paths":{"/inputSchemas/outputSchemas":{"get":{"operationId":"inputSchemasOutputSchemas","responses":{"204":{"description":"OK"}}}}}}`))
	if err != nil {
		t.Fatal(err)
	}
	artifacts, err := SourceArtifacts(document)
	if err != nil {
		t.Fatal(err)
	}
	compileTypeScriptArtifactSet(t, artifacts, "literal-names.ts", `import { createClient } from "./index.js";
const api = createClient({baseURL:"https://literal.test"});
void api.$routes["GET /inputSchemas/outputSchemas"]();
`)
}
