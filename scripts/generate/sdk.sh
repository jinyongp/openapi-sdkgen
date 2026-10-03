#!/usr/bin/env bash
set -euo pipefail
source "$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)/scripts/lib/commands.sh"
set -- "${SCRIPT_ARGS[@]}"
if [[ "${1:-}" == --help || $# -lt 2 || $# -gt 3 ]]; then
  echo "usage: sdk.sh INPUT OUTPUT [TARGET=typescript]; writes the generated SDK to OUTPUT" >&2
  [[ "${1:-}" == --help ]] && exit 0
  exit 2
fi
cd "$ROOT"
run_step "generate SDK" go run ./cmd/openapi-sdkgen generate --input "$1" --output "$2" --target "${3:-typescript}"
