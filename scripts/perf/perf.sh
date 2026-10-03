#!/usr/bin/env bash
set -euo pipefail
source "$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)/scripts/lib/commands.sh"
set -- "${SCRIPT_ARGS[@]}"
if [[ "${1:-}" == --help || "${2:-}" == --help ]]; then
  printf '%s\n' 'perf.sh' 'Measures real workloads and stream runtime behavior. Results are saved in .tmp/runs; --verbose displays measurements.' >&2
  exit 0
fi
cd "$ROOT"

perf_dir="$LOG_DIR/measurement"
binary="$perf_dir/openapi-sdkgen.test"
shape_log="$perf_dir/workloads.log"
bench_log="$perf_dir/bench.log"
metric_log="$perf_dir/process.log"
stream_log="$perf_dir/stream-runtime.log"
mkdir -p "$perf_dir"

run_step "build performance test binary" go test -c -o "$binary" ./cmd/openapi-sdkgen

if ! OPENAPI_SDKGEN_PERF=1 run_data "performance measurement" "$binary" -test.v -test.run '^TestPerformance(Workloads|PublicationWorkload)Deterministic$' >"$shape_log"; then
  echo "failed performance workload verification" >&2
  script_diagnostic "$shape_log"
  exit 1
fi

if ! run_data "performance measurement" "$binary" -test.run '^$' -test.bench '^Benchmark(Generation|ArtifactPublication|IncrementalGeneration)/' -test.benchmem -test.benchtime "${PERF_BENCHTIME:-1x}" -test.count "${PERF_COUNT:-1}" >"$bench_log"; then
  echo "failed performance benchmarks" >&2
  script_diagnostic "$bench_log"
  exit 1
fi

if ! OPENAPI_SDKGEN_PERF=1 run_data "performance measurement" "$binary" -test.v -test.run '^TestPerformanceProcessMetrics$' >"$metric_log"; then
  echo "failed performance process metrics" >&2
  script_diagnostic "$metric_log"
  exit 1
fi

"$ROOT/scripts/verification/prepare.sh"
if ! OPENAPI_SDKGEN_STREAM_PERF=1 run_data "stream performance" ts_pnpm exec vitest run tests/stream-performance.test.ts >"$stream_log"; then
  echo "failed TypeScript stream performance acceptance" >&2
  script_diagnostic "$stream_log"
  exit 1
fi

if [[ "$SCRIPT_VERBOSE" == 1 ]]; then cat "$shape_log" "$bench_log" "$metric_log" >&2; fi
script_note "performance results: $perf_dir"
script_note "ok performance baseline"
