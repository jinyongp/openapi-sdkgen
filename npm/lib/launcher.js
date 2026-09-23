import { spawnSync } from "node:child_process";
import { createHash, randomUUID } from "node:crypto";
import { chmod, mkdir, readFile, rename, rm, stat, writeFile } from "node:fs/promises";
import { homedir } from "node:os";
import { isAbsolute, join, resolve } from "node:path";
import { setTimeout as delay } from "node:timers/promises";

const defaultReleaseBaseURL =
  "https://github.com/jinyongp/openapi-sdkgen/releases/download";
const maxChecksumBytes = 1024 * 1024;
const maxBinaryBytes = 128 * 1024 * 1024;
const maxRedirects = 5;
const downloadTimeoutMs = 30_000;
const cacheLockTimeoutMs = 30_000;
const staleCacheLockMs = 120_000;
const semverPattern =
  /^(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)(?:-([0-9A-Za-z-]+)(?:\.[0-9A-Za-z-]+)*)?(?:\+([0-9A-Za-z-]+)(?:\.[0-9A-Za-z-]+)*)?$/;

const targets = {
  darwin: {
    arm64: { os: "darwin", arch: "arm64", extension: "" },
    x64: { os: "darwin", arch: "amd64", extension: "" },
  },
  linux: {
    arm64: { os: "linux", arch: "arm64", extension: "" },
    x64: { os: "linux", arch: "amd64", extension: "" },
  },
  win32: {
    arm64: { os: "windows", arch: "arm64", extension: ".exe" },
    x64: { os: "windows", arch: "amd64", extension: ".exe" },
  },
};

export function resolveReleaseTarget(
  platform = process.platform,
  architecture = process.arch,
) {
  const target = targets[platform]?.[architecture];
  if (target === undefined) {
    throw new Error(
      `unsupported platform ${platform}/${architecture}; supported platforms are darwin, linux, and win32 on arm64 or x64`,
    );
  }
  return {
    ...target,
    key: `${target.os}-${target.arch}`,
    executable: `openapi-sdkgen${target.extension}`,
  };
}

export function releaseAssetName(version, target) {
  validateVersion(version);
  return `openapi-sdkgen_${version}_${target.os}_${target.arch}${target.extension}`;
}

export function resolveCacheRoot({
  platform = process.platform,
  env = process.env,
  home = homedir(),
} = {}) {
  const override = env.OPENAPI_SDKGEN_CACHE_DIR;
  if (override !== undefined && override !== "") {
    return isAbsolute(override) ? override : resolve(override);
  }
  if (platform === "win32") {
    const base = env.LOCALAPPDATA || join(home, "AppData", "Local");
    return join(base, "openapi-sdkgen", "cache");
  }
  if (platform === "darwin") {
    return join(home, "Library", "Caches", "openapi-sdkgen");
  }
  const base = env.XDG_CACHE_HOME || join(home, ".cache");
  return join(base, "openapi-sdkgen");
}

export function cacheEntryPaths({
  cacheRoot,
  version,
  platform = process.platform,
  architecture = process.arch,
}) {
  validateVersion(version);
  const target = resolveReleaseTarget(platform, architecture);
  const directory = join(cacheRoot, version, target.key);
  return {
    target,
    directory,
    executable: join(directory, target.executable),
    checksum: join(directory, "sha256"),
    versionDirectory: join(cacheRoot, version),
  };
}

