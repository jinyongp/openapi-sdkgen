#!/usr/bin/env bash
set -euo pipefail
source "$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)/scripts/lib/commands.sh"
set -- "${SCRIPT_ARGS[@]}"
cd "$ROOT"
if [[ "${1:-}" == --help || $# -ne 2 ]]; then
  echo "usage: inspect.sh CORPUS_DIRECTORY REPORT_JSON; writes an inspect measurement report" >&2
  [[ "${1:-}" == --help ]] && exit 0
  exit 2
fi
bash "$ROOT/scripts/dev/go.sh" build
require_system_node
run_step "inspect corpus benchmark" node scripts/inspect-benchmark.mjs "$@"
