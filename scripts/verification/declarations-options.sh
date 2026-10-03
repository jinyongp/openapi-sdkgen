#!/usr/bin/env bash
set -euo pipefail
source "$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)/scripts/lib/commands.sh"
set -- "${SCRIPT_ARGS[@]}"
if [[ "${1:-}" == --help || "${2:-}" == --help ]]; then
  printf '%s\n' 'declarations-options.sh' 'Uses .tmp/bin/openapi-sdkgen to write disposable declaration option fixtures.' >&2
  exit 0
fi
bash "$ROOT/scripts/dev/go.sh" build
run_step "TypeScript declaration options" ts_node node "$TYPESCRIPT_ROOT/verification/prepare-declarations.mjs" "$ROOT/.tmp/bin/openapi-sdkgen"
