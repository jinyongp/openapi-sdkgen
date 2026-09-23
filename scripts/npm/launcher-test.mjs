import assert from "node:assert/strict";
import { spawnSync } from "node:child_process";
import { createHash } from "node:crypto";
import { access, chmod, mkdir, mkdtemp, readFile, rm, writeFile } from "node:fs/promises";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { pathToFileURL } from "node:url";

const [packageDirectory, installedBin] = process.argv.slice(2);
if (!packageDirectory) {
  throw new Error("usage: launcher-test.mjs <package-directory> [installed-bin]");
}

const launcher = await import(pathToFileURL(join(packageDirectory, "lib", "launcher.js")).href);
const packageJSON = JSON.parse(await readFile(join(packageDirectory, "package.json"), "utf8"));
const version = packageJSON.version;
const releaseBaseURL = "https://release.example.test/download/";
const expectedTargets = [
  ["darwin", "x64", "darwin", "amd64", ""],
  ["darwin", "arm64", "darwin", "arm64", ""],
  ["linux", "x64", "linux", "amd64", ""],
  ["linux", "arm64", "linux", "arm64", ""],
  ["win32", "x64", "windows", "amd64", ".exe"],
  ["win32", "arm64", "windows", "arm64", ".exe"],
];

for (const [platform, architecture, os, arch, extension] of expectedTargets) {
  const target = launcher.resolveReleaseTarget(platform, architecture);
  assert.deepEqual(
    { os: target.os, arch: target.arch, extension: target.extension },
    { os, arch, extension },
  );
  assert.equal(
    launcher.releaseAssetName(version, target),
    `openapi-sdkgen_${version}_${os}_${arch}${extension}`,
  );
}
assert.throws(
  () => launcher.resolveReleaseTarget("freebsd", "x64"),
  /unsupported platform/,
);

