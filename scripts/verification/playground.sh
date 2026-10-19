#!/usr/bin/env bash
source "$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)/scripts/lib/commands.sh"
set -- "${SCRIPT_ARGS[@]}"
if [[ "${1:-}" == --help ]]; then
  echo 'playground.sh: builds the WASM generator, checks both source targets and ZIP contents; executes no generated consumer' >&2
  exit 0
fi
run_step 'Playground WASM generation and ZIP' corepack pnpm --dir "$ROOT/docs" run playground:test
