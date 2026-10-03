#!/usr/bin/env bash
# Independent CI suites also serve the local aggregate command.
source "$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)/scripts/lib/commands.sh"
set -- "${SCRIPT_ARGS[@]}"
suite="${1:-all}"
case "$suite" in
  --help|help)
    echo 'ci.sh [all|go|typescript-runtime|typescript-tools|compiler VERSION]: independent checks. Installs no tools; writes disposable build/test artifacts. all runs suites sequentially.' >&2
    exit 0 ;;
  go)
    for check in fmt-check vet mod-tidy-check mod-verify test; do bash "$ROOT/scripts/dev/go.sh" "$check"; done ;;
  typescript-runtime) bash "$ROOT/scripts/verification/typescript.sh" test ;;
  typescript-tools)
    bash "$ROOT/scripts/dev/typescript.sh" fmt-check
    bash "$ROOT/scripts/dev/typescript.sh" lint
    bash "$ROOT/scripts/verification/declarations-test.sh"
    run_step 'verification tool tests' ts_node node --test "$TYPESCRIPT_ROOT/verification/verification.test.mjs" "$TYPESCRIPT_ROOT/verification/sdk-delivery-runner.test.mjs"
    bash "$ROOT/scripts/generate/check-test.sh"
    bash "$ROOT/scripts/npm/source-check.sh"
    bash "$ROOT/scripts/release/test.sh" ;;
  compiler) bash "$ROOT/scripts/verification/compatibility.sh" "${2:-all}" ;;
  all)
    for item in go typescript-runtime typescript-tools compiler; do bash "$ROOT/scripts/dev/ci.sh" "$item"; done ;;
  *) script_error "unknown CI suite: $suite"; exit 2 ;;
esac
script_note "ok CI $suite"
