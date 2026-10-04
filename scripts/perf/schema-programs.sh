#!/usr/bin/env bash
set -euo pipefail
source "$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)/scripts/lib/commands.sh"
set -- "${SCRIPT_ARGS[@]}"
if [[ "${1:-}" == --help ]]; then
  printf '%s\n' 'schema-programs.sh [OUTPUT] [BASELINE_SOURCE]' 'Measures 1000 APIs and 10 selected APIs: generation, sharing, strict source, declarations and executed browser bundles. An optional captured source tree supplies the before generator. No size thresholds.' >&2
  exit 0
fi
schema_output="${1:-$ROOT/.tmp/schema-programs-measurement}"
if [[ "$schema_output" != /* ]]; then schema_output="$ROOT/$schema_output"; fi
mkdir -p "$schema_output"
run_step 'schema program measurement generator' go build -o "$schema_output/current-generator" "$ROOT/cmd/openapi-sdkgen"
schema_baseline=""
if [[ -n "${2:-}" ]]; then
  schema_baseline="$schema_output/before-generator"
  run_step 'captured source baseline generator' go -C "$2" build -o "$schema_baseline" ./cmd/openapi-sdkgen
fi
run_step 'schema program sharing and delivery measurement' ts_node node "$TYPESCRIPT_ROOT/verification/schema-programs-measure.mjs" "$schema_output" "$schema_baseline"
script_note "ok schema program measurement: $schema_output/results.json"
