import assert from "node:assert/strict";
import path from "node:path";
import { fileURLToPath } from "node:url";
import { spawnSync } from "node:child_process";
import test from "node:test";

const root = fileURLToPath(new URL("../../../", import.meta.url));
const library = path.join(root, "scripts/agent/lib.sh");
// Shell functions isolate launcher choice; no installation, network or SDK source mutation.
function run(version, invocation) {
  const result = spawnSync(
    "bash",
    [
      "-c",
      `source "$1"\nmock_node_version="$2"\nnode() { printf '%s\\n' "$mock_node_version"; }\nfnm() { printf 'manager:%s\\n' "$*"; }\ncorepack() { printf 'corepack:%s\\n' "$*"; }\n${invocation}`,
      "node-selection-test",
      library,
      version,
    ],
    {
      cwd: root,
      encoding: "utf8",
      timeout: 10000,
    },
  );
  return result;
}

test("agent node helper reuses the exact active runtime instead of fnm", () => {
  const result = run("v24.21.0", 'ts_node printf "direct-command\\n"');
  assert.equal(result.status, 0, result.stderr);
  assert.equal(result.stdout, "direct-command\n");
});

test("agent pnpm helper retains its pin without re-entering fnm", () => {
  const result = run("v24.21.0", "ts_pnpm run typecheck");
  assert.equal(result.status, 0, result.stderr);
  assert.equal(
    result.stdout,
    `corepack:pnpm@12.4.1 --config.store-dir=${root.replace(/\/$/, "")}/.tmp/pnpm-store run typecheck\n`,
  );
});

test("a different active runtime still uses the pinned fnm fallback", () => {
  const result = run("v26.10.0", "ts_node node --version");
  assert.equal(result.status, 0, result.stderr);
  assert.equal(result.stdout, "manager:exec --using 24.21.0 node --version\n");
});

test("the reused runtime preserves child failure status", () => {
  const result = run("v24.21.0", "ts_node false");
  assert.equal(result.status, 1);
});
