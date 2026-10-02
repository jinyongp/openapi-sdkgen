import assert from "node:assert/strict";
import { readdirSync } from "node:fs";
import { resolve, join } from "node:path";
import { pathToFileURL } from "node:url";

const directory = resolve(process.argv[2]);
const sdk = await import(pathToFileURL(join(directory, "selective/index.js")));
const staticFiles = readdirSync(join(directory, "selective/operations"), { recursive: true })
  .filter((file) => file.endsWith(".js"));
assert.equal(staticFiles.length, 1, "The fixture exposes exactly one selected operation");
const reference = await import(pathToFileURL(join(directory, "selective/operations", staticFiles[0])));
const prepared = await sdk.loadOperations([
  sdk.operations.getTask,
  sdk.routes["GET /tasks/{task-id}"],
  reference.operation,
]);
const requests = [];
const api = sdk.createClient({
  operations: prepared,
  baseURL: "https://api.example.test",
  fetch: async (input) => {
    requests.push(String(input));
    return Response.json({ id: "one", state: "open" });
  },
});
assert.deepEqual(Object.keys(api.$routes), ["GET /tasks/{task-id}"]);
const response = await api.$routes["GET /tasks/{task-id}"]({ path: { "task-id": "one" } });
assert.equal(response.id, "one");
assert.deepEqual(requests, ["https://api.example.test/tasks/one"]);
const raw = await api.$operations.getTask.raw({ path: { "task-id": "one" } });
const owner = await api.$operations.getTask.links.owner(raw);
assert.equal(owner.id, "one");
assert.deepEqual(requests, ["https://api.example.test/tasks/one", "https://api.example.test/tasks/one", "https://api.example.test/owners"]);
assert.deepEqual(Object.keys(api.$routes), ["GET /tasks/{task-id}"]);
console.log("pass: operation ID, route, static reference and private Link target with mock Fetch");
