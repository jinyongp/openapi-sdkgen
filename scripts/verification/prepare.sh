#!/usr/bin/env bash
set -euo pipefail
source "$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)/scripts/lib/commands.sh"
set -- "${SCRIPT_ARGS[@]}"
if [[ "${1:-}" == --help || "${2:-}" == --help ]]; then
  printf '%s\n' 'prepare.sh' 'Builds a generator and writes disposable conformance/declaration fixtures under test/typescript/fixtures/generated.' >&2
  exit 0
fi
generator="${SDKGEN_PREPARE_GENERATOR:-$ROOT/.tmp/bin/openapi-sdkgen}"
if [[ -z "${SDKGEN_PREPARE_GENERATOR:-}" ]]; then bash "$ROOT/scripts/dev/go.sh" build; fi
if [[ ! -x "$generator" ]]; then script_error "prepared generator is not executable: $generator"; exit 2; fi
run_step "generate catalog conformance fixtures" ts_node node \
  "$TYPESCRIPT_ROOT/verification/prepare-fixtures.mjs" "$generator"
run_step "TypeScript declaration option fixtures" ts_node node \
  "$TYPESCRIPT_ROOT/verification/prepare-declarations.mjs" "$generator"
