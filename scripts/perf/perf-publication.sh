#!/usr/bin/env bash
set -euo pipefail
source "$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)/scripts/lib/commands.sh"
set -- "${SCRIPT_ARGS[@]}"
if [[ "${1:-}" == --help || "${2:-}" == --help ]]; then
  printf '%s\n' 'perf-publication.sh [BENCHTIME=3x] [COUNT=5]' 'Measures artifact publication; logs are saved under .tmp/runs.' >&2
  exit 0
fi
cd "$ROOT"

benchtime="${1:-3x}"
count="${2:-5}"
if ! [[ "$count" =~ ^[1-9][0-9]*$ ]]; then
  script_error "performance publication count must be a positive integer: $count"
  exit 2
fi

perf_dir="$LOG_DIR/measurement"
binary="$perf_dir/openapi-sdkgen.test"
log="$perf_dir/bench.log"
mkdir -p "$perf_dir"

run_step "build publication performance test binary" go test -c -o "$binary" ./cmd/openapi-sdkgen
if ! run_data 'publication benchmark' "$binary" \
  -test.run '^$' \
  -test.bench '^BenchmarkArtifactPublication/(fresh-1x|incremental-noop-1x|incremental-one-change-1x|incremental-stale-1x)$' \
  -test.benchmem \
  -test.benchtime "$benchtime" \
  -test.count "$count" >"$log"; then
  echo "failed publication performance benchmarks" >&2
  script_diagnostic "$log"
  exit 1
fi

if [[ "$SCRIPT_VERBOSE" == 1 ]]; then cat "$log" >&2; fi
script_note "publication results: $log"
script_note "ok publication performance benchmark"
