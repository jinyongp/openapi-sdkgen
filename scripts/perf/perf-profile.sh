#!/usr/bin/env bash
set -euo pipefail
source "$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)/scripts/lib/commands.sh"
set -- "${SCRIPT_ARGS[@]}"
if [[ "${1:-}" == --help || "${2:-}" == --help ]]; then
  printf '%s\n' 'perf-profile.sh [WORKLOAD=self-contained] [PHASE=emit]' 'Builds a performance test binary and records CPU/heap profiles under .tmp/runs; --verbose shows profile summaries.' >&2
  exit 0
fi
cd "$ROOT"

profile_dir="$LOG_DIR/profiles"
binary="$profile_dir/openapi-sdkgen.test"
cpu_profile="$profile_dir/cpu.pprof"
heap_profile="$profile_dir/heap.pprof"
profile_log="$profile_dir/profile.log"
workload="${1:-${PERF_PROFILE_WORKLOAD:-self-contained}}"
phase="${2:-${PERF_PROFILE_PHASE:-emit}}"
mkdir -p "$profile_dir"

run_step "build performance profile binary" go test -c -o "$binary" ./cmd/openapi-sdkgen
if ! run_data "performance measurement" "$binary" -test.run '^$' -test.bench "^BenchmarkGeneration/$workload/$phase$" -test.benchtime 1x -test.count 1 -test.cpuprofile "$cpu_profile" -test.memprofile "$heap_profile" >"$profile_log"; then
  echo "failed performance profile" >&2
  script_diagnostic "$profile_log"
  exit 1
fi

if ! rg -q '^BenchmarkGeneration/' "$profile_log"; then
  script_error "no benchmark matched workload=$workload phase=$phase"
  exit 2
fi

run_step 'CPU profile summary' go tool pprof -top -nodecount=12 "$cpu_profile"
run_step 'heap profile summary' go tool pprof -top -alloc_space -nodecount=12 "$heap_profile"
script_note "profiles: $profile_dir"
script_note "ok performance profiles"
