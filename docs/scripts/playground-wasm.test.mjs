import assert from "node:assert/strict";
import { readFileSync, mkdtempSync, mkdirSync, writeFileSync, rmSync } from "node:fs";
import { tmpdir } from "node:os";
import { resolve, dirname } from "node:path";
import { spawnSync } from "node:child_process";
import { test } from "node:test";
import { unzipSync, strFromU8 } from "fflate";
import { archiveArtifacts } from "../.vitepress/theme/playground/download.ts";

await import("../public/playground/wasm_exec.js");
const go = new globalThis.Go();
const wasm = await WebAssembly.instantiate(readFileSync(new URL("../public/playground/openapi-sdkgen.wasm", import.meta.url)), go.importObject);
void go.run(wasm.instance);
for (let attempt = 0; attempt < 100 && !globalThis.openapiSDKGenGenerate; attempt += 1) {
  await new Promise((resolve) => setTimeout(resolve, 10));
}
assert.equal(typeof globalThis.openapiSDKGenGenerate, "function");
const source = JSON.stringify({ openapi: "3.2.0", info: { title: "Playground", version: "1" }, paths: { "/health": { get: { operationId: "health", responses: { 204: { description: "Healthy" } } } } } });

test("WASM generates both targets and ZIP retains every exact file", async () => {
  for (const target of ["typescript", "golang"]) {
    const options = target === "golang" ? { goImportPath: "example.com/consumer/internal/api", goPackage: "client", addons: ["metadata"] } : {};
    const result = JSON.parse(globalThis.openapiSDKGenGenerate(source, target, JSON.stringify(options)));
    assert.equal(result.error, undefined);
    assert.equal(result.diagnostics, undefined);
    assert.ok(result.artifacts.length > 0);
    if (target === "golang") {
      const client = result.artifacts.find((artifact) => artifact.path === "client.go");
      assert.match(client.content, /package client/);
      assert.match(client.content, /example.com\/consumer\/internal\/api\/types/);
      assert.ok(result.artifacts.some((artifact) => artifact.path.startsWith("metadata/")));
      assert.ok(result.artifacts.every((artifact) => artifact.path.endsWith(".go") || artifact.path === "internal/runtime/unicode17/LICENSE.txt"));
      assert.ok(result.artifacts.some((artifact) => artifact.path === "internal/runtime/unicode17/LICENSE.txt"));
    } else assert.ok(result.artifacts.some((artifact) => artifact.path === "index.ts"));
    const archive = await archiveArtifacts(result.artifacts);
    const files = unzipSync(archive);
    assert.deepEqual(Object.keys(files), result.artifacts.map((artifact) => artifact.path));
    for (const artifact of result.artifacts) assert.equal(strFromU8(files[artifact.path]), artifact.content);
  }
});

test("WASM reports invalid options and preserves the two-argument TypeScript call", () => {
  assert.equal(JSON.parse(globalThis.openapiSDKGenGenerate(source, "typescript")).error, undefined);
  assert.ok(JSON.parse(globalThis.openapiSDKGenGenerate(source, "golang", "invalid JSON")).error);
  const result = JSON.parse(globalThis.openapiSDKGenGenerate(source, "golang", "{}"));
  assert.ok(result.error || result.diagnostics);
  assert.equal(result.artifacts, undefined);
});


test("WASM Go output compiles offline and validates a real HTTP response", () => {
  const schema = { type: "object", required: ["text", "url", "expression"], properties: {
    text: { type: "string", pattern: "(?i:^(?<word>\\p{L}+)-\\k<word>$)" },
    url: { type: "string", format: "iri", $vocabulary: { "https://json-schema.org/draft/2020-12/vocab/format-assertion": true } },
    expression: { type: "string", format: "regex", $vocabulary: { "https://json-schema.org/draft/2020-12/vocab/format-assertion": true } },
  } };
  const document = JSON.stringify({ openapi: "3.2.0", info: { title: "Owned runtime", version: "1" }, paths: { "/health": { get: { operationId: "health", responses: { 200: { description: "Healthy", content: { "application/json": { schema } } } } } } } });
  const result = JSON.parse(globalThis.openapiSDKGenGenerate(document, "golang", JSON.stringify({ goImportPath: "example.com/consumer/internal/api", addons: ["server"] })));
  assert.equal(result.error, undefined);
  assert.equal(result.diagnostics, undefined);
  const scratch = mkdtempSync(resolve(tmpdir(), "sdkgen-wasm-go-"));
  try {
    for (const artifact of result.artifacts) {
      const destination = resolve(scratch, "internal/api", artifact.path);
      mkdirSync(dirname(destination), { recursive: true });
      writeFileSync(destination, artifact.content);
    }
    writeFileSync(resolve(scratch, "go.mod"), "module example.com/consumer\n\ngo 1.27.0\n");
    writeFileSync(resolve(scratch, "main.go"), `package main
import("context";"encoding/json";"fmt";"net/http";"net/http/httptest";api "example.com/consumer/internal/api")
func main(){server:=httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter,r *http.Request){w.Header().Set("Content-Type","application/json");json.NewEncoder(w).Encode(map[string]string{"text":"Go-go","url":"https://例え.テスト/道","expression":"(?<=a)b"})}));defer server.Close();client,err:=api.NewClient(api.ClientOptions{BaseURL:server.URL});if err!=nil{panic(err)};raw,err:=client.Operations.Health().Raw(context.Background());if err!=nil{panic(err)};value,_:=raw.Data.Get("text");text,_:=value.String();fmt.Println(text)}
`);
    const env = { ...process.env, GOWORK: "off", GOTOOLCHAIN: "local", GOPROXY: "off", GOSUMDB: "off", CGO_ENABLED: "0", GOMODCACHE: resolve(scratch, "module-cache") };
    const compiled = spawnSync("go", ["run", "-mod=readonly", "."], { cwd: scratch, env, encoding: "utf8", timeout: 60000 });
    assert.equal(compiled.status, 0, `${compiled.error ?? ""} ${compiled.stderr}`);
    assert.equal(compiled.stdout.trim(), "Go-go");
    const graph = spawnSync("go", ["list", "-m", "-json", "all"], { cwd: scratch, env, encoding: "utf8", timeout: 10000 });
    assert.equal(graph.status, 0, graph.stderr);
    assert.equal(JSON.parse(graph.stdout).Path, "example.com/consumer");
    assert.equal(JSON.parse(graph.stdout).Main, true);
  } finally { rmSync(scratch, { recursive: true, force: true }); }
});
