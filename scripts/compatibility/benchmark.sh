#!/usr/bin/env bash
set -euo pipefail
source "$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)/scripts/lib/commands.sh"
set -- "${SCRIPT_ARGS[@]}"

if [[ "${1:-}" == --help || $# -gt 7 ]]; then
  echo "usage: $0 <manifest.json> <corpus-directory> <report.json> [typecheck-timeout] [document-id] [selection.toml] [metadata]" >&2
  [[ "${1:-}" == --help ]] && exit 0
  exit 2
fi

manifest="${1:-test/compatibility/holdout.json}"
corpus="${2:-.tmp/compatibility-holdout}"
output="${3:-.tmp/compatibility-benchmark.json}"
timeout="${4:-3m}"
document="${5:-}"
selection="${6:-}"
addon="${7:-}"
binary="$LOG_DIR/benchmark"
arguments=(
  --mode run
  --manifest "$manifest"
  --corpus-root "$corpus"
  --output "$output"
  --typescript-root "$TYPESCRIPT_ROOT"
  --typecheck-timeout "$timeout"
)
if [[ -n "$document" ]]; then
  arguments+=(--document "$document")
fi
if [[ -n "$selection" ]]; then
  arguments+=(--selection "$selection")
fi
if [[ -n "$addon" ]]; then
  arguments+=(--with "$addon")
fi

run_step "build compatibility benchmark" go build -o "$binary" ./scripts/compat-benchmark
require_system_node
run_step "compatibility benchmark" "$binary" "${arguments[@]}"
