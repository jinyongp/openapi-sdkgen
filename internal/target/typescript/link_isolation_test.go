package typescript

import (
	"strings"
	"testing"

	sdkgen "openapi-sdkgen/internal/compiler"
	"openapi-sdkgen/internal/generator"
)

func TestGeneratedResponseLinksKeepValidSiblingsAroundQuarantinedLink(t *testing.T) {
	document, err := sdkgen.Compile([]byte(`{
  "openapi":"3.1.0",
  "info":{"title":"Sibling Links","version":"1"},
  "paths":{
    "/source":{"get":{"operationId":"getSource","responses":{"200":{
      "description":"OK",
      "links":{
        "before":{"operationId":"getBefore"},
        "invalid":{},
        "after":{"operationId":"getAfter"}
      }
    }}}},
    "/before":{"get":{"operationId":"getBefore","responses":{"204":{"description":"OK"}}}},
    "/after":{"get":{"operationId":"getAfter","responses":{"204":{"description":"OK"}}}}
  }
}`))
	if err != nil {
		t.Fatal(err)
	}
	probe := `import { createClient } from "./index.js"
declare const api: ReturnType<typeof createClient>
api.$operations.getSource()
api.$links.getSource.before
api.$links.getSource.after
// @ts-expect-error quarantined Link capability is not emitted
api.$links.getSource.invalid
`
	compileTypeScriptArtifactsWithProbe(t, document, "link-siblings.probe.ts", probe)
}

func TestGeneratedResponseLinksOmitUnsupportedOperationRefForm(t *testing.T) {
	document, err := sdkgen.Compile([]byte(`{
  "openapi":"3.1.0",
  "info":{"title":"External Link target","version":"1"},
  "paths":{
    "/source":{"get":{"operationId":"getSource","responses":{"200":{
      "description":"OK",
      "links":{"remote":{"operationRef":"https://example.test/openapi.json#/paths/~1target/get"}}
    }}}}
  }
}`))
	if err != nil {
		t.Fatal(err)
	}
	plan, diagnostics, err := (Generator{}).Prepare(document, generator.Options{})
	if err != nil {
		t.Fatal(err)
	}
	if len(diagnostics) != 1 {
		t.Fatalf("diagnostics = %#v", diagnostics)
	}
	value := diagnostics[0]
	if value.Severity != "warning" || value.Code != "SDKGEN-W509" ||
		value.Scope != "capability" || value.Effect != "omit-capability" ||
		value.Capability != "response-link" ||
		!strings.Contains(value.Message, "compiled document closure") {
		t.Fatalf("unsupported operationRef diagnostic = %#v", value)
	}
	if _, err := (Generator{}).Emit(plan); err != nil {
		t.Fatalf("emit with omitted external operationRef Link: %v", err)
	}
	probe := `import { createClient } from "./index.js"
declare const api: ReturnType<typeof createClient>
api.$operations.getSource()
// @ts-expect-error unsupported external operationRef Link helper is omitted
api.$links.getSource.remote
`
	compileTypeScriptArtifactsWithProbe(t, document, "external-link.probe.ts", probe)
}
