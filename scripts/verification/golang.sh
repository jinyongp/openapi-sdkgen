#!/usr/bin/env bash
source "$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)/scripts/lib/commands.sh"
set -- "${SCRIPT_ARGS[@]}"
mode="${1:-test}"
if (($# > 0)); then shift; fi
cd "$ROOT"
case "$mode" in
  test) run_step 'Go target tests' go test ./internal/target/golang "$@" ;;
  race) run_step 'Go target race' go test -race ./internal/target/golang "$@" ;;
  coverage)
    mkdir -p "$ROOT/.tmp/golang"
    run_step 'Go target coverage' go test -covermode=atomic -coverprofile="$ROOT/.tmp/golang/coverage.out" ./internal/target/golang "$@"
    script_note 'coverage: .tmp/golang/coverage.out' ;;
  --help|help) echo 'golang.sh [test|race|coverage] [GO_TEST_ARGUMENTS...]: Go target suite; generated consumers include their own race/lifetime checks. Coverage is optional.' >&2 ;;
  *) script_error "unknown Go verification mode: $mode"; exit 2 ;;
esac
