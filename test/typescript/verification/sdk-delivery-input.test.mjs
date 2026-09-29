import assert from "node:assert/strict";
import fs from "node:fs";
import path from "node:path";
import { randomUUID } from "node:crypto";
import { fileURLToPath } from "node:url";
import { test } from "node:test";
import { writeFixedInput } from "./sdk-delivery-input.mjs";

const root = fileURLToPath(new URL("../../../", import.meta.url));
const first = JSON.stringify({ info: { version: "1" } });
const second = JSON.stringify({ info: { version: "2" } });
function fixture(t) {
  const directory = path.join(root, ".tmp/sdk-input-tests", randomUUID());
  fs.mkdirSync(directory, { recursive: true });
  t.after(() => fs.rmSync(directory, { recursive: true, force: true }));
  return directory;
}

test("a new run saves the exact fixed workload", (t) => {
  const file = path.join(fixture(t), "new/input.json");
  writeFixedInput(file, first);
  assert.equal(fs.readFileSync(file, "utf8"), first);
});

test("a matching reused workload can be written to a fresh run", (t) => {
  const directory = fixture(t);
  const original = path.join(directory, "original.json");
  const output = path.join(directory, "new/input.json");
  fs.writeFileSync(original, first);
  writeFixedInput(output, first, original);
  assert.equal(fs.readFileSync(output, "utf8"), first);
  assert.equal(fs.readFileSync(original, "utf8"), first);
});

test("same-run recovery rejects a changed document before overwriting its evidence", (t) => {
  const file = path.join(fixture(t), "input.json");
  fs.writeFileSync(file, first);
  assert.throws(() => writeFixedInput(file, second, file), /Reused input differs/);
  assert.equal(fs.readFileSync(file, "utf8"), first);
});

test("mismatched reuse creates neither an input nor its destination directory", (t) => {
  const directory = fixture(t);
  const original = path.join(directory, "original.json");
  const output = path.join(directory, "new/input.json");
  fs.writeFileSync(original, first);
  assert.throws(() => writeFixedInput(output, second, original), /Reused input differs/);
  assert(!fs.existsSync(path.dirname(output)));
  assert.equal(fs.readFileSync(original, "utf8"), first);
});

test("a pre-existing destination cannot be silently replaced", (t) => {
  const file = path.join(fixture(t), "input.json");
  fs.writeFileSync(file, first);
  assert.throws(() => writeFixedInput(file, second), /Existing run input differs/);
  assert.equal(fs.readFileSync(file, "utf8"), first);
});

test("matching same-run input is not rewritten", (t) => {
  const file = path.join(fixture(t), "input.json");
  fs.writeFileSync(file, first);
  const original = fs.statSync(file, { bigint: true });
  writeFixedInput(file, first, file);
  const current = fs.statSync(file, { bigint: true });
  assert.equal(current.mtimeNs, original.mtimeNs);
  assert.equal(current.ino, original.ino);
  assert.equal(fs.readFileSync(file, "utf8"), first);
});
