#!/usr/bin/env bash
set -euo pipefail
source "$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)/scripts/lib/commands.sh"
set -- "${SCRIPT_ARGS[@]}"
if [[ "${1:-}" == --help || "${2:-}" == --help ]]; then
  printf '%s\n' 'representation-artifacts.sh --report REPORT [--fixtures IDS] [--pairs N]' 'Checks compiled/bundled/declaration artifacts from a source report; writes artifacts and a report. stdout is JSON.' >&2
  exit 0
fi
run_data "representation-artifacts" ts_node node "$TYPESCRIPT_ROOT/verification/artifacts.mjs" "$@"
