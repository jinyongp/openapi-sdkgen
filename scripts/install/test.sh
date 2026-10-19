#!/usr/bin/env bash
set -euo pipefail
source "$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)/scripts/lib/commands.sh"
set -- "${SCRIPT_ARGS[@]}"
if [[ "${1:-}" == --help ]]; then
  printf '%s\n' 'test.sh: tests POSIX installers with release download fixtures; PowerShell installer tests run in Windows CI.' >&2
  exit 0
fi
run_step 'POSIX installer fixtures' node --test "$ROOT/scripts/install/install.test.mjs"
