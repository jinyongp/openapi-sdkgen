package typescript

import (
	"testing"

	sdkgen "openapi-sdkgen/internal/compiler"
	"openapi-sdkgen/internal/diagnostic"
	"openapi-sdkgen/internal/failure"
	"openapi-sdkgen/internal/generator"
)

func TestSchemaOnlySequentialResponseKeepsBaseOperationWithoutStreamCapability(t *testing.T) {
	document, err := sdkgen.Compile([]byte(`{
  "openapi":"3.2.0",
  "info":{"title":"Schema-only sequential response","version":"1"},
  "paths":{
    "/events":{"get":{"operationId":"listEvents","responses":{"200":{
      "description":"OK",
      "content":{"application/x-ndjson":{"schema":{"type":"array","items":{"type":"string"}}}}
    }}}}
  }
}`))
	if err != nil {
		t.Fatal(err)
	}
	probe := `import { createClient } from "./index.js"
declare const api: ReturnType<typeof createClient>
const values: Promise<readonly string[]> = api.$operations.listEvents()
void values
// @ts-expect-error schema-only sequential response does not declare a stream capability
api.$operations.listEvents.stream
`
	compileTypeScriptArtifactsWithProbe(t, document, "schema-only-stream.probe.ts", probe)
}

func TestExplicitStreamPreparationFailureRemainsDocumentBlocking(t *testing.T) {
	document, err := sdkgen.Compile([]byte(`{
  "openapi":"3.2.0",
  "info":{"title":"Explicit stream failure","version":"1"},
  "paths":{
    "/events":{"get":{"operationId":"watchEvents","responses":{"200":{
      "description":"OK",
      "content":{"application/x-ndjson":{
        "schema":{"type":"array","items":{"type":"string"}},
        "itemSchema":{"type":"string"}
      }}
    }}}}
  }
}`))
	if err != nil {
		t.Fatal(err)
	}
	document.Operations[0].Responses[0].Content[0].ItemSchema = map[string]any{
		"$ref": "https://example.test/schemas.json#/Thing",
	}
	manifest, err := buildManifest(document)
	if err != nil {
		t.Fatalf("base operation manifest should remain representable: %v", err)
	}
	if _, err := generatedStreams(document, manifest); err == nil {
		t.Fatal("explicit stream helper unexpectedly remained representable")
	}
	if _, _, err := newWireRenderContext(wirePropertiesLiteral).operationResponseWireBodies(document, document.Operations[0]); err == nil {
		t.Fatal("stream itemSchema failure did not propagate into the shared runtime wire descriptor")
	}
	prepared, values, err := (Generator{}).Prepare(document, generator.Options{})
	if err != nil {
		t.Fatal(err)
	}
	if len(values) != 1 || values[0].Code != "SDKGEN-E510" ||
		values[0].Severity != diagnostic.SeverityError ||
		values[0].Scope != failure.ScopeDocument ||
		values[0].Effect != failure.EffectBlock ||
		values[0].Operation != "watchEvents" {
		t.Fatalf("stream failure diagnostic = %#v", values)
	}
	value, err := prepared.Value("typescript")
	if err != nil {
		t.Fatal(err)
	}
	plan := value.(*sourcePlan)
	if plan.manifest == nil || len(plan.manifest.Operations) != 1 {
		t.Fatalf("base operation manifest was lost before stream blocking: %#v", plan.manifest)
	}
	if plan.modules != nil {
		t.Fatal("blocking stream failure unexpectedly produced an emission module plan")
	}
}
