#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
PNPM_VERSION="12.4.1"
package_dir="$ROOT/npm"
config="$ROOT/.github/npm/packages.yml"

node - "$ROOT/package.json" "$package_dir/package.json" <<'NODE'
const fs = require("node:fs");
const [rootPath, packagePath] = process.argv.slice(2);
const root = JSON.parse(fs.readFileSync(rootPath, "utf8"));
const pkg = JSON.parse(fs.readFileSync(packagePath, "utf8"));

if (root.private !== true) throw new Error("root package must be private");
if (root.packageManager !== "pnpm@12.4.1") throw new Error("root packageManager must stay pinned");
if (!Array.isArray(root.workspaces) || !root.workspaces.includes("npm")) {
  throw new Error("root workspaces must include npm");
}
if (pkg.name !== "openapi-sdkgen") throw new Error("unexpected npm package name");
if (pkg.version !== "0.0.0") throw new Error("source npm version must remain a template");
if (pkg.bin?.["openapi-sdkgen"] !== "./bin/openapi-sdkgen.js") {
  throw new Error("npm bin contract changed");
}
if (pkg.engines?.node !== ">=22") throw new Error("npm Node consumer floor changed");
if (JSON.stringify(pkg.files) !== JSON.stringify(["NOTICE"])) {
  throw new Error("source package must leave launcher/runtime injection to Releaseway");
}
NODE

cmp -s "$ROOT/LICENSE" "$package_dir/LICENSE" || {
  echo "npm/LICENSE must match root LICENSE" >&2
  exit 1
}
cmp -s "$ROOT/NOTICE" "$package_dir/NOTICE" || {
  echo "npm/NOTICE must match root NOTICE" >&2
  exit 1
}

for forbidden in   "$package_dir/bin/openapi-sdkgen.js"   "$package_dir/lib/launcher.js"   "$package_dir/.releaseway/native.json"   "$package_dir/.releaseway/runtime.cjs"; do
  if [[ -e "$forbidden" ]]; then
    echo "Releaseway-owned npm path must not exist in source: $forbidden" >&2
    exit 1
  fi
done

required_config=(
  "source: git-tag"
  "prefix: v"
  "prerelease-tag: next"
  "mode: direct"
  "cache-env: OPENAPI_SDKGEN_CACHE_DIR"
  "openapi-sdkgen_{version}_darwin_arm64.tar.gz"
  "openapi-sdkgen_{version}_darwin_amd64.tar.gz"
  "openapi-sdkgen_{version}_linux_arm64.tar.gz"
  "openapi-sdkgen_{version}_linux_amd64.tar.gz"
  "openapi-sdkgen_{version}_windows_arm64.zip"
  "openapi-sdkgen_{version}_windows_amd64.zip"
  "linux-arm64-musl:"
  "linux-x64-musl:"
)
for text in "${required_config[@]}"; do
  if ! grep -Fq -- "$text" "$config"; then
    echo "npm Releaseway config is missing: $text" >&2
    exit 1
  fi
done

temporary="$(mktemp -d "${TMPDIR:-/tmp}/openapi-sdkgen-npm-source-check.XXXXXX")"
trap 'rm -rf "$temporary"' EXIT
store="${npm_config_store_dir:-$ROOT/.tmp/pnpm-store}"
corepack "pnpm@$PNPM_VERSION" --dir "$package_dir" --config.store-dir="$store" pack --pack-destination "$temporary" >/dev/null

tarball="$(find "$temporary" -maxdepth 1 -name '*.tgz' -print -quit)"
if [[ -z "$tarball" ]]; then
  echo "npm source package did not pack" >&2
  exit 1
fi
inventory="$temporary/inventory.txt"
tar -tzf "$tarball" >"$inventory"
for required in package/package.json package/README.md package/LICENSE package/NOTICE; do
  grep -Fqx "$required" "$inventory" || {
    echo "npm source tarball is missing $required" >&2
    exit 1
  }
done
for forbidden in package/bin/openapi-sdkgen.js package/.releaseway/native.json package/.releaseway/runtime.cjs; do
  if grep -Fqx "$forbidden" "$inventory"; then
    echo "npm source tarball unexpectedly owns Releaseway path $forbidden" >&2
    exit 1
  fi
done

echo "ok npm Releaseway source contract"
