import assert from "node:assert/strict";
import fs from "node:fs";
import os from "node:os";
import path from "node:path";
import { test } from "node:test";
import { runtimeBoundaryViolations } from "./runtime-boundaries.mjs";

test("canonical runtime includes no forbidden dependency or ownership cycle", () => {
  const root = new URL("../../../internal/target/typescript/runtime/", import.meta.url);
  assert.deepEqual(runtimeBoundaryViolations(root.pathname), []);
});

test("boundaries reject value/type/re-export/import type/dynamic edges and cycles", () => {
  const root = fs.mkdtempSync(path.join(os.tmpdir(), "sdkgen-boundaries-"));
  try {
    fs.mkdirSync(path.join(root, "schema"));
    fs.mkdirSync(path.join(root, "http"));
    fs.writeFileSync(path.join(root, "http/a.ts"), "export interface A {}\n");
    for (const source of [
      'import { A } from "../http/a.js";',
      'import type { A } from "../http/a.js";',
      'export type { A } from "../http/a.js";',
      'type A = import("../http/a.js").A;',
      'const a = import("../http/a.js");',
      "const a = import(url);",
    ]) {
      fs.writeFileSync(path.join(root, "schema/a.ts"), source);
      assert.equal(runtimeBoundaryViolations(root).length, 1, source);
    }
    fs.writeFileSync(path.join(root, "schema/a.ts"), 'import type { B } from "./b.js";');
    fs.writeFileSync(path.join(root, "schema/b.ts"), 'import type { A } from "./a.js";');
    assert.match(runtimeBoundaryViolations(root)[0], /^cycle:/);
  } finally {
    fs.rmSync(root, { recursive: true, force: true });
  }
});

test("generated composition factories cannot capture provider or client dependencies", () => {
  const root = fs.mkdtempSync(path.join(os.tmpdir(), "sdkgen-compositions-"));
  try {
    fs.mkdirSync(path.join(root, "internal/execution-compositions"), { recursive: true });
    fs.mkdirSync(path.join(root, "internal/client"), { recursive: true });
    fs.writeFileSync(path.join(root, "internal/client/factory.ts"), "export interface Client {}\n");
    fs.writeFileSync(
      path.join(root, "internal/execution-compositions/services.ts"),
      'import type { Client } from "../client/factory.js";',
    );
    assert.match(
      runtimeBoundaryViolations(root, { generated: true })[0],
      /forbidden composition -> client/,
    );
    fs.writeFileSync(
      path.join(root, "internal/execution-compositions/services.ts"),
      "const services = createServices();",
    );
    assert.match(
      runtimeBoundaryViolations(root, { generated: true })[0],
      /composition initializes module state/,
    );
  } finally {
    fs.rmSync(root, { recursive: true, force: true });
  }
});
