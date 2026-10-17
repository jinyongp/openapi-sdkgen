import { copyFileSync, mkdirSync, rmSync } from "node:fs";
import { resolve } from "node:path";
import { spawnSync } from "node:child_process";
import { fileURLToPath } from "node:url";

const docsDirectory = resolve(fileURLToPath(new URL("..", import.meta.url)));
const repositoryDirectory = resolve(docsDirectory, "..");
const outputDirectory = resolve(docsDirectory, "public", "playground");

mkdirSync(outputDirectory, { recursive: true });

const goRoot = spawnSync("go", ["env", "GOROOT"], {
  cwd: repositoryDirectory,
  encoding: "utf8",
});
if (goRoot.status !== 0) {
  process.stderr.write(goRoot.stderr);
  process.exit(goRoot.status ?? 1);
}

const build = spawnSync(
  "go",
  ["build", "-trimpath", "-ldflags=-s -w", "-o", resolve(outputDirectory, "openapi-sdkgen.wasm"), "./cmd/playground-wasm"],
  {
    cwd: repositoryDirectory,
    env: { ...process.env, GOOS: "js", GOARCH: "wasm" },
    stdio: "inherit",
  },
);
if (build.status !== 0) {
  process.exit(build.status ?? 1);
}

// Go's downloaded toolchain stores this asset read-only. Replace the old copy
// so repeated builds also work when its source permissions were preserved.
const wasmExecPath = resolve(outputDirectory, "wasm_exec.js");
rmSync(wasmExecPath, { force: true });
copyFileSync(
  resolve(goRoot.stdout.trim(), "lib", "wasm", "wasm_exec.js"),
  wasmExecPath,
);
copyFileSync(
  resolve(docsDirectory, "scripts", "playground-worker.js"),
  resolve(outputDirectory, "generator-worker.js"),
);
