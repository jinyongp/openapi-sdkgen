#!/usr/bin/env bash
set -euo pipefail
source "$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)/scripts/lib/commands.sh"
set -- "${SCRIPT_ARGS[@]}"
if [[ "${1:-}" == --help ]]; then
  printf '%s\n' 'build.sh VERSION OUTPUT: stamps standalone installers for one release; does not publish.' >&2
  exit 0
fi
[[ $# == 2 && "$1" =~ ^[0-9]+\.[0-9]+\.[0-9]+(-[0-9A-Za-z]+([.-][0-9A-Za-z]+)*)?$ ]] || { script_error 'Expected VERSION OUTPUT'; exit 2; }
mkdir -p "$2"
for name in install.sh install.ps1; do
  sed "s/@VERSION@/$1/g" "$ROOT/scripts/install/$name" > "$2/$name"
done
script_note "ok installers for $1"
