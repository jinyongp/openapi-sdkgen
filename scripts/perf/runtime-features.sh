#!/usr/bin/env bash
set -euo pipefail
source "$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)/scripts/lib/commands.sh"
set -- "${SCRIPT_ARGS[@]}"
if [[ "${1:-}" == --help ]]; then
  printf '%s\n' 'runtime-features.sh [OUTPUT]' 'Generates and verifies 73 feature SDKs, then measures a three-operation browser SDK. Default output: .tmp/runtime-feature-matrix.' >&2
  exit 0
fi
cd "$ROOT"
matrix_output="${1:-$ROOT/.tmp/runtime-feature-matrix}"
if [[ "$matrix_output" != /* ]]; then matrix_output="$ROOT/$matrix_output"; fi
SDKGEN_RUNTIME_MATRIX_DIR="$matrix_output" run_step 'runtime feature native matrix' go test ./internal/target/typescript -run '^TestRuntimeFeatureNativeMatrixRegression$' -count=1
run_step 'runtime feature browser measurement' ts_node node "$TYPESCRIPT_ROOT/verification/runtime-feature-measure.mjs" "$matrix_output"
script_note "ok runtime feature measurement: $matrix_output/size-results.json"
