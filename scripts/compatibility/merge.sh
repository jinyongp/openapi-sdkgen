#!/usr/bin/env bash
set -euo pipefail
source "$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)/scripts/lib/commands.sh"
set -- "${SCRIPT_ARGS[@]}"

if [[ "${1:-}" == --help || $# -lt 3 ]]; then
  echo "usage: $0 <manifest.json> <report.json> <shard-report.json>..." >&2
  [[ "${1:-}" == --help ]] && exit 0
  exit 2
fi

manifest="$1"
output="$2"
shift 2
binary="$LOG_DIR/benchmark"
run_step "build compatibility benchmark" go build -o "$binary" ./scripts/compat-benchmark
run_step "merge compatibility reports" "$binary" \
  --mode merge --manifest "$manifest" --output "$output" "$@"
