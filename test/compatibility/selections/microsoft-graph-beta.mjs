import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { registerHooks, createRequire } from "node:module";
import { resolve } from "node:path";
import { pathToFileURL } from "node:url";

const source = pathToFileURL(resolve(process.argv[2]) + "/").href;
const require = createRequire(pathToFileURL(resolve(process.argv[3], "package.json")));
const ts = require("typescript-5-9");
// Run the generated SDK directly: the hook transpiles only modules actually loaded.
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
assert.equal(Object.keys(names.routes).length, 9);
const selected = [sdk.routes["GET /users"], sdk.routes["GET /users/{user-id}"], sdk.routes["PATCH /users/{user-id}"]];
const operations = await sdk.loadOperations(selected);
const requests = [];
const client = sdk.createClient({ baseURL: "https://graph.example.test/beta", operations, fetch: async (url, init) => {
  requests.push({ url: String(url), method: init.method, body: init.body });
  return init.method === "PATCH" ? new Response(null, { status: 204 }) : new Response(JSON.stringify(
    String(url).includes("/users/") ? { "@odata.type": "#microsoft.graph.user", id: "one", displayName: "Selected user" } : { value: [{ "@odata.type": "#microsoft.graph.user", id: "one" }] },
  ), { headers: { "content-type": "application/json" } });
} });
const list = await client.$routes["GET /users"]({ query: { "$top": 1 } });
assert.equal(list.value[0].id, "one");
const user = await client.$routes["GET /users/{user-id}"]({ path: { "user-id": "one /two" } });
assert.equal(user.id, "one");
await client.$routes["PATCH /users/{user-id}"]({ path: { "user-id": "one" }, body: { "@odata.type": "#microsoft.graph.user", displayName: "Updated" } });
assert.match(requests[0].url, /\$top=1|%24top=1/);
assert.match(requests[1].url, /one%20%2Ftwo/);
assert.equal(requests[2].method, "PATCH");
assert.equal(JSON.parse(requests[2].body).displayName, "Updated");
assert.equal(Object.keys(client.$routes).length, 3);
console.log("pass: Graph selected loadOperations, query/path/body, model decoding and mock Fetch");
