#!/usr/bin/env bash
set -euo pipefail
source "$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)/scripts/lib/commands.sh"
set -- "${SCRIPT_ARGS[@]}"
if [[ "${1:-}" == --help || "${2:-}" == --help ]]; then
  printf '%s\n' 'perf-acceptance.sh' 'Runs warmed repeated measurements against the existing baseline thresholds. Writes .tmp/runs measurement logs.' >&2
  exit 0
fi
cd "$ROOT"

gate_dir="$LOG_DIR/measurement"
binary="$gate_dir/openapi-sdkgen.test"
stream_log="$gate_dir/stream-runtime.log"
mkdir -p "$gate_dir"

run_step "build performance acceptance binary" go test -c -o "$binary" ./cmd/openapi-sdkgen

OPENAPI_SDKGEN_PERF=1 run_step "warm process metrics" "$binary" -test.run '^TestPerformanceProcessMetrics$'
workloads=(self-contained templated-resource parameter-heavy external-reference high-artifact server-addon links-heavy)
benchmark_pattern='^Benchmark(Generation|ArtifactPublication|IncrementalGeneration)/'
run_step "warm benchmark" "$binary" -test.run '^$' -test.bench "$benchmark_pattern" -test.benchtime 1x -test.count 1

for index in 01 02 03 04 05; do
	run_data "performance measurement" "$binary" -test.run '^$' -test.bench "$benchmark_pattern" -test.benchmem -test.benchtime 1x -test.count 1 >"$gate_dir/run-$index-bench.log"
	for workload in "${workloads[@]}"; do
		OPENAPI_SDKGEN_PERF=1 OPENAPI_SDKGEN_PERF_WORKLOAD="$workload" run_data "performance measurement" "$binary" -test.v -test.run '^TestPerformanceProcessMetrics$' >"$gate_dir/run-$index-$workload-process.log"
	done
done

"$ROOT/scripts/verification/prepare.sh"
if ! OPENAPI_SDKGEN_STREAM_PERF=1 OPENAPI_SDKGEN_STREAM_PERF_EVIDENCE="$gate_dir/stream-framing.jsonl" run_data "stream performance" ts_pnpm exec vitest run tests/stream-performance.test.ts tests/stream-framing-performance.test.ts >"$stream_log"; then
  echo "failed TypeScript stream performance acceptance" >&2
  script_diagnostic "$stream_log"
  exit 1
fi
script_note "ok TypeScript stream performance acceptance"

OPENAPI_SDKGEN_PERF_GATE=1 \
OPENAPI_SDKGEN_PERF_GATE_LOG_DIR="$gate_dir" \
OPENAPI_SDKGEN_PERF_GATE_BASELINE="$ROOT/cmd/openapi-sdkgen/testdata/performance-baseline.json" \
run_step "performance acceptance" "$binary" -test.v -test.run '^TestPerformanceAcceptanceGate$'
