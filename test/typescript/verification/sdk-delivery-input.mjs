import assert from "node:assert/strict";
import fs from "node:fs";
import path from "node:path";
import { createHash } from "node:crypto";

const hash = (bytes) => createHash("sha256").update(bytes).digest("hex");

/** Validate saved input before writing anything, including same-run recovery. */
export function writeFixedInput(destination, contents, reusedInput) {
  const expected = hash(contents);
  if (reusedInput !== undefined) {
    assert.equal(
      hash(fs.readFileSync(reusedInput)),
      expected,
      "Reused input differs from the fixed workload",
    );
  }
  if (fs.existsSync(destination)) {
    assert.equal(
      hash(fs.readFileSync(destination)),
      expected,
      "Existing run input differs from the fixed workload",
    );
    return;
  }
  fs.mkdirSync(path.dirname(destination), { recursive: true });
  fs.writeFileSync(destination, contents, { flag: "wx" });
}
