package typescript

import (
	"strings"
	"testing"

	sdkgen "openapi-sdkgen/internal/compiler"
)

func TestSchemaModulesTypecheckRecursiveJSONValue(t *testing.T) {
	document, err := sdkgen.Compile([]byte(`{
  "openapi":"3.0.3",
  "info":{"title":"Recursive JSON","version":"1"},
  "paths":{
    "/items":{"get":{
      "operationId":"getItems",
      "responses":{"200":{"description":"OK","content":{"application/json":{"schema":{"$ref":"#/components/schemas/JSONValue"}}}}}
    }}
  },
  "components":{"schemas":{
    "JSONValue":{"oneOf":[
      {"type":"string","nullable":true},
      {"type":"number"},
      {"type":"boolean"},
      {"type":"object","additionalProperties":{"$ref":"#/components/schemas/JSONValue"}},
      {"type":"array","items":{"$ref":"#/components/schemas/JSONValue"}}
    ]}
  }}
}`))
	if err != nil {
		t.Fatal(err)
	}
	artifacts, err := SourceArtifacts(document)
	if err != nil {
		t.Fatal(err)
	}
	source := schemaProjectionSource(artifacts)
	if !strings.Contains(source, "{ readonly [key: string]: Input }") ||
		!strings.Contains(source, "{ readonly [key: string]: Output }") {
		t.Fatalf("recursive JSON projection did not use readonly index signatures:\n%s", source)
	}
	if strings.Contains(source, "Readonly<Record<string, Input>>") ||
		strings.Contains(source, "Readonly<Record<string, Output>>") {
		t.Fatalf("recursive JSON projection retained Record alias recursion:\n%s", source)
	}
	compileTypeScriptArtifacts(t, document)
}

func TestOperationModuleTypechecksRecursiveBodyInput(t *testing.T) {
	document, err := sdkgen.Compile([]byte(`{
  "openapi":"3.0.3",
  "info":{"title":"Recursive operation input","version":"1"},
  "paths":{
    "/bulk":{"put":{
      "operationId":"putBulk",
      "requestBody":{"required":true,"content":{"application/json":{"schema":{"$ref":"#/components/schemas/BulkWrite"}}}},
      "responses":{"204":{"description":"OK"}}
    }}
  },
  "components":{"schemas":{
    "Any":{"anyOf":[
      {"type":"string"},
      {"type":"number"},
      {"type":"integer"},
      {"type":"boolean"},
      {"type":"object","additionalProperties":true,"nullable":true},
      {"type":"array","items":{"$ref":"#/components/schemas/Any"}}
    ]},
    "Metadata":{"allOf":[
      {"$ref":"#/components/schemas/Any"},
      {"description":"Arbitrary JSON metadata."}
    ]},
    "BulkWrite":{
      "type":"array",
      "items":{
        "type":"object",
        "required":["key","value"],
        "properties":{
          "key":{"type":"string"},
          "value":{"type":"string"},
          "metadata":{"$ref":"#/components/schemas/Metadata"}
        }
      }
    }
  }}
}`))
	if err != nil {
		t.Fatal(err)
	}
	artifacts, err := SourceArtifacts(document)
	if err != nil {
		t.Fatal(err)
	}
	operationSource := operationArtifactSource(t, artifacts, "PUT /bulk")
	for _, expected := range []string{
		"interface __sdkgen_Input {",
		"export type Input = __sdkgen_Input",
		"export type Output = __sdkgen_Output",
		"export type BaseCall = __sdkgen_Call",
		"export type ExactCall =",
		"bindGeneratedOperation(request,",
	} {
		if !strings.Contains(operationSource, expected) {
			t.Fatalf("recursive operation source missing %q:\n%s", expected, operationSource)
		}
	}
	if strings.Contains(operationSource, "bindOperation<Input, Output") {
		t.Fatalf("recursive operation re-instantiated public types through runtime binder:\n%s", operationSource)
	}
	compileTypeScriptArtifacts(t, document)
}

func TestTypeScriptAcceptsStandaloneRecursiveIndexSignatureAlias(t *testing.T) {
	document, err := sdkgen.Compile([]byte(`{
  "openapi":"3.1.1",
  "info":{"title":"Probe","version":"1"},
  "paths":{"/ok":{"get":{"operationId":"getOK","responses":{"204":{"description":"OK"}}}}}
}`))
	if err != nil {
		t.Fatal(err)
	}
	probe := `type JSONValue =
  | string
  | number
  | boolean
  | null
  | { readonly [key: string]: JSONValue }
  | readonly JSONValue[]
declare const value: JSONValue
void value
`
	compileTypeScriptArtifactsWithProbe(t, document, "recursive-record-alias.probe.ts", probe)
}

func TestSchemaModulesTypecheckMutuallyRecursiveMaps(t *testing.T) {
	document, err := sdkgen.Compile([]byte(`{
  "openapi":"3.1.1",
  "info":{"title":"Mutually recursive maps","version":"1"},
  "paths":{
    "/items":{"get":{
      "operationId":"getItems",
      "responses":{"200":{"description":"OK","content":{"application/json":{"schema":{"$ref":"#/components/schemas/A"}}}}}
    }}
  },
  "components":{"schemas":{
    "A":{"type":"object","additionalProperties":{"$ref":"#/components/schemas/B"}},
    "B":{"type":"object","additionalProperties":{"$ref":"#/components/schemas/A"}}
  }}
}`))
	if err != nil {
		t.Fatal(err)
	}
	compileTypeScriptArtifacts(t, document)
}

func TestOperationModuleTypechecksRecursiveInputWithStreamCapability(t *testing.T) {
	document, err := sdkgen.Compile([]byte(`{
  "openapi":"3.2.0",
  "info":{"title":"Recursive stream input","version":"1"},
  "paths":{
    "/events":{"post":{
      "operationId":"publishAndStreamEvents",
      "requestBody":{"required":true,"content":{"application/json":{"schema":{"$ref":"#/components/schemas/JSONValue"}}}},
      "responses":{"200":{"description":"OK","content":{"application/x-ndjson":{"itemSchema":{"type":"string"}}}}}
    }}
  },
  "components":{"schemas":{
    "JSONValue":{"oneOf":[
      {"type":"string"},
      {"type":"number"},
      {"type":"boolean"},
      {"type":"object","additionalProperties":{"$ref":"#/components/schemas/JSONValue"}},
      {"type":"array","items":{"$ref":"#/components/schemas/JSONValue"}}
    ]}
  }}
}`))
	if err != nil {
		t.Fatal(err)
	}
	artifacts, err := SourceArtifacts(document)
	if err != nil {
		t.Fatal(err)
	}
	operationSource := operationArtifactSource(t, artifacts, "POST /events")
	if !strings.Contains(operationSource, "bindStreamOperation<Input, string, Options>") {
		t.Fatalf("recursive stream operation did not exercise generic stream binder:\n%s", operationSource)
	}
	compileTypeScriptArtifacts(t, document)
}
