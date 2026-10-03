#!/usr/bin/env bash
set -euo pipefail

source "$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)/scripts/lib/commands.sh"
set -- "${SCRIPT_ARGS[@]}"
if [[ "${1:-}" == --help ]]; then
  echo 'source-check.sh: pack and inspect the source npm distribution; uses the pnpm cache/network and disposable output.' >&2
  exit 0
fi
require_system_node
PNPM_VERSION="12.4.1"
package_dir="$ROOT/npm"
node - "$package_dir/package.json" <<'NODE'
const fs = require("node:fs");
const [packagePath] = process.argv.slice(2);
const pkg = JSON.parse(fs.readFileSync(packagePath, "utf8"));

if (pkg.name !== "openapi-sdkgen") throw new Error("unexpected npm package name");
if (pkg.bin?.["openapi-sdkgen"] !== "./bin/openapi-sdkgen.js") {
  throw new Error("npm bin contract changed");
}
if (pkg.engines?.node !== ">=22") throw new Error("npm Node consumer floor changed");
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

temporary="$(mktemp -d "${TMPDIR:-/tmp}/openapi-sdkgen-npm-source-check.XXXXXX")"
trap 'rm -rf "$temporary"' EXIT
store="${npm_config_store_dir:-$ROOT/.tmp/pnpm-store}"
run_step 'pack npm source distribution' corepack "pnpm@$PNPM_VERSION" --dir "$package_dir" --config.store-dir="$store" pack --pack-destination "$temporary"

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

script_note "ok npm source distribution"