export async function ensureBinary({
  version,
  platform = process.platform,
  architecture = process.arch,
  env = process.env,
  cacheRoot = resolveCacheRoot({ platform, env }),
  releaseBaseURL = defaultReleaseBaseURL,
  fetchImpl = globalThis.fetch,
} = {}) {
  validateVersion(version);
  if (typeof fetchImpl !== "function") {
    throw new Error("Fetch API is unavailable in this Node.js runtime");
  }
  const releaseBase = new URL(ensureTrailingSlash(releaseBaseURL));
  if (releaseBase.protocol !== "https:") {
    throw new Error("release download origin must use HTTPS");
  }

  const paths = cacheEntryPaths({ cacheRoot, version, platform, architecture });
  if (await hasValidCacheEntry(paths)) {
    return paths.executable;
  }
  await mkdir(paths.versionDirectory, { recursive: true, mode: 0o700 });

  return withCacheLock(paths, async () => {
    if (await hasValidCacheEntry(paths)) {
      return paths.executable;
    }
    await rm(paths.directory, { recursive: true, force: true });

    const tagBase = new URL(`v${version}/`, releaseBase);
    const asset = releaseAssetName(version, paths.target);
    const checksumBytes = await fetchBytes(new URL("checksums.txt", tagBase), {
      fetchImpl,
      maxBytes: maxChecksumBytes,
      label: "release checksums",
    });
    const expectedDigest = checksumForAsset(checksumBytes, asset);
    const binaryBytes = await fetchBytes(new URL(asset, tagBase), {
      fetchImpl,
      maxBytes: maxBinaryBytes,
      label: `release binary ${asset}`,
    });
    const actualDigest = sha256(binaryBytes);
    if (actualDigest !== expectedDigest) {
      throw new Error(
        `release binary checksum mismatch for ${asset}: expected ${expectedDigest}, got ${actualDigest}`,
      );
    }

    await publishCacheEntry(paths, binaryBytes, expectedDigest);
    if (!(await hasValidCacheEntry(paths))) {
      throw new Error(`cached release binary failed verification for ${paths.target.key}`);
    }
    return paths.executable;
  });
}

export function executeBinary(
  executable,
  args,
  { spawnImpl = spawnSync } = {},
) {
  const result = spawnImpl(executable, args, { stdio: "inherit" });
  if (result.error !== undefined) {
    throw new Error(`failed to execute cached openapi-sdkgen binary: ${result.error.message}`);
  }
  if (result.status === null) {
    throw new Error(
      `openapi-sdkgen binary terminated without an exit status${result.signal ? ` (${result.signal})` : ""}`,
    );
  }
  return result.status;
}

export async function runLauncher({
  version,
  args = process.argv.slice(2),
  platform = process.platform,
  architecture = process.arch,
  env = process.env,
  cacheRoot,
  releaseBaseURL,
  fetchImpl,
  spawnImpl,
} = {}) {
  const executable = await ensureBinary({
    version,
    platform,
    architecture,
    env,
    ...(cacheRoot === undefined ? {} : { cacheRoot }),
    ...(releaseBaseURL === undefined ? {} : { releaseBaseURL }),
    ...(fetchImpl === undefined ? {} : { fetchImpl }),
  });
  return executeBinary(executable, args, {
    ...(spawnImpl === undefined ? {} : { spawnImpl }),
  });
}

function validateVersion(version) {
  if (typeof version !== "string" || !semverPattern.test(version)) {
    throw new Error(`invalid openapi-sdkgen package version: ${String(version)}`);
  }
}

async function hasValidCacheEntry(paths) {
  let expectedDigest;
  let binary;
  try {
    expectedDigest = (await readFile(paths.checksum, "utf8")).trim().toLowerCase();
    if (!/^[0-9a-f]{64}$/.test(expectedDigest)) return false;
    binary = await readFile(paths.executable);
  } catch (error) {
    if (error?.code === "ENOENT") return false;
    throw new Error(
      `cannot read openapi-sdkgen cache entry ${paths.directory}: ${error instanceof Error ? error.message : String(error)}`,
    );
  }
  return sha256(binary) === expectedDigest;
}

async function withCacheLock(paths, action) {
  const lockDirectory = join(paths.versionDirectory, `.${paths.target.key}.lock`);
  const deadline = Date.now() + cacheLockTimeoutMs;

  while (true) {
    try {
      await mkdir(lockDirectory, { mode: 0o700 });
      break;
    } catch (error) {
      if (error?.code !== "EEXIST") throw error;
    }

    if (await hasValidCacheEntry(paths)) {
      return paths.executable;
    }

    try {
      const lock = await stat(lockDirectory);
      if (Date.now() - lock.mtimeMs > staleCacheLockMs) {
        await rm(lockDirectory, { recursive: true, force: true });
        continue;
      }
    } catch (error) {
      if (error?.code === "ENOENT") continue;
      throw error;
    }

    if (Date.now() >= deadline) {
      throw new Error(`timed out waiting for openapi-sdkgen cache lock for ${paths.target.key}`);
    }
    await delay(50);
  }

  try {
    return await action();
  } finally {
    await rm(lockDirectory, { recursive: true, force: true });
  }
}

