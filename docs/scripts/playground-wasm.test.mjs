import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
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
      assert.ok(result.artifacts.every((artifact) => artifact.path.endsWith(".go")));
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
