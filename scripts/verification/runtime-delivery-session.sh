#!/usr/bin/env bash
set -euo pipefail
source "$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)/scripts/lib/commands.sh"
set -- "${SCRIPT_ARGS[@]}"
if [[ "${1:-}" == --help || "${2:-}" == --help ]]; then
  printf '%s\n' 'runtime-delivery-session.sh [RUNTIME_REPORT]' 'Serves a prepared runtime session on a temporary loopback port until stopped. stdout is readiness JSON.' >&2
  exit 0
fi
require_system_node
run_live data 'runtime session server' node "$TYPESCRIPT_ROOT/verification/runtime-delivery-session.mjs" "$@"
