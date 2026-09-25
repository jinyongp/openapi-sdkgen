package typescript

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	sdkgen "openapi-sdkgen/internal/compiler"
	"openapi-sdkgen/internal/compiler/ir"
	"openapi-sdkgen/internal/generator"
)

func TestAllowEmptyValueControlsFormQueryRuntimeAndIgnoresNAStyles(t *testing.T) {
	document, err := sdkgen.Compile([]byte(`{
  "openapi":"3.1.1",
  "info":{"title":"Empty query values","version":"1"},
  "paths":{
    "/allowed":{"get":{"operationId":"allowed","parameters":[{"name":"search","in":"query","allowEmptyValue":true,"schema":{"type":"string"}}],"responses":{"204":{"description":"OK"}}}},
    "/false":{"get":{"operationId":"explicitFalse","parameters":[{"name":"search","in":"query","allowEmptyValue":false,"schema":{"type":"string"}}],"responses":{"204":{"description":"OK"}}}},
    "/absent":{"get":{"operationId":"absent","parameters":[{"name":"search","in":"query","schema":{"type":"string"}}],"responses":{"204":{"description":"OK"}}}},
    "/ignored-true":{"get":{"operationId":"ignoredTrue","parameters":[{"name":"filter","in":"query","style":"deepObject","allowEmptyValue":true,"schema":{"type":"object","properties":{"name":{"type":"string"}}}}],"responses":{"204":{"description":"OK"}}}},
    "/ignored-absent":{"get":{"operationId":"ignoredAbsent","parameters":[{"name":"filter","in":"query","style":"deepObject","schema":{"type":"object","properties":{"name":{"type":"string"}}}}],"responses":{"204":{"description":"OK"}}}}
  }
}`))
	if err != nil {
		t.Fatal(err)
	}
	output := compileTypeScriptArtifacts(t, document)
	script := `
import { pathToFileURL } from "node:url";
const { createClient } = await import(pathToFileURL(process.argv[1]).href);
const requests = [];
const api = createClient({
  baseURL: "https://api.example.test",
  fetch: async (input) => {
    requests.push(new URL(String(input)));
    return new Response(null, { status: 204 });
  },
});
await api.$operations.allowed({ query: { search: "" } });
if (requests.length !== 1 || requests[0].pathname !== "/allowed" || requests[0].search !== "?search=")
  throw new Error("allowEmptyValue true did not preserve explicit empty query value");
for (const [name, operation] of [["false", api.$operations.explicitFalse], ["absent", api.$operations.absent]]) {
  const before = requests.length;
  let rejected = false;
  try { await operation({ query: { search: "" } }); } catch { rejected = true; }
  if (!rejected || requests.length !== before)
    throw new Error(name + " empty query value was not rejected before fetch");
}
await api.$operations.ignoredTrue({ query: { filter: { name: "" } } });
await api.$operations.ignoredAbsent({ query: { filter: { name: "" } } });
const last = requests.slice(-2).map((request) => request.search);
if (JSON.stringify(last) !== JSON.stringify(["?filter%5Bname%5D=", "?filter%5Bname%5D="]))
  throw new Error("allowEmptyValue changed a deepObject n/a serialization: " + JSON.stringify(last));
`
	if result, err := exec.Command("node", "--input-type=module", "--eval", script, filepath.Join(output, "index.js")).CombinedOutput(); err != nil {
		t.Fatalf("execute allowEmptyValue client runtime test: %v\n%s", err, result)
	}
}

