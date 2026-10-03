#!/usr/bin/env bash
set -euo pipefail
source "$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)/scripts/lib/commands.sh"
set -- "${SCRIPT_ARGS[@]}"
if [[ "${1:-}" == --help || "${2:-}" == --help ]]; then
  printf '%s\n' 'declarations.sh [--runtime DIR|--generated DIR|--files LIST_JSON] [--json REPORT_JSON]' 'Checks declarations. Reads the selected sources; optional --json writes the report.' >&2
  exit 0
fi
if [[ "$#" == 0 ]]; then
  set -- --runtime "$ROOT/internal/target/typescript/runtime"
fi
run_step "TypeScript declarations" ts_node node "$TYPESCRIPT_ROOT/verification/declarations.mjs" "$@"
