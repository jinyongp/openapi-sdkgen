package typescript

import (
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	sdkgen "openapi-sdkgen/internal/compiler"
)

func TestOpenAPI30SchemaNormalizationsMatchEquivalentTargetSemantics(t *testing.T) {
	tests := []struct {
		name       string
		candidate  string
		equivalent string
	}{
		{name: "boolean true", candidate: "true", equivalent: `{}`},
		{name: "boolean false", candidate: "false", equivalent: `{"not":{}}`},
		{name: "const", candidate: `{"type":"string","const":"ready"}`, equivalent: `{"type":"string","enum":["ready"]}`},
		{name: "nullable type array", candidate: `{"type":["string","null"],"minLength":2}`, equivalent: `{"type":"string","nullable":true,"minLength":2}`},
		{name: "numeric exclusive minimum", candidate: `{"type":"number","exclusiveMinimum":2.5}`, equivalent: `{"type":"number","minimum":2.5,"exclusiveMinimum":true}`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			compile := func(schema string) *sdkgen.Result {
				result, err := sdkgen.CompileResult([]byte(`{"openapi":"3.0.3","info":{"title":"Equivalence","version":"1"},"paths":{},"components":{"schemas":{"Value":` + schema + `}}}`))
				if err != nil {
					t.Fatal(err)
				}
				if result.Document == nil {
					t.Fatalf("compile result = %#v", result)
				}
				return &result
			}
			candidate := compile(test.candidate)
			equivalent := compile(test.equivalent)
			candidateSchema := candidate.Document.ComponentSchemas["Value"]
			equivalentSchema := equivalent.Document.ComponentSchemas["Value"]

			candidateType, err := schemaType(candidate.Document, candidateSchema, projectionInput)
			if err != nil {
				t.Fatal(err)
			}
			equivalentType, err := schemaType(equivalent.Document, equivalentSchema, projectionInput)
			if err != nil {
				t.Fatal(err)
			}
			if candidateType != equivalentType {
				t.Fatalf("public type differs: candidate %q equivalent %q", candidateType, equivalentType)
			}

			candidateWire, err := newWireRenderContext(wirePropertiesLiteral).wireSchemaDescriptorForDocument(candidate.Document, candidateSchema, projectionInput)
			if err != nil {
				t.Fatal(err)
			}
			equivalentWire, err := newWireRenderContext(wirePropertiesLiteral).wireSchemaDescriptorForDocument(equivalent.Document, equivalentSchema, projectionInput)
			if err != nil {
				t.Fatal(err)
			}
			if candidateWire != equivalentWire {
				t.Fatalf("wire descriptor differs:\ncandidate: %s\nequivalent: %s", candidateWire, equivalentWire)
			}
		})
	}
}

