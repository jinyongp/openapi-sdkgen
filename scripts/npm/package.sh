#!/usr/bin/env bash
set -euo pipefail

SCRIPT_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
ROOT="${SOURCE_ROOT:-$SCRIPT_ROOT}"
cd "$ROOT"

if [[ $# -ne 1 ]]; then
  echo "usage: scripts/npm/package.sh <semver>" >&2
  exit 2
fi

version="$1"
semver='^(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)(-([0-9A-Za-z-]+)(\.[0-9A-Za-z-]+)*)?(\+([0-9A-Za-z-]+)(\.[0-9A-Za-z-]+)*)?$'
if ! [[ "$version" =~ $semver ]]; then
  echo "npm package version must be SemVer without a leading v: $version" >&2
  exit 2
fi

package_dir="${NPM_PACKAGE_DIR:-$ROOT/.tmp/npm-package}"
rm -rf "$package_dir"
mkdir -p "$package_dir"
cp -R "$ROOT/npm/." "$package_dir/"
cp "$ROOT/LICENSE" "$package_dir/LICENSE"
cp "$ROOT/NOTICE" "$package_dir/NOTICE"

sed -i.bak "s/\"version\": \"0.0.0\"/\"version\": \"$version\"/" "$package_dir/package.json"
rm "$package_dir/package.json.bak"

if find "$package_dir/bin" -mindepth 2 -type f -print -quit | grep -q .; then
  echo "npm package source unexpectedly contains bundled native binaries" >&2
  exit 1
fi

printf 'ok npm package %s\n' "$version"
