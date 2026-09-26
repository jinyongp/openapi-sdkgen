package typescript

import (
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	sdkgen "openapi-sdkgen/internal/compiler"
	"openapi-sdkgen/internal/diagnostic"
	"openapi-sdkgen/internal/failure"
	"openapi-sdkgen/internal/generator"
)

func TestPrepareOmitsGETAndHEADBodiesAsFetchTargetCapabilities(t *testing.T) {
	for _, version := range []string{"3.1.1", "3.2.0"} {
		for _, method := range []string{"get", "head"} {
			t.Run(version+"-"+method, func(t *testing.T) {
				document, err := sdkgen.Compile([]byte(`{
  "openapi":"` + version + `",
  "info":{"title":"Fetch body","version":"1"},
  "paths":{"/items":{"` + method + `":{
    "operationId":"fetchBody",
    "requestBody":{"required":true,"content":{"application/json":{"schema":{"type":"string"}}}},
    "responses":{"204":{"description":"OK"}}
  }}}
}`))
				if err != nil {
					t.Fatal(err)
				}
				_, values, err := (Generator{}).Prepare(document, generator.Options{})
				if err != nil {
					t.Fatal(err)
				}
				if len(values) != 2 || !diagnosticsContainCode(values, "SDKGEN-E512") {
					t.Fatalf("diagnostics = %#v", values)
				}
				var omission diagnostic.Diagnostic
				for _, value := range values {
					if value.Code == "SDKGEN-W511" {
						omission = value
					}
				}
				if omission.Code == "" || omission.Severity != diagnostic.SeverityWarning ||
					omission.Scope != failure.ScopeOperation || omission.Effect != failure.EffectOmitOperation ||
					omission.Phase != diagnostic.PhaseTarget || omission.Target != "typescript" ||
					!strings.HasSuffix(omission.Location.Pointer, "/requestBody") {
					t.Fatalf("diagnostics = %#v", values)
				}
			})
		}
	}
}

func TestPrepareSeparatesFetchForbiddenMethodsFromAllowedQueryAndCustomMethods(t *testing.T) {
	document, err := sdkgen.Compile([]byte(`{
  "openapi":"3.2.0",
  "info":{"title":"Fetch methods","version":"1"},
  "paths":{"/methods":{"additionalOperations":{
    "QUERY":{"operationId":"queryItems","responses":{"204":{"description":"OK"}}},
    "CONNECT":{"operationId":"connectItems","responses":{"204":{"description":"OK"}}},
    "TRACE":{"operationId":"traceItems","responses":{"204":{"description":"OK"}}},
    "TRACK":{"operationId":"trackItems","responses":{"204":{"description":"OK"}}},
    "PURGE":{"operationId":"purgeItems","responses":{"204":{"description":"OK"}}}
  }}}
}`))
	if err != nil {
		t.Fatal(err)
	}
	_, values, err := (Generator{}).Prepare(document, generator.Options{})
	if err != nil {
		t.Fatal(err)
	}
	var forbidden []string
	for _, value := range values {
		if value.Code == "SDKGEN-W511" {
			forbidden = append(forbidden, value.Operation)
			if value.Severity != diagnostic.SeverityWarning || value.Scope != failure.ScopeOperation ||
				value.Effect != failure.EffectOmitOperation || value.Target != "typescript" || value.Phase != diagnostic.PhaseTarget {
				t.Fatalf("diagnostic = %#v", value)
			}
		}
	}
	if strings.Join(forbidden, ",") != "connectItems,traceItems,trackItems" {
		t.Fatalf("forbidden operations = %q; diagnostics = %#v", forbidden, values)
	}
}

func TestPrepareOmitsOrdinaryTRACEWithoutBodyBeforeEmit(t *testing.T) {
	document, err := sdkgen.Compile([]byte(`{
  "openapi":"3.1.1",
  "info":{"title":"Trace","version":"1"},
  "paths":{"/trace":{"trace":{
    "operationId":"traceItems",
    "responses":{"204":{"description":"OK"}}
  }}}
}`))
	if err != nil {
		t.Fatal(err)
	}
	_, values, err := (Generator{}).Prepare(document, generator.Options{})
	if err != nil {
		t.Fatal(err)
	}
	if len(values) != 2 || !diagnosticsContainCode(values, "SDKGEN-W511") || !diagnosticsContainCode(values, "SDKGEN-E512") {
		t.Fatalf("diagnostics = %#v", values)
	}
	for _, value := range values {
		if value.Code == "SDKGEN-W511" && (value.Severity != diagnostic.SeverityWarning ||
			value.Scope != failure.ScopeOperation || value.Effect != failure.EffectOmitOperation || value.Operation != "traceItems") {
			t.Fatalf("diagnostics = %#v", values)
		}
	}
}

func TestOAS30MeaningfulDELETEBodyIsSentExactly(t *testing.T) {
	document, err := sdkgen.Compile([]byte(`{
  "openapi":"3.0.3",
  "info":{"title":"Delete runtime","version":"1"},
  "paths":{"/items":{"delete":{
    "operationId":"deleteItem",
    "requestBody":{"required":true,"content":{"application/json":{"schema":{"type":"object","required":["id"],"properties":{"id":{"type":"string"}}}}}},
    "responses":{"204":{"description":"Deleted"}}
  }}}
}`))
	if err != nil {
		t.Fatal(err)
	}
	output := compileTypeScriptArtifacts(t, document)
	script := `
import { pathToFileURL } from "node:url";
const { createClient } = await import(pathToFileURL(process.argv[1]).href);
const seen = [];
const api = createClient({
  baseURL: "https://api.example.test",
  fetch: async (input, init) => {
    seen.push({
      method: init.method,
      body: String(init.body),
      contentType: new Headers(init.headers).get("content-type"),
      path: new URL(String(input)).pathname,
    });
    return new Response(null, { status: 204 });
  },
});
await api.$operations.deleteItem({ body: { id: "item-1" } });
if (seen.length !== 1) throw new Error("unexpected request count: " + seen.length);
const request = seen[0];
if (request.method !== "DELETE") throw new Error("method mismatch: " + request.method);
if (request.path !== "/items") throw new Error("path mismatch: " + request.path);
if (request.body !== '{"id":"item-1"}') throw new Error("body mismatch: " + request.body);
if (request.contentType !== "application/json") throw new Error("content type mismatch: " + request.contentType);
`
	if result, err := exec.Command("node", "--input-type=module", "--eval", script, filepath.Join(output, "index.js")).CombinedOutput(); err != nil {
		t.Fatalf("execute DELETE compatibility runtime test: %v\n%s", err, result)
	}
}
