#!/usr/bin/env bash
set -euo pipefail
source "$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)/scripts/lib/commands.sh"
set -- "${SCRIPT_ARGS[@]}"
if [[ "${1:-}" == --help || "${2:-}" == --help ]]; then
  printf '%s\n' 'representation-measure.sh --report REPORT --artifacts REPORT [--pairs N]' 'Measures prepared SDKs; writes a report. stdout is a JSON summary.' >&2
  exit 0
fi
run_data "representation measurement" ts_node node "$TYPESCRIPT_ROOT/verification/measure.mjs" "$@"
