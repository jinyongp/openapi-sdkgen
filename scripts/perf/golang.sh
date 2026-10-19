#!/usr/bin/env bash
source "$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)/scripts/lib/commands.sh"
set -- "${SCRIPT_ARGS[@]}"
if [[ "${1:-}" == --help ]]; then
  echo 'golang.sh [micro [GO_BENCHMARK_ARGUMENTS...] | runtime [--output DIR] | generation [--output DIR] | builds [--output DIR] | platform [--output DIR] | providers [--output DIR] | features [--output DIR]]: optional repeated local measurements; no CI timing or size gate' >&2
  exit 0
fi
cd "$ROOT"
case "${1:-micro}" in
  runtime|generation|builds|platform|providers|features)
    run_data 'Go fixed-workload measurements' python3 "$ROOT/scripts/perf/golang/measure.py" "$@" ;;
  micro)
    if (($#)); then shift; fi
    run_data 'Go generation measurements' go test ./internal/target/golang -run '^$' -bench '^BenchmarkGoGeneration$' -benchmem -count=5 "$@" ;;
  -*) run_data 'Go generation measurements' go test ./internal/target/golang -run '^$' -bench '^BenchmarkGoGeneration$' -benchmem -count=5 "$@" ;;
  *) script_error "unknown Go performance mode: $1"; exit 2 ;;
esac