func TestServerAllowEmptyValueMatchesClientPolicy(t *testing.T) {
	document, err := sdkgen.Compile([]byte(`{
  "openapi":"3.1.1",
  "info":{"title":"Inbound empty query values","version":"1"},
  "paths":{},
  "webhooks":{
    "allowed":{"get":{"operationId":"allowed","parameters":[{"name":"search","in":"query","allowEmptyValue":true,"schema":{"type":"string"}}],"responses":{"204":{"description":"OK"}}}},
    "false":{"get":{"operationId":"explicitFalse","parameters":[{"name":"search","in":"query","allowEmptyValue":false,"schema":{"type":"string"}}],"responses":{"204":{"description":"OK"}}}},
    "absent":{"get":{"operationId":"absent","parameters":[{"name":"search","in":"query","schema":{"type":"string"}}],"responses":{"204":{"description":"OK"}}}},
    "ignoredTrue":{"get":{"operationId":"ignoredTrue","parameters":[{"name":"filter","in":"query","style":"deepObject","allowEmptyValue":true,"schema":{"type":"object","properties":{"name":{"type":"string"}}}}],"responses":{"204":{"description":"OK"}}}},
    "ignoredAbsent":{"get":{"operationId":"ignoredAbsent","parameters":[{"name":"filter","in":"query","style":"deepObject","schema":{"type":"object","properties":{"name":{"type":"string"}}}}],"responses":{"204":{"description":"OK"}}}}
  }
}`))
	if err != nil {
		t.Fatal(err)
	}
	output := compileServerArtifactsForCompatibility(t, document)
	script := `
import { pathToFileURL } from "node:url";
const { createWebhookRouter } = await import(pathToFileURL(process.argv[1]).href);
const seen = [];
const handler = (name) => async ({ params }) => { seen.push([name, params.query]); return { status: 204 }; };
const router = createWebhookRouter({
  allowed: { GET: handler("allowed") },
  false: { GET: handler("false") },
  absent: { GET: handler("absent") },
  ignoredTrue: { GET: handler("ignoredTrue") },
  ignoredAbsent: { GET: handler("ignoredAbsent") },
}, { routes: {
  allowed: "/allowed",
  false: "/false",
  absent: "/absent",
  ignoredTrue: "/ignored-true",
  ignoredAbsent: "/ignored-absent",
} });
if ((await router.fetch(new Request("https://host.test/allowed?search=", { method: "GET" }))).status !== 204)
  throw new Error("allowEmptyValue true inbound request was rejected");
for (const path of ["/false?search=", "/absent?search="]) {
  const response = await router.fetch(new Request("https://host.test" + path, { method: "GET" }));
  if (response.status !== 400) throw new Error(path + " empty query value was accepted");
}
for (const path of ["/ignored-true?filter%5Bname%5D=", "/ignored-absent?filter%5Bname%5D="]) {
  const response = await router.fetch(new Request("https://host.test" + path, { method: "GET" }));
  if (response.status !== 204) throw new Error(path + " n/a empty-value field changed decoding");
}
if (seen.length !== 3) throw new Error("unexpected handler calls: " + JSON.stringify(seen));
if (seen[0][1].search !== "") throw new Error("allowed empty query did not decode as empty string");
if (JSON.stringify(seen[1][1]) !== JSON.stringify({ filter: { name: "" } }) ||
    JSON.stringify(seen[2][1]) !== JSON.stringify({ filter: { name: "" } }))
  throw new Error("n/a deepObject behavior diverged: " + JSON.stringify(seen));
`
	if result, err := exec.Command("node", "--input-type=module", "--eval", script, filepath.Join(output, "server", "webhooks.js")).CombinedOutput(); err != nil {
		t.Fatalf("execute allowEmptyValue server runtime test: %v\n%s", err, result)
	}
}

func compileServerArtifactsForCompatibility(t *testing.T, document *ir.Document) string {
	t.Helper()
	registry, err := generator.NewAddonRegistry(generator.AddonServer)
	if err != nil {
		t.Fatal(err)
	}
	options, err := registry.Resolve([]string{"server"})
	if err != nil {
		t.Fatal(err)
	}
	artifacts, err := (Generator{}).Generate(document, options)
	if err != nil {
		t.Fatal(err)
	}
	directory := t.TempDir()
	source := filepath.Join(directory, "source")
	writeTargetArtifacts(t, source, artifacts)
	if err := os.WriteFile(filepath.Join(source, "package.json"), []byte(`{"type":"module"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(source, "tsconfig.json"), []byte(serverTSConfig), 0o600); err != nil {
		t.Fatal(err)
	}
	tsc := filepath.Join("..", "..", "..", "test", "typescript", "node_modules", "typescript", "lib", "tsc.js")
	if _, err := os.Stat(tsc); err != nil {
		t.Skipf("TypeScript compiler unavailable for server compatibility test: %v", err)
	}
	if result, err := exec.Command("node", tsc, "--project", filepath.Join(source, "tsconfig.json")).CombinedOutput(); err != nil {
		t.Fatalf("compile generated server compatibility target: %v\n%s", err, result)
	}
	output := filepath.Join(directory, "output")
	if err := os.WriteFile(filepath.Join(output, "package.json"), []byte(`{"type":"module"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	return output
}
