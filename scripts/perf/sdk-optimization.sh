#!/usr/bin/env bash
source "$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)/scripts/lib/commands.sh"
run_data 'SDK optimization preflight' python3 "$ROOT/scripts/perf/sdk-optimization/record.py" "${SCRIPT_ARGS[@]}"
