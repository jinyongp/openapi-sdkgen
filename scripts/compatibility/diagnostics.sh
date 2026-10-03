#!/usr/bin/env bash
set -euo pipefail

source "$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)/scripts/lib/commands.sh"
set -- "${SCRIPT_ARGS[@]}"
cd "$ROOT"

if [[ "${1:-}" == --help || $# -ne 2 ]]; then
  echo "usage: $0 <manifest.json> <output.json>" >&2
  [[ "${1:-}" == --help ]] && exit 0
  exit 2
fi

run_step "diagnostic harvest" go run ./scripts/diagnostic-harvest --manifest "$1" --output "$2"
