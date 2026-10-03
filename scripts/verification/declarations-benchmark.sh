#!/usr/bin/env bash
set -euo pipefail
source "$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)/scripts/lib/commands.sh"
set -- "${SCRIPT_ARGS[@]}"
if [[ "${1:-}" == --help || "${2:-}" == --help ]]; then
  printf '%s\n' 'declarations-benchmark.sh [REPORT_JSON]' 'Builds the generator, prepares fixtures, measures declaration checking and writes the report.' >&2
  exit 0
fi
report="${1:-$ROOT/.tmp/declaration-benchmark/report.json}"
[[ "$report" == /* ]] || report="$ROOT/$report"
mkdir -p "$(dirname "$report")"
build_started="$(date +%s%N)"
bash "$ROOT/scripts/dev/go.sh" build
export SDKGEN_DECLARATION_BUILD_MILLIS="$(( ($(date +%s%N) - build_started) / 1000000 ))"
SDKGEN_PREPARE_GENERATOR="$ROOT/.tmp/bin/openapi-sdkgen" bash "$ROOT/scripts/verification/prepare.sh"
run_step "TypeScript declaration benchmark" ts_node node \
  "$TYPESCRIPT_ROOT/verification/declarations-benchmark.mjs" "$ROOT/.tmp/bin/openapi-sdkgen" "$report"
