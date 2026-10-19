#!/usr/bin/env bash
set -euo pipefail
source "$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)/scripts/lib/commands.sh"
set -- "${SCRIPT_ARGS[@]}"
if [[ "${1:-}" == --help ]]; then
  printf '%s\n' 'test.sh: tests POSIX installers and deployed-site verification with download fixtures; PowerShell installer tests run in Windows CI.' >&2
  exit 0
fi
run_step 'POSIX installer and site fixtures' node --test "$ROOT/scripts/install/install.test.mjs" "$ROOT/scripts/install/verify-site.test.mjs"
