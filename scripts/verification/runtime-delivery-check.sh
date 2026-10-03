#!/usr/bin/env bash
set -euo pipefail
source "$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)/scripts/lib/commands.sh"
set -- "${SCRIPT_ARGS[@]}"
if [[ "${1:-}" == --help || "${2:-}" == --help ]]; then
  printf '%s\n' 'runtime-delivery-check.sh [BASELINE_COMMIT]' 'Builds and compares runtime delivery against the selected revision; writes .tmp/runtime-delivery reports.' >&2
  exit 0
fi
run_step "runtime delivery comparison" ts_node node "$TYPESCRIPT_ROOT/verification/runtime-delivery.mjs" "$@"
