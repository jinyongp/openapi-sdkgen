#!/usr/bin/env bash
set -euo pipefail

source "$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)/scripts/lib/commands.sh"
set -- "${SCRIPT_ARGS[@]}"
if [[ "${1:-}" == --help ]]; then
  printf '%s\n' 'check.sh: installs pinned TypeScript dependencies, then runs repository checks, cross-builds and example clients. Writes temporary artifacts; does not publish.' >&2
  exit 0
fi
bash "$ROOT/scripts/dev/typescript.sh" install
bash "$ROOT/scripts/dev/check.sh" "$@"
