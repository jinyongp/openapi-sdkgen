import assert from "node:assert/strict";
import fs from "node:fs";
import path from "node:path";
import { randomUUID } from "node:crypto";
import { spawn, spawnSync } from "node:child_process";
import { once } from "node:events";
import { fileURLToPath } from "node:url";
import { test } from "node:test";

import { requireVerificationSpace } from "./sdk-delivery-compile.mjs";
import { runMeasured } from "./measured-process.mjs";

const root = fileURLToPath(new URL("../../../", import.meta.url));

test(
  "a hangup terminates the detached measured workload",
  { skip: process.platform === "win32" },
  async (t) => {
    const directory = path.join(root, ".tmp/sdk-runner-tests", randomUUID());
    fs.mkdirSync(directory, { recursive: true });
    t.after(() => fs.rmSync(directory, { recursive: true, force: true }));
    const marker = path.join(directory, "late-write");
    const ready = path.join(directory, "ready");
    const helper = path.join(directory, "helper.mjs");
    const workload = `require('node:fs').writeFileSync(${JSON.stringify(ready)},'ready');setTimeout(()=>require('node:fs').writeFileSync(${JSON.stringify(marker)},'late'),1000)`;
    fs.writeFileSync(
      helper,
      `import {runMeasured} from ${JSON.stringify(new URL("./measured-process.mjs", import.meta.url).href)};await runMeasured(process.execPath,['-e',${JSON.stringify(workload)}],${JSON.stringify({ cwd: directory, resourceFile: path.join(directory, "resources.json"), timeout: 10000 })});`,
    );
    const child = spawn(process.execPath, [helper], { stdio: "ignore" });
    t.after(() => child.kill());
    const closed = once(child, "exit");
    const deadline = Date.now() + 5000;
    while (!fs.existsSync(ready) && Date.now() < deadline)
      await new Promise((resolve) => setTimeout(resolve, 10));
    assert(fs.existsSync(ready), "measured workload did not start");
    child.kill("SIGHUP");
    const [status, signal] = await closed;
    assert.equal(status, 129);
    assert.equal(signal, null);
    await new Promise((resolve) => setTimeout(resolve, 1100));
    assert(!fs.existsSync(marker), "child continued writing after hangup");
  },
);

test("a timed measurement terminates its workload before releasing ownership", async (t) => {
  const directory = path.join(root, ".tmp/sdk-runner-tests", randomUUID());
  fs.mkdirSync(directory, { recursive: true });
  t.after(() => fs.rmSync(directory, { recursive: true, force: true }));
  const marker = path.join(directory, "late-write");
  const result = await runMeasured(
    process.execPath,
    [
      "-e",
      `setTimeout(()=>require('node:fs').writeFileSync(${JSON.stringify(marker)},'late'),1000)`,
    ],
    { cwd: directory, resourceFile: path.join(directory, "resources.json"), timeout: 300 },
  );
  assert.equal(result.error?.code, "ETIMEDOUT");
  await new Promise((resolve) => setTimeout(resolve, 1100));
  assert(!fs.existsSync(marker), "timed-out child continued writing");
});

test("storage preflight uses account-available bytes rather than reserved free blocks", () => {
  assert.throws(
    () =>
      requireVerificationSpace(root, 4096, () => ({
        bsize: 4096,
        bavail: 0,
        bfree: 1000000,
        ffree: 1000,
      })),
    /found 0/,
  );
  assert.deepEqual(
    requireVerificationSpace(root, 4096, () => ({
      bsize: 4096,
      bavail: 2,
      bfree: 1000000,
      ffree: 1000,
    })),
    { availableBytes: 8192, minimumBytes: 4096, freeInodes: 1000 },
  );
});

function runnerFixture(t) {
  const directory = path.join(root, ".tmp/sdk-runner-tests", randomUUID());
  const scripts = path.join(directory, "scripts/verification");
  fs.mkdirSync(scripts, { recursive: true });
  fs.mkdirSync(path.join(directory, ".tmp"));
  const wrapper = path.join(scripts, "sdk-delivery-check.sh");
  fs.copyFileSync(path.join(root, "scripts/verification/sdk-delivery-check.sh"), wrapper);
  fs.mkdirSync(path.join(directory, "scripts/lib"), { recursive: true });
  // Run the real script and lock; substitute only the expensive command bodies.
  fs.writeFileSync(
    path.join(directory, "scripts/lib/commands.sh"),
    `ROOT=${JSON.stringify(directory)}
TYPESCRIPT_ROOT="$ROOT/test"
SCRIPT_ARGS=("$@")
run_step() { printf '%s\\n' "$1" >> "$ROOT/calls"; return "\${TEST_FAILURE_STATUS:-0}"; }
`,
  );
  t.after(() => fs.rmSync(directory, { recursive: true, force: true }));
  return { directory, wrapper, lock: path.join(directory, ".tmp/sdk-delivery-check.lock") };
}

test("an existing SDK run lock rejects a second run before any compiler or generator", (t) => {
  const { directory, wrapper, lock } = runnerFixture(t);
  fs.mkdirSync(lock);
  fs.writeFileSync(path.join(lock, "owner"), "other-run\n");
  const result = spawnSync("bash", [wrapper], { encoding: "utf8", timeout: 10000 });
  assert.equal(result.status, 75);
  assert.match(result.stderr, /already locked/);
  assert(!fs.existsSync(path.join(directory, "calls")));
  assert.equal(fs.readFileSync(path.join(lock, "owner"), "utf8"), "other-run\n");
});

for (const failure of [0, 17]) {
  test(`SDK runner releases its lock and preserves exit status ${failure}`, (t) => {
    const { directory, wrapper, lock } = runnerFixture(t);
    const result = spawnSync("bash", [wrapper], {
      encoding: "utf8",
      timeout: 10000,
      env: { ...process.env, TEST_FAILURE_STATUS: String(failure) },
    });
    assert.equal(result.status, failure, result.stderr);
    assert(!fs.existsSync(lock));
  });
}
