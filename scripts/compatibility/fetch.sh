#!/usr/bin/env bash
set -euo pipefail
source "$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)/scripts/lib/commands.sh"
set -- "${SCRIPT_ARGS[@]}"

if [[ "${1:-}" == --help || "${2:-}" == --help ]]; then
  echo 'fetch.sh [fresh|offline] [MANIFEST] [CORPUS]; fresh downloads pinned inputs, offline verifies existing corpus' >&2
  exit 0
fi
mode=fresh
if [[ "${1:-}" == fresh || "${1:-}" == offline ]]; then mode="$1"; shift; fi
manifest="${1:-test/compatibility/holdout.json}"
corpus="${2:-.tmp/compatibility-holdout}"
mode="${3:-$mode}"
case "$mode" in
  fresh) extra=() ;;
  offline) extra=(--offline) ;;
  *)
    echo "compatibility corpus mode must be fresh or offline" >&2
    exit 2
    ;;
esac

binary="$LOG_DIR/benchmark"
run_step "build compatibility benchmark" go build -o "$binary" ./scripts/compat-benchmark
run_step "compatibility corpus $mode" "$binary"   --mode fetch   --manifest "$manifest"   --corpus-root "$corpus"   "${extra[@]}"
