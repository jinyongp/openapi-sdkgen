#!/usr/bin/env bash
set -euo pipefail
source "$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)/scripts/lib/commands.sh"
set -- "${SCRIPT_ARGS[@]}"
if [[ "${1:-}" == --help || $# -lt 2 ]]; then
  echo "usage: sdk.sh INPUT OUTPUT [TARGET=typescript] [-- GENERATOR_OPTIONS...]; writes the generated SDK to OUTPUT" >&2
  [[ "${1:-}" == --help ]] && exit 0
  exit 2
fi
input="$1"
output="$2"
shift 2
target="typescript"
if (($# > 0)) && [[ "$1" != -- ]]; then target="$1"; shift; fi
if (($# > 0)); then
  if [[ "$1" != -- ]]; then script_error 'generator options must follow --'; exit 2; fi
  shift
fi
for argument in "$@"; do
  case "$argument" in
    --input|-input|--input=*|-input=*|--output|-output|--output=*|-output=*|--target|-target|--target=*|-target=*) script_error "sdk generation owns $argument"; exit 2 ;;
  esac
done
cd "$ROOT"
run_step "generate SDK" go run ./cmd/openapi-sdkgen generate --input "$input" --output "$output" --target "$target" "$@"
