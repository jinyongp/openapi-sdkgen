#!/usr/bin/env bash
set -euo pipefail
source "$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)/scripts/lib/commands.sh"
set -- "${SCRIPT_ARGS[@]}"
if [[ "${1:-}" == --help || "${2:-}" == --help ]]; then
  printf '%s\n' 'declarations-test.sh' 'Runs declaration checker behavior tests using disposable sources.' >&2
  exit 0
fi
run_step "TypeScript declaration checker self-tests" ts_node node --test "$TYPESCRIPT_ROOT/verification/declarations.test.mjs"
