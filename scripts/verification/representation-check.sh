#!/usr/bin/env bash
set -euo pipefail
source "$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)/scripts/lib/commands.sh"
set -- "${SCRIPT_ARGS[@]}"
if [[ "${1:-}" == --help || "${2:-}" == --help ]]; then
  printf '%s\n' 'representation-check.sh --baseline BINARY --candidate BINARY [--fixtures IDS] [--pairs N] [--aa-pairs N] [--memory-pairs N]' 'Compares SDK behavior and measures costs; writes disposable SDKs/reports under .tmp/preimplementation. stdout is a JSON summary.' >&2
  exit 0
fi
run_data "representation-check" ts_node node "$TYPESCRIPT_ROOT/verification/preimplementation.mjs" "$@"
