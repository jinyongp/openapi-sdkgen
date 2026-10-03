#!/usr/bin/env bash
set -euo pipefail
source "$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)/scripts/lib/commands.sh"
set -- "${SCRIPT_ARGS[@]}"
if [[ "${1:-}" == --help || "${2:-}" == --help ]]; then
  printf '%s\n' 'runtime-delivery-browser-report.sh [OBSERVED_SUMMARY_JSON]' 'Reads browser observations and writes a report. stdout is JSON.' >&2
  exit 0
fi
run_data "runtime browser report" ts_node node "$TYPESCRIPT_ROOT/verification/runtime-delivery-browser-report.mjs" "$@"
