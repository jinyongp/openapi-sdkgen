package typescript

import (
	"strings"
	"testing"

	compiler "openapi-sdkgen/internal/compiler"
)

// No named schema can accidentally supply the helper import for these prepared
// server definitions. The only dependency signal comes from their inline body.
func TestPreparedServerDefinitionsCarryWirePropertyNeeds(t *testing.T) {
	document, err := compiler.Compile([]byte(`{
  "openapi":"3.1.0", "info":{"title":"Prepared inline owners","version":"1"}, "paths":{},
  "webhooks":{"delivery":{"post":{
    "requestBody":{"required":true,"content":{"application/json":{"schema":{"type":"object","properties":{"value":{"type":"integer"}}}}}},
    "responses":{"204":{"description":"done"}}
  }}},
  "components":{"callbacks":{"completed":{"{$request.query.callback}":{"post":{
    "requestBody":{"required":true,"content":{"application/json":{"schema":{"type":"object","properties":{"value":{"type":"integer"}}}}}},
    "responses":{"204":{"description":"done"}}
  }}}}}
}`))
	if err != nil {
		t.Fatal(err)
	}
	if len(document.Schemas) != 0 || len(document.ComponentSchemas) != 0 {
		t.Fatal("test requires no named wire schema owner")
	}
	webhooks, err := collectWebhooks(document)
	if err != nil {
		t.Fatal(err)
	}
	callbacks, err := collectCallbacks(document)
	if err != nil {
		t.Fatal(err)
	}
	if len(webhooks) != 1 || !webhooks[0].usesWireProperties || len(callbacks) != 1 || !callbacks[0].usesWireProperties {
		t.Fatal("prepared inline definition lost its renderer needs")
	}
	artifacts, err := emitPreparedServerArtifacts(document, webhooks, callbacks)
	if err != nil {
		t.Fatal(err)
	}
	for _, artifact := range artifacts {
		if artifact.Path == "server/runtime.ts" {
			continue
		}
		source := string(artifact.Data)
		if !strings.Contains(source, "import { wireProperties as __sdkgen_Properties }") || !strings.Contains(source, "/* @__PURE__ */ __sdkgen_Properties(") {
			t.Fatalf("prepared definition requires a missing helper: %s", artifact.Path)
		}
	}
}
