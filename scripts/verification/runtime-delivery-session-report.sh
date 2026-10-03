#!/usr/bin/env bash
set -euo pipefail
source "$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)/scripts/lib/commands.sh"
set -- "${SCRIPT_ARGS[@]}"
if [[ "${1:-}" == --help || "${2:-}" == --help ]]; then
  printf '%s\n' 'runtime-delivery-session-report.sh [OBSERVED_SUMMARY_JSON] [SESSION_MANIFEST]' 'Reads observed session results and writes a report. stdout is JSON.' >&2
  exit 0
fi
run_data "runtime session report" ts_node node "$TYPESCRIPT_ROOT/verification/runtime-delivery-session-report.mjs" "$@"
