#!/usr/bin/env bash
set -euo pipefail
source "$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)/scripts/lib/commands.sh"
set -- "${SCRIPT_ARGS[@]}"
if [[ "${1:-}" == --help || "${2:-}" == --help ]]; then
  printf '%s\n' 'typescript.sh [typecheck|test|coverage]' 'Prepares generated fixtures and checks the runtime/SDK; coverage writes optional reports.' >&2
  exit 0
fi

mode="${1:-test}"
case "$mode" in
  typecheck|test|coverage) ;;
  *)
    echo "usage: $0 [typecheck|test|coverage]" >&2
    exit 2
    ;;
esac

"$ROOT/scripts/verification/prepare.sh"
run_step "TypeScript runtime typecheck" ts_node node "$TYPESCRIPT_ROOT/node_modules/typescript/lib/tsc.js" --project "$ROOT/internal/target/typescript/runtime/tsconfig.json"
run_step "TypeScript typecheck" ts_pnpm run typecheck
if [[ "$mode" == typecheck ]]; then
  script_note "ok TypeScript typecheck"
  exit 0
fi
if [[ "$mode" == "coverage" ]]; then
  run_step "TypeScript coverage" ts_pnpm run coverage
else
  run_step "TypeScript test" ts_pnpm run test
fi
run_step "emitted provider native/declaration checks" ts_node node "$TYPESCRIPT_ROOT/verification/execution-providers.mjs"
run_step "selective client guide examples" ts_node node "$TYPESCRIPT_ROOT/verification/selection-docs.mjs"
