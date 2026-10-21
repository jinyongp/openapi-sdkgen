import assert from "node:assert/strict";
import fs from "node:fs";
import os from "node:os";
import path from "node:path";
import { spawnSync } from "node:child_process";
import { fileURLToPath } from "node:url";

const here = path.dirname(fileURLToPath(import.meta.url)), root = path.resolve(here, "../../..");
const temp = fs.mkdtempSync(path.join(os.tmpdir(), "sdk-projects-"));
const run = (command, args) => spawnSync(command, args, { cwd: root, encoding: "utf8" });
const node = (script, ...args) => run(process.execPath, [path.join(here, script), ...args]);
const fixture = (name, invalid = false) => {
  const sdk = path.join(temp, name); fs.mkdirSync(sdk);
  const files = {
    "a.ts": 'import type { B } from "./b.js"; export interface A { readonly next?: B; }\n',
    "b.ts": 'import type { A } from "./a.js"; export interface B { readonly next?: A; }\n',
    "identity.ts": 'export const token: unique symbol = Symbol("nominal");\n/*' + '.'.repeat(800) + '*/\n',
    "calls.ts": 'import type { A } from "./a.js"; import type { token } from "./identity.js";\nexport interface Routes { readonly "GET /one": { input: A; output: { readonly [token]: true } }; }\nexport interface OperationRoutes { readonly one: "GET /one"; }\nexport type OperationMethod<R extends keyof Routes> = (input: Routes[R]["input"]) => Promise<Routes[R]["output"]>;\nexport interface Client { readonly $routes: { readonly "GET /one": OperationMethod<"GET /one"> }; readonly $operations: { readonly one: OperationMethod<"GET /one"> }; }\n',
    "index.ts": 'export type { Client, OperationRoutes, OperationMethod } from "./calls.js";\n',
    "lazy.ts": 'export async function load(): Promise<unknown> { return import("./identity.js"); }\n',
  };
  if (invalid) files["invalid.ts"] = 'export function invalid(): string { return 123; }\n';
  for (const [name, text] of Object.entries(files)) fs.writeFileSync(path.join(sdk, name), text);
  fs.writeFileSync(path.join(sdk, "package.json"), JSON.stringify({ type: "module" }));
  const inventory = path.join(temp, name + "-inventory"), plan = path.join(temp, name + "-plan");
  let result = node("inventory.mjs", sdk, inventory); assert.equal(result.status, 0, result.stderr);
  result = node("project-plan.mjs", inventory, plan, "1000", "2"); assert.equal(result.status, 0, result.stderr);
  return { sdk, inventory, plan: path.join(plan, "plan.json") };
};
try {
  const good = fixture("good"), plan = JSON.parse(fs.readFileSync(good.plan, "utf8"));
  assert.equal(plan.files, 6); assert.ok(plan.groups.length >= 4);
  const owner = new Map(plan.groups.flatMap((group) => group.files.map((file) => [file, group.id])));
  assert.equal(owner.get("a.ts"), owner.get("b.ts"));
  assert.ok(owner.get("identity.ts") < owner.get("calls.ts"));
  assert.ok(owner.get("calls.ts") <= owner.get("index.ts"));
  const reference = path.join(temp, "reference.json");
  fs.writeFileSync(reference, JSON.stringify({ routes: ["GET /one"], operations: ["one"], operationRoutes: { one: "GET /one" } }));
  const usage = path.join(temp, "usage.mts");
  fs.writeFileSync(usage, 'import type { Client } from "__SDK_ROOT__";\nexport function pick(client: Client): keyof Client["$operations"] {\n// @ts-expect-error unknown route\nclient.$routes["GET /missing"];\nreturn "one";\n}\n');
  const check = (fixture, name, compiler = "typescript-5-9", removeComments = false) => run("python3", [path.join(here, "check-projects.py"), "--node", process.execPath,
    "--plan", fixture.plan, "--output", path.join(temp, name), "--reference", reference,
    "--compiler", path.join(root, "test/typescript/node_modules", compiler, "lib/tsc.js"), "--timeout", "120", "--usage-consumer", usage, ...(removeComments ? ["--remove-declaration-comments"] : [])]);
  let result = check(good, "good-check"); assert.equal(result.status, 0, result.stdout + result.stderr);
  const trace = path.join(temp, "reference-phases.jsonl");
  result = node("trace-ts-phases.mjs", "typescript-5-9", path.join(temp, "good-check/configs/2.json"), trace);
  assert.equal(result.status, 0, result.stdout + result.stderr);
  const phases = fs.readFileSync(trace, "utf8").trim().split("\n").map(JSON.parse);
  assert.ok(phases[0].projectReferences > 0);
  assert.ok(phases.find((phase) => phase.phase === "program-created").declarationFiles > 0);
  assert.equal(phases.find((phase) => phase.phase === "checked").diagnostics, 0);
  result = check(good, "native-check", "typescript"); assert.equal(result.status, 0, result.stdout + result.stderr);
  result = check(good, "commentless-check", "typescript-5-9", true); assert.equal(result.status, 0, result.stdout + result.stderr);
  result = check(good, "good-check"); assert.notEqual(result.status, 0, "must reject existing outputs/build info");
  const bad = fixture("bad", true);
  result = check(bad, "bad-check"); assert.notEqual(result.status, 0, "unexported internal semantic error must fail");
  const badResult = JSON.parse(fs.readFileSync(path.join(temp, "bad-check/result.json"), "utf8"));
  assert.equal(badResult.status, "fail"); assert.ok(badResult.completedProjects < badResult.projects);
  const failure = fs.readdirSync(path.join(temp, "bad-check")).filter((name) => name.startsWith("project-")).map((name) => fs.readFileSync(path.join(temp, "bad-check", name, "stdout.log"), "utf8")).join("");
  assert.match(failure, /TS2322/);
  fs.appendFileSync(path.join(good.sdk, "a.ts"), "// changed\n");
  result = check(good, "changed-check"); assert.notEqual(result.status, 0);
  assert.match(result.stderr, /changed or unowned source/);
  result = node("project-plan.mjs", good.inventory, path.join(temp, "changed-plan")); assert.notEqual(result.status, 0);
  const added = fixture("added");
  fs.writeFileSync(path.join(added.sdk, "extra.ts"), "export const extra: number = 1;\n");
  result = check(added, "added-check"); assert.notEqual(result.status, 0);
  assert.match(result.stderr, /sources added\/removed/);
  result = node("project-plan.mjs", added.inventory, path.join(temp, "incomplete-plan")); assert.notEqual(result.status, 0);
  assert.match(result.stderr, /inventory is incomplete/);
  fs.appendFileSync(path.join(bad.sdk, "identity.ts"), 'declare global { interface Window { sdkTest: true; } }\n');
  const augmented = path.join(temp, "augmented-inventory");
  result = node("inventory.mjs", bad.sdk, augmented); assert.equal(result.status, 0, result.stderr);
  result = node("project-plan.mjs", augmented, path.join(temp, "augmented-plan")); assert.notEqual(result.status, 0);
  assert.match(result.stderr, /augmentation/);
  console.log("complete ownership, SCC, cross-project nominal identity, native/JS strict, internal error and stale-source rejection: pass");
} finally { fs.rmSync(temp, { recursive: true, force: true }); }
