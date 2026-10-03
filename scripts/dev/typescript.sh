#!/usr/bin/env bash
source "$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)/scripts/lib/commands.sh"
set -- "${SCRIPT_ARGS[@]}"
command="${1:-help}"
shift || true
case "$command" in
  help|--help|-h)
    printf 'TypeScript commands: install, lock, fmt, fmt-check, lint\ninstall uses the network; lock/fmt modify files. Verification uses scripts/verification/typescript.sh. Versions follow .node-version and packageManager.\n' >&2
    exit 0 ;;
esac
if [[ "${1:-}" == --help ]]; then
  script_error "typescript.sh $command [tool arguments...]" 'install: network/node_modules; lock: lockfile; fmt: source; checks: temporary fixtures/reports.'
  exit 0
fi
case "$command" in
  install) run_step 'TypeScript dependencies' ts_pnpm install --frozen-lockfile "$@" ;;
  lock) run_step 'TypeScript lockfile' ts_pnpm install --lockfile-only "$@" ;;
  fmt|fmt-check)
    if [[ "$command" == fmt ]]; then task=fmt; option=(); else task=fmt:check; option=(--check); fi
    run_step 'TypeScript format' ts_pnpm run "$task" "$@"
    run_step 'Runtime format' ts_pnpm exec oxfmt "${option[@]}" "$ROOT/internal/target/typescript/runtime" ;;
  lint) run_step 'TypeScript lint' ts_pnpm run lint "$@" ;;
  *) script_error "unknown TypeScript command: $command"; exit 2 ;;
esac
