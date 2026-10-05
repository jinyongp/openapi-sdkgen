import assert from "node:assert/strict";
import fs from "node:fs";
import os from "node:os";
import path from "node:path";
import { spawnSync } from "node:child_process";
import { fileURLToPath } from "node:url";
import { test } from "node:test";

test("private comparisons retain detailed causes locally and protect their artifacts", () => {
  const directory = fs.mkdtempSync(path.join(os.tmpdir(), "sdkgen-private-measurement-"));
  try {
    const input = path.join(directory, "input.json");
    const manifest = path.join(directory, "manifest.json");
    fs.writeFileSync(input, JSON.stringify({ paths: {} }));
    const sentinel = "CONFIDENTIAL_PROVIDER_SENTINEL";
    for (const privateInput of [false, true]) {
      const output = path.join(directory, String(privateInput));
      fs.writeFileSync(
        manifest,
        JSON.stringify({
          private: privateInput,
          providers: [{ name: sentinel, input, sha256: "invalid-pinned-hash" }],
        }),
      );
      const run = spawnSync(
        process.execPath,
        [
          fileURLToPath(new URL("./provider-size-measure.mjs", import.meta.url)),
          output,
          process.execPath,
          process.execPath,
          manifest,
        ],
        { encoding: "utf8" },
      );
      assert.equal(run.status, 1);
      if (privateInput) {
        assert(
          !`${run.stdout}${run.stderr}`.includes(sentinel),
          "private input names escaped to command output",
        );
        const diagnostic = path.join(output, "failure.private.log");
        assert(fs.readFileSync(diagnostic, "utf8").includes(sentinel), "actual cause was lost");
        if (process.platform !== "win32") {
          assert.equal(fs.statSync(output).mode & 0o777, 0o700);
          assert.equal(fs.statSync(diagnostic).mode & 0o777, 0o600);
        }
      } else {
        assert(
          `${run.stdout}${run.stderr}`.includes(sentinel),
          "public input failure cause was hidden",
        );
      }
    }
  } finally {
    fs.rmSync(directory, { recursive: true, force: true });
  }
});
