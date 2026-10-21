#!/usr/bin/env bash
set -euo pipefail
source "$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)/scripts/lib/commands.sh"
set -- "${SCRIPT_ARGS[@]}"
if [[ "${1:-}" == --help ]]; then
  printf '%s\n' 'test.sh: checks release workflow dispatch arguments with a mocked GitHub CLI. Does not publish.' >&2
  exit 0
fi
if [[ "${SDKGEN_RELEASE_FIXTURE_CHILD:-0}" != 1 ]]; then
  run_step 'release workflow dispatch tests' env SDKGEN_RELEASE_FIXTURE_CHILD=1 bash "$ROOT/scripts/release/test.sh" "$@"
  run_step 'release CI reuse tests' python3 -B "$ROOT/.github/scripts/release_ci_test.py"
  exit 0
fi

test_root="$(mktemp -d "${TMPDIR:-/tmp}/sdkgen-release-dispatch.XXXXXX")"
trap 'rm -rf "$test_root"' EXIT
mkdir -p "$test_root/bin"
cat >"$test_root/bin/gh" <<'SH'
#!/usr/bin/env bash
printf '%s\n' "$@" >"${RELEASE_TEST_ARGS:?}"
exit "${RELEASE_TEST_EXIT:-0}"
SH
chmod +x "$test_root/bin/gh"
export PATH="$test_root/bin:$PATH" RELEASE_TEST_ARGS="$test_root/args"

assert_dispatch() {
  local ref="$1" mode="$2" prerelease="$3"
  shift 3
  bash "$ROOT/scripts/release/publish.sh" "$@"
  if [[ "$ref" != main ]]; then
    printf '%s\n' workflow run release.yml --ref "$ref" -f "tag=$ref" >"$test_root/expected"
  else
    printf '%s\n' workflow run release.yml --ref "$ref" -f "mode=$mode" -f "prerelease-id=$prerelease" >"$test_root/expected"
  fi
  diff -u "$test_root/expected" "$RELEASE_TEST_ARGS"
}

assert_dispatch main plan ""
assert_dispatch main plan "" --dry-run
assert_dispatch main release "" --yes
assert_dispatch main release rc --yes --prerelease rc
assert_dispatch v1.2.0 release "" --resume v1.2.0
assert_dispatch v1.3.0-rc.0 release "" --resume v1.3.0-rc.0

assert_rejected() {
  rm -f "$RELEASE_TEST_ARGS"
  if bash "$ROOT/scripts/release/publish.sh" "$@" >"$test_root/output" 2>&1; then
    script_error "Accepted invalid release arguments: $*"; exit 1
  fi
  [[ ! -e "$RELEASE_TEST_ARGS" ]] || { script_error 'Invalid arguments dispatched a workflow'; exit 1; }
}
assert_rejected --resume
assert_rejected --resume main
assert_rejected --prerelease
assert_rejected --dry-run --yes
assert_rejected --dry-run --resume v1.2.0
assert_rejected --resume v1.2.0 --prerelease rc
assert_rejected patch
assert_rejected v1.2.1
assert_rejected --since v1.2.0
if RELEASE_TEST_EXIT=1 bash "$ROOT/scripts/release/publish.sh" --yes >"$test_root/output" 2>&1; then
  script_error 'Ignored workflow dispatch failure'; exit 1
fi
script_note 'ok release dispatch arguments and failure propagation'
