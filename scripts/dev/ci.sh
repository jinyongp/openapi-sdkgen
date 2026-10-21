#!/usr/bin/env bash
# Independent CI suites also serve the local aggregate command.
source "$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)/scripts/lib/commands.sh"
set -- "${SCRIPT_ARGS[@]}"
cd "$ROOT"
suite="${1:-all}"
case "$suite" in
  --help|help)
    echo 'ci.sh [all|go|go-core|go-consumers SHARD COUNT|typescript-generator|typescript-runtime|typescript-matrix|typescript-tools|compiler VERSION [all|consumers|declarations]]: independent checks. all runs suites sequentially.' >&2
    exit 0 ;;
  go)
    bash "$ROOT/scripts/dev/ci.sh" go-core
    bash "$ROOT/scripts/dev/ci.sh" go-consumers 1 1
    bash "$ROOT/scripts/dev/ci.sh" typescript-generator ;;
  go-core)
    for check in fmt-check vet mod-tidy-check mod-verify; do bash "$ROOT/scripts/dev/go.sh" "$check"; done
    run_step 'Go core tests' python3 "$ROOT/scripts/dev/go_ci.py" core
    run_step 'Go CI partition tests' python3 -B -m unittest discover -s "$ROOT/scripts/dev" -p '*_test.py'
    bash "$ROOT/scripts/generate/go-check-test.sh" ;;
  go-consumers) run_step 'generated Go consumer tests' python3 "$ROOT/scripts/dev/go_ci.py" golang "${2:-1}" "${3:-1}" ;;
  typescript-generator) run_step 'TypeScript generator tests' python3 "$ROOT/scripts/dev/go_ci.py" typescript ;;
  typescript-runtime)
    bash "$ROOT/scripts/verification/typescript.sh" test ;;
  typescript-matrix)
    run_step 'TypeScript native matrix' python3 "$ROOT/scripts/dev/go_ci.py" native ;;
  typescript-tools)
    bash "$ROOT/scripts/dev/typescript.sh" fmt-check
    bash "$ROOT/scripts/dev/typescript.sh" lint
    bash "$ROOT/scripts/verification/declarations-test.sh"
    run_step 'verification tool tests' ts_node node --test "$TYPESCRIPT_ROOT/verification/verification.test.mjs" "$TYPESCRIPT_ROOT/verification/sdk-delivery-runner.test.mjs" "$TYPESCRIPT_ROOT/verification/runtime-boundaries.test.mjs" "$TYPESCRIPT_ROOT/verification/provider-size-measure.test.mjs"
    bash "$ROOT/scripts/generate/check-test.sh"
    bash "$ROOT/scripts/npm/source-check.sh"
    bash "$ROOT/scripts/release/test.sh" ;;
  compiler) bash "$ROOT/scripts/verification/compatibility.sh" "${2:-all}" "${3:-all}" ;;
  all)
    for item in go typescript-runtime typescript-matrix typescript-tools compiler; do bash "$ROOT/scripts/dev/ci.sh" "$item"; done ;;
  *) script_error "unknown CI suite: $suite"; exit 2 ;;
esac
script_note "ok CI $suite"