const root = await mkdtemp(join(tmpdir(), "openapi-sdkgen-launcher-test-"));
try {
  const payload = Buffer.from("#!/bin/sh\nprintf 'fixture binary\\n'\n");
  const digest = sha256(payload);
  const linuxTarget = launcher.resolveReleaseTarget("linux", "x64");
  const asset = launcher.releaseAssetName(version, linuxTarget);
  const requests = [];

  const fetchImpl = async (url) => {
    const value = String(url);
    requests.push(value);
    if (value === `${releaseBaseURL}v${version}/checksums.txt`) {
      return new Response(`${digest}  ${asset}\n`);
    }
    if (value === `${releaseBaseURL}v${version}/${asset}`) {
      return new Response(payload);
    }
    return new Response(null, { status: 404 });
  };

  const cacheRoot = join(root, "success");
  const executable = await launcher.ensureBinary({
    version,
    platform: "linux",
    architecture: "x64",
    cacheRoot,
    releaseBaseURL,
    fetchImpl,
  });
  assert.deepEqual(await readFile(executable), payload);
  assert.deepEqual(requests, [
    `${releaseBaseURL}v${version}/checksums.txt`,
    `${releaseBaseURL}v${version}/${asset}`,
  ]);

  const cached = await launcher.ensureBinary({
    version,
    platform: "linux",
    architecture: "x64",
    cacheRoot,
    releaseBaseURL,
    fetchImpl: async () => {
      throw new Error("cache hit attempted network access");
    },
  });
  assert.equal(cached, executable);

  await assert.rejects(
    launcher.ensureBinary({
      version,
      platform: "linux",
      architecture: "x64",
      cacheRoot: join(root, "missing-checksum"),
      releaseBaseURL,
      fetchImpl: async (url) =>
        String(url).endsWith("checksums.txt")
          ? new Response(`${digest}  another-file\n`)
          : new Response(payload),
    }),
    /do not contain/,
  );

  const corruptRoot = join(root, "checksum-mismatch");
  await assert.rejects(
    launcher.ensureBinary({
      version,
      platform: "linux",
      architecture: "x64",
      cacheRoot: corruptRoot,
      releaseBaseURL,
      fetchImpl: async (url) =>
        String(url).endsWith("checksums.txt")
          ? new Response(`${digest}  ${asset}\n`)
          : new Response(Buffer.from("corrupt")),
    }),
    /checksum mismatch/,
  );
  const corruptPaths = launcher.cacheEntryPaths({
    cacheRoot: corruptRoot,
    version,
    platform: "linux",
    architecture: "x64",
  });
  await assert.rejects(access(corruptPaths.executable));

  await assert.rejects(
    launcher.ensureBinary({
      version,
      platform: "linux",
      architecture: "x64",
      cacheRoot: join(root, "oversized"),
      releaseBaseURL,
      fetchImpl: async (url) =>
        String(url).endsWith("checksums.txt")
          ? new Response(`${digest}  ${asset}\n`)
          : new Response(payload, {
              headers: { "content-length": String(128 * 1024 * 1024 + 1) },
            }),
    }),
    /download limit/,
  );

  let concurrentRequests = 0;
  const concurrentFetch = async (url) => {
    concurrentRequests += 1;
    await new Promise((resolve) => setTimeout(resolve, 20));
    return String(url).endsWith("checksums.txt")
      ? new Response(`${digest}  ${asset}\n`)
      : new Response(payload);
  };
  const concurrentRoot = join(root, "concurrent");
  const [first, second] = await Promise.all([
    launcher.ensureBinary({
      version,
      platform: "linux",
      architecture: "x64",
      cacheRoot: concurrentRoot,
      releaseBaseURL,
      fetchImpl: concurrentFetch,
    }),
    launcher.ensureBinary({
      version,
      platform: "linux",
      architecture: "x64",
      cacheRoot: concurrentRoot,
      releaseBaseURL,
      fetchImpl: concurrentFetch,
    }),
  ]);
  assert.equal(first, second);
  assert.equal(concurrentRequests, 2);
  assert.deepEqual(await readFile(first), payload);

  let spawned;
  const status = launcher.executeBinary("/fixture/openapi-sdkgen", ["one", "two"], {
    spawnImpl(executablePath, args, options) {
      spawned = { executablePath, args, options };
      return { status: 23 };
    },
  });
  assert.equal(status, 23);
  assert.equal(spawned.executablePath, "/fixture/openapi-sdkgen");
  assert.deepEqual(spawned.args, ["one", "two"]);
  assert.equal(spawned.options.stdio, "inherit");

  if (installedBin !== undefined && process.platform !== "win32") {
    const installedCache = join(root, "installed");
    const paths = launcher.cacheEntryPaths({
      cacheRoot: installedCache,
      version,
      platform: process.platform,
      architecture: process.arch,
    });
    const fixture = Buffer.from(
      `#!/bin/sh
if [ "\$1" = "--version" ]; then
  printf 'openapi-sdkgen ${version}\\n'
  exit 0
fi
printf 'ARGS:%s\\n' "\$*"
exit 23
`,
    );
    await mkdir(paths.directory, { recursive: true, mode: 0o700 });
    await writeFile(paths.executable, fixture, { mode: 0o755 });
    await chmod(paths.executable, 0o755);
    await writeFile(paths.checksum, `${sha256(fixture)}\n`, { mode: 0o600 });

    const environment = {
      ...process.env,
      OPENAPI_SDKGEN_CACHE_DIR: installedCache,
    };
    const versionResult = spawnSync(installedBin, ["--version"], {
      encoding: "utf8",
      env: environment,
    });
    assert.equal(versionResult.status, 0, versionResult.stderr);
    assert.equal(versionResult.stdout.trim(), `openapi-sdkgen ${version}`);

    const argsResult = spawnSync(installedBin, ["__launcher_test__", "alpha"], {
      encoding: "utf8",
      env: environment,
    });
    assert.equal(argsResult.status, 23, argsResult.stderr);
    assert.equal(argsResult.stdout.trim(), "ARGS:__launcher_test__ alpha");
  }
} finally {
  await rm(root, { recursive: true, force: true });
}

function sha256(value) {
  return createHash("sha256").update(value).digest("hex");
}