async function publishCacheEntry(paths, binaryBytes, digest) {
  const temporaryDirectory = join(
    paths.versionDirectory,
    `.${paths.target.key}.tmp-${process.pid}-${randomUUID()}`,
  );
  const temporaryExecutable = join(temporaryDirectory, paths.target.executable);
  const temporaryChecksum = join(temporaryDirectory, "sha256");

  await mkdir(temporaryDirectory, { mode: 0o700 });
  try {
    await writeFile(temporaryExecutable, binaryBytes, { mode: 0o600 });
    if (paths.target.os !== "windows") {
      await chmod(temporaryExecutable, 0o755);
    }
    await writeFile(temporaryChecksum, `${digest}\n`, { mode: 0o600 });
    await rename(temporaryDirectory, paths.directory);
  } finally {
    await rm(temporaryDirectory, { recursive: true, force: true });
  }
}

async function fetchBytes(url, { fetchImpl, maxBytes, label }) {
  let currentURL = new URL(url);
  for (let redirect = 0; redirect <= maxRedirects; redirect += 1) {
    if (currentURL.protocol !== "https:") {
      throw new Error(`${label} URL must use HTTPS: ${currentURL.origin}`);
    }
    let response;
    try {
      response = await fetchImpl(currentURL, {
        redirect: "manual",
        signal: AbortSignal.timeout(downloadTimeoutMs),
        headers: { "user-agent": "openapi-sdkgen-npm-launcher" },
      });
    } catch (error) {
      throw new Error(
        `failed to download ${label}: ${error instanceof Error ? error.message : String(error)}`,
      );
    }

    if (isRedirectStatus(response.status)) {
      if (redirect === maxRedirects) {
        throw new Error(`${label} exceeded ${maxRedirects} HTTPS redirects`);
      }
      const location = response.headers.get("location");
      if (location === null) {
        throw new Error(`${label} redirect is missing Location`);
      }
      currentURL = new URL(location, currentURL);
      continue;
    }
    if (!response.ok) {
      throw new Error(`failed to download ${label}: HTTP ${response.status}`);
    }

    const declaredLength = response.headers.get("content-length");
    if (
      declaredLength !== null &&
      /^\d+$/.test(declaredLength) &&
      Number(declaredLength) > maxBytes
    ) {
      throw new Error(`${label} exceeds the ${maxBytes}-byte download limit`);
    }
    if (response.body === null) {
      return new Uint8Array();
    }

    const reader = response.body.getReader();
    const chunks = [];
    let total = 0;
    while (true) {
      const next = await reader.read();
      if (next.done) break;
      if (next.value.byteLength > maxBytes - total) {
        throw new Error(`${label} exceeds the ${maxBytes}-byte download limit`);
      }
      total += next.value.byteLength;
      chunks.push(next.value);
    }
    const bytes = new Uint8Array(total);
    let offset = 0;
    for (const chunk of chunks) {
      bytes.set(chunk, offset);
      offset += chunk.byteLength;
    }
    return bytes;
  }
  throw new Error(`failed to download ${label}`);
}

function checksumForAsset(checksumBytes, asset) {
  let text;
  try {
    text = new TextDecoder("utf-8", { fatal: true }).decode(checksumBytes);
  } catch {
    throw new Error("release checksums are not valid UTF-8");
  }
  let digest;
  for (const line of text.split(/\r?\n/)) {
    if (line.trim() === "") continue;
    const match = line.match(/^([0-9A-Fa-f]{64})\s+(.+)$/);
    if (match === null) {
      throw new Error("release checksums contain a malformed entry");
    }
    if (match[2] !== asset) continue;
    if (digest !== undefined) {
      throw new Error(`release checksums contain duplicate entries for ${asset}`);
    }
    digest = match[1].toLowerCase();
  }
  if (digest === undefined) {
    throw new Error(`release checksums do not contain ${asset}`);
  }
  return digest;
}

function sha256(value) {
  return createHash("sha256").update(value).digest("hex");
}

function ensureTrailingSlash(value) {
  return value.endsWith("/") ? value : `${value}/`;
}

function isRedirectStatus(status) {
  return [301, 302, 303, 307, 308].includes(status);
}