func TestOpenAPI30BooleanAdditionalPropertiesStayNativeAcrossTarget(t *testing.T) {
	result, err := sdkgen.CompileResult([]byte(`{
  "openapi":"3.0.3",
  "info":{"title":"Boolean additional properties","version":"1"},
  "paths":{
    "/open":{"post":{"operationId":"acceptOpen","requestBody":{"required":true,"content":{"application/json":{"schema":{"$ref":"#/components/schemas/Open"}}}},"responses":{"204":{"description":"OK"}}}},
    "/closed":{"post":{"operationId":"rejectClosed","requestBody":{"required":true,"content":{"application/json":{"schema":{"$ref":"#/components/schemas/Closed"}}}},"responses":{"204":{"description":"OK"}}}}
  },
  "components":{"schemas":{
    "Open":{"type":"object","additionalProperties":true},
    "Closed":{"type":"object","additionalProperties":false}
  }}
}`))
	if err != nil {
		t.Fatal(err)
	}
	if result.Document == nil || len(result.Diagnostics) != 0 {
		t.Fatalf("compile result = %#v", result)
	}

	openSchema := result.Document.ComponentSchemas["Open"]
	closedSchema := result.Document.ComponentSchemas["Closed"]
	openType, err := schemaType(result.Document, openSchema, projectionInput)
	if err != nil {
		t.Fatal(err)
	}
	closedType, err := schemaType(result.Document, closedSchema, projectionInput)
	if err != nil {
		t.Fatal(err)
	}
	if openType != "Readonly<Record<string, unknown>>" ||
		closedType != "Readonly<Record<string, never>>" {
		t.Fatalf("types = open %q closed %q", openType, closedType)
	}

	openWire, err := newWireRenderContext(wirePropertiesLiteral).wireSchemaDescriptorForDocument(result.Document, openSchema, projectionInput)
	if err != nil {
		t.Fatal(err)
	}
	closedWire, err := newWireRenderContext(wirePropertiesLiteral).wireSchemaDescriptorForDocument(result.Document, closedSchema, projectionInput)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(openWire, "additionalProperties: { boolean: true }") {
		t.Fatalf("open wire descriptor = %s", openWire)
	}
	if !strings.Contains(closedWire, "additionalProperties: false") {
		t.Fatalf("closed wire descriptor = %s", closedWire)
	}

	output := compileTypeScriptArtifacts(t, result.Document)
	script := `
import { pathToFileURL } from "node:url";
const { createClient } = await import(pathToFileURL(process.argv[1]).href);
let calls = 0;
const api = createClient({
  baseURL: "https://api.example.test",
  fetch: async () => { calls++; return new Response(null, { status: 204 }); },
});
await api.$operations.acceptOpen({ body: { extra: { nested: true } } });
if (calls !== 1) throw new Error("open additionalProperties did not reach fetch");
let rejected = false;
try { await api.$operations.rejectClosed({ body: { extra: true } }); }
catch (error) {
  rejected = String(error).includes("unexpected property extra") || String(error.cause).includes("unexpected property extra");
}
if (!rejected || calls !== 1) throw new Error("closed additionalProperties accepted an unknown property");
`
	if runtimeOutput, err := exec.Command("node", "--input-type=module", "--eval", script, filepath.Join(output, "index.js")).CombinedOutput(); err != nil {
		t.Fatalf("execute boolean additionalProperties runtime test: %v\n%s", err, runtimeOutput)
	}
}

func TestOpenAPI30BooleanFalseNormalizationRejectsRequestAndResponseRuntimeValues(t *testing.T) {
	document, err := sdkgen.Compile([]byte(`{
  "openapi":"3.0.3",
  "info":{"title":"Boolean false compatibility","version":"1"},
  "paths":{
    "/input":{"post":{"operationId":"denyInput","requestBody":{"required":true,"content":{"application/json":{"schema":false}}},"responses":{"204":{"description":"OK"}}}},
    "/output":{"get":{"operationId":"denyOutput","responses":{"200":{"description":"OK","content":{"application/json":{"schema":false}}}}}}
  }
}`))
	if err != nil {
		t.Fatal(err)
	}
	output := compileTypeScriptArtifacts(t, document)
	script := `
import { pathToFileURL } from "node:url";
const { createClient } = await import(pathToFileURL(process.argv[1]).href);
let fetched = 0;
const api = createClient({
  baseURL: "https://api.example.test",
  fetch: async (input) => {
    fetched++;
    if (new URL(String(input)).pathname === "/output")
      return new Response(JSON.stringify("invalid"), { status: 200, headers: { "content-type": "application/json" } });
    return new Response(null, { status: 204 });
  },
});
let inputRejected = false;
try { await api.$operations.denyInput({ body: "invalid" }); } catch { inputRejected = true; }
if (!inputRejected || fetched !== 0) throw new Error("normalized false request schema reached fetch");
let outputRejected = false;
try { await api.$operations.denyOutput(); } catch { outputRejected = true; }
if (!outputRejected || fetched !== 1) throw new Error("normalized false response schema accepted a value");
`
	if result, err := exec.Command("node", "--input-type=module", "--eval", script, filepath.Join(output, "index.js")).CombinedOutput(); err != nil {
		t.Fatalf("execute boolean false compatibility runtime test: %v\n%s", err, result)
	}
}
