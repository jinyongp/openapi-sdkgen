#!/usr/bin/env bash
set -euo pipefail
source "$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)/scripts/lib/commands.sh"
set -- "${SCRIPT_ARGS[@]}"
if [[ "${1:-}" == --help || "${2:-}" == --help ]]; then
  printf '%s\n' 'sdk-delivery-browser.sh V1_REPORT V2_REPORT' 'Serves prepared SDK delivery cases on temporary loopback ports until stopped. stdout is readiness JSON.' >&2
  exit 0
fi
require_system_node
run_live data 'SDK browser server' node "$TYPESCRIPT_ROOT/verification/sdk-delivery-browser.mjs" "$@"
