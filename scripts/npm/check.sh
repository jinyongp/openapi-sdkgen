#!/usr/bin/env bash
set -euo pipefail

SCRIPT_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
ROOT="${SOURCE_ROOT:-$SCRIPT_ROOT}"
cd "$ROOT"

package_dir="${1:-${NPM_PACKAGE_DIR:-$ROOT/.tmp/npm-package}}"
if [[ ! -f "$package_dir/package.json" ||
  ! -f "$package_dir/bin/openapi-sdkgen.js" ||
  ! -f "$package_dir/lib/launcher.js" ]]; then
  echo "npm package directory is incomplete: $package_dir" >&2
  exit 2
fi

node "$SCRIPT_ROOT/scripts/npm/launcher-test.mjs" "$package_dir"

test_dir="${NPM_TEST_DIR:-$ROOT/.tmp/npm-package-install}"
rm -rf "$test_dir"
mkdir -p "$test_dir"
npm pack --ignore-scripts --pack-destination "$test_dir" "$package_dir"
tarball="$(find "$test_dir" -maxdepth 1 -name '*.tgz' -print -quit)"
if [[ -z "$tarball" ]]; then
  echo "npm package tarball was not created" >&2
  exit 1
fi

tarball_files="$test_dir/tarball-files.txt"
tar -tzf "$tarball" >"$tarball_files"
required_files=(
  package/package.json
  package/README.md
  package/LICENSE
  package/NOTICE
  package/bin/openapi-sdkgen.js
  package/lib/launcher.js
)
for required_file in "${required_files[@]}"; do
  if ! grep -Fqx "$required_file" "$tarball_files"; then
    echo "npm package tarball is missing required file: $required_file" >&2
    exit 1
  fi
done
if grep -Eq '^package/bin/(darwin|linux|windows)-' "$tarball_files"; then
  echo "npm package tarball contains bundled native platform binaries" >&2
  exit 1
fi

install_dir="$test_dir/install"
npm install --ignore-scripts --no-audit --no-fund --offline --prefix "$install_dir" "$tarball"
installed_package="$install_dir/node_modules/openapi-sdkgen"
installed_bin="$install_dir/node_modules/.bin/openapi-sdkgen"
if [[ ! -x "$installed_bin" ]]; then
  echo "installed npm launcher is missing: $installed_bin" >&2
  exit 1
fi
node "$SCRIPT_ROOT/scripts/npm/launcher-test.mjs" "$installed_package" "$installed_bin"

echo "ok npm package check"
