import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { registerHooks, createRequire } from "node:module";
import { resolve } from "node:path";
import { pathToFileURL } from "node:url";

const source = pathToFileURL(resolve(process.argv[2]) + "/").href;
const require = createRequire(pathToFileURL(resolve(process.argv[3], "package.json")));
const ts = require("typescript-5-9");
registerHooks({
  resolve(specifier, context, next) {
    if (specifier.endsWith(".js") && (specifier.startsWith(source) || context.parentURL?.startsWith(source))) {
      return next(specifier.slice(0, -3) + ".ts", context);
    }
    return next(specifier, context);
  },
  load(url, context, next) {
    if (url.startsWith(source) && url.endsWith(".ts")) {
      return { format: "module", shortCircuit: true, source: ts.transpileModule(readFileSync(new URL(url), "utf8"), {
        compilerOptions: { target: ts.ScriptTarget.ES2022, module: ts.ModuleKind.ESNext },
      }).outputText };
    }
    return next(url, context);
  },
});
const sdk = await import(new URL("selective/index.ts", source));
const names = await import(new URL("selective/all.ts", source));
assert.deepEqual(Object.keys(names.routes), ["GET /users/$count"]);
const operations = await sdk.loadOperations([sdk.routes["GET /users/$count"]]);
let body = "3";
const api = sdk.createClient({
  baseURL: "https://graph.example.test/beta",
  operations,
  fetch: async (url, init) => {
    assert.equal(String(url), "https://graph.example.test/beta/users/$count");
    assert.equal(init.method, "GET");
    return new Response(body, { headers: { "content-type": "text/plain; charset=utf-8" } });
  },
});
assert.equal(await api.$routes["GET /users/$count"](), 3);
assert.equal((await api.$routes["GET /users/$count"].raw()).data, 3);
body = "0";
assert.equal(await api.$routes["GET /users/$count"](), 0);
for (const invalid of ["", " ", "3items", "3.5", "NaN", "Infinity"]) {
  body = invalid;
  await assert.rejects(api.$routes["GET /users/$count"](), error => error.code === "RESPONSE_DECODE_FAILED");
}
console.log("pass: Graph count selected loadOperations, typed scalar/raw responses and malformed-body rejection with mock Fetch");
