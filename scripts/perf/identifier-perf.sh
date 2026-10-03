#!/usr/bin/env bash
set -euo pipefail
source "$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)/scripts/lib/commands.sh"
set -- "${SCRIPT_ARGS[@]}"
if [[ "${1:-}" == --help || "${2:-}" == --help ]]; then
  printf '%s\n' 'identifier-perf.sh' 'Measures local/aggregate identifier allocation; saves measurements under .tmp/runs.' >&2
  exit 0
fi
cd "$ROOT"

# Keep the allocator's scale/allocation probe separate from full CLI metrics.
# No generated output, external corpus or production identity is modified.
perf_dir="$LOG_DIR/measurement"
mkdir -p "$perf_dir"
log="$(mktemp "$perf_dir/local-identifiers-XXXXXX.log")"
script_note "identifier benchmark results: $log"
if ! run_data "identifier benchmark" go test ./internal/target/typescript -run '^$' -bench '^Benchmark(Local|Aggregate)IdentifierPlan/' -benchmem -benchtime=3x -count=5 >"$log"; then
  echo "failed local and aggregate identifier benchmarks" >&2
  script_diagnostic "$log"
  exit 1
fi
if [[ "$SCRIPT_VERBOSE" == 1 ]]; then cat "$log" >&2; fi
script_note "ok local and aggregate identifier benchmarks"
