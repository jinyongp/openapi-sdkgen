#!/usr/bin/env bash
source "$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)/scripts/lib/commands.sh"
set -- "${SCRIPT_ARGS[@]}"
command="${1:-help}"
shift || true
if [[ "$command" == help || "$command" == --help || "${1:-}" == --help ]]; then
  printf 'Documentation commands: install, lock, validate, dev, build, preview\ninstall/lock use the network; lock updates the lockfile. validate/build write documentation assets. dev/preview serve on DOCS_PORT.\n' >&2
  exit 0
fi
require_system_node
pnpm_docs() { corepack pnpm --dir "$ROOT/docs" "$@"; }
case "$command" in
  install) run_step 'Documentation dependencies' pnpm_docs install --frozen-lockfile "$@" ;;
  lock) run_step 'Documentation lockfile' pnpm_docs install --lockfile-only "$@" ;;
  validate) run_step 'Documentation validation' pnpm_docs run docs:validate "$@" ;;
  build) run_step 'Documentation build' pnpm_docs run docs:build "$@" ;;
  dev|preview)
    run_live diagnostic "documentation $command" corepack pnpm --dir "$ROOT/docs" run "docs:$command" --host 127.0.0.1 --port "${DOCS_PORT:-4173}" --strictPort "$@" ;;
  *) script_error "unknown documentation command: $command"; exit 2 ;;
esac
