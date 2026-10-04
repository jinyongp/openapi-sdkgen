#!/usr/bin/env bash
set -euo pipefail
source "$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)/scripts/lib/commands.sh"
set -- "${SCRIPT_ARGS[@]}"
if [[ "${1:-}" == --help || "${2:-}" == --help ]]; then
  printf '%s\n' 'test.sh' 'Exercises release safety and retries in disposable local Git repositories with a mocked GitHub CLI. Does not publish to GitHub.' >&2
  exit 0
fi

if [[ "${SDKGEN_RELEASE_FIXTURE_CHILD:-0}" != 1 ]]; then
  run_step 'release safety and retry simulation' env SDKGEN_RELEASE_FIXTURE_CHILD=1 bash "$ROOT/scripts/release/test.sh" "$@"
  exit 0
fi

test_root="$(mktemp -d "${TMPDIR:-/tmp}/openapi-sdkgen-release-test.XXXXXX")"
origin="$test_root/origin.git"
repository="$test_root/repository"
mock_bin="$test_root/bin"

cleanup() {
  rm -rf "$test_root"
}
trap cleanup EXIT

fail() {
  echo "release script test: $*" >&2
  exit 1
}

assert_contains() {
  local output="$1"
  local expected="$2"
  if [[ "$output" != *"$expected"* ]]; then
    printf '%s\n' "$output" >&2
    fail "expected output to contain: $expected"
  fi
}

assert_not_contains() {
  local output="$1"
  local unexpected="$2"
  if [[ "$output" == *"$unexpected"* ]]; then
    printf '%s\n' "$output" >&2
    fail "expected output not to contain: $unexpected"
  fi
}

git init --bare "$origin" >/dev/null
git --git-dir="$origin" config core.hooksPath "$origin/hooks"
git init --initial-branch=main "$repository" >/dev/null
git -C "$repository" config user.name "release test"
git -C "$repository" config user.email "release-test@example.test"
git -C "$repository" config commit.gpgsign false
git -C "$repository" config tag.gpgsign false
git -C "$repository" config core.hooksPath "$repository/.git/hooks"
git -C "$repository" remote add origin "$origin"
mkdir -p "$repository/scripts/release" "$repository/scripts/lib" "$mock_bin"
cp "$ROOT/scripts/release/publish.sh" "$repository/scripts/release/publish.sh"
cp "$ROOT/scripts/lib/commands.sh" "$ROOT/scripts/lib/redact.py" "$repository/scripts/lib/"
cp "$ROOT/scripts/lib/ui.sh" "$repository/scripts/lib/ui.sh"
chmod +x "$repository/scripts/release/publish.sh"

cat >"$repository/scripts/release/check.sh" <<'EOF'
#!/usr/bin/env bash
printf 'mock release check progress\n'
if [[ -n "${RELEASE_TEST_CHECK_MARKER:-}" ]]; then
  printf 'ran\n' >"$RELEASE_TEST_CHECK_MARKER"
fi
if [[ "${RELEASE_TEST_FAIL_CHECK:-}" == "1" ]]; then
  exit 1
fi
if [[ "${RELEASE_TEST_MUTATE_HEAD:-}" == "1" ]]; then
  git commit --allow-empty -m "test: mutate release head" >/dev/null
fi
if [[ "${RELEASE_TEST_MUTATE_TREE:-}" == "1" ]]; then
  printf '\n# unexpected check mutation\n' >>scripts/lib/ui.sh
fi
EOF
chmod +x "$repository/scripts/release/check.sh"

printf "fixture\n" >"$repository/README.md"
printf ".tmp/\n" >"$repository/.gitignore"

cat >"$mock_bin/gh" <<'EOF'
#!/usr/bin/env bash
printf '%s\n' "$*" >>"${RELEASE_TEST_GH_ARGS:?}"
EOF
chmod +x "$mock_bin/gh"

git -C "$repository" add .
git -C "$repository" commit -m "feat: release helper" >/dev/null
git -C "$repository" tag -a v1.0.0 -m v1.0.0
git -C "$repository" push --follow-tags -u origin main >/dev/null
if [[ -n "$(git -C "$repository" status --porcelain)" ]]; then
  fail "test repository is unexpectedly dirty after setup"
fi

git -C "$repository" switch -c future >/dev/null
git -C "$repository" commit --allow-empty -m "feat!: future breaking change" >/dev/null
git -C "$repository" tag -a v9.0.0 -m v9.0.0
git -C "$repository" push --follow-tags origin future >/dev/null
git -C "$repository" switch main >/dev/null
git -C "$repository" commit --allow-empty -m "fix: main release change" >/dev/null
git -C "$repository" push origin main >/dev/null

run_release() {
  GITHUB_REPOSITORY=test/repository PATH="$mock_bin:$PATH" "$repository/scripts/release/publish.sh" "$@" 2>&1
}

dry_run_marker="$test_root/dry-run-check"
dry_run_head="$(git -C "$repository" rev-parse HEAD)"
output="$(RELEASE_TEST_CHECK_MARKER="$dry_run_marker" run_release --since v1.0.0 --dry-run patch)"
assert_contains "$output" "Last stable tag v1.0.0"
assert_contains "$output" "Tag          v1.0.1"
assert_contains "$(cat "$dry_run_marker")" "ran"
[[ "$(git -C "$repository" rev-parse HEAD)" == "$dry_run_head" ]] || fail "dry run committed"

if output="$(RELEASE_TEST_FAIL_CHECK=1 run_release --since v1.0.0 --dry-run patch 2>&1)"; then
  fail "accepted a dry run with failing release checks"
fi
assert_contains "$output" "Checks failed; aborting release."

output="$(run_release -- --since v1.0.0 --dry-run patch)"
assert_contains "$output" "Release checks passed. No tag or push was created."

if output="$(run_release --since v1.0.0 --dry-run v0.9.0 2>&1)"; then
  fail "accepted a version older than the reachable stable tag"
fi
assert_contains "$output" "Tag must be newer than reachable tag v1.0.0"

if output="$(run_release --since v1.0.0 --dry-run v1.0.1-01 2>&1)"; then
  fail "accepted a prerelease numeric identifier with a leading zero"
fi
assert_contains "$output" "Numeric prerelease identifiers must not have leading zeroes"

git -C "$repository" tag -a v1.1.0-rc.2 -m v1.1.0-rc.2
git -C "$repository" push origin v1.1.0-rc.2 >/dev/null
if output="$(run_release --since v1.0.0 --dry-run v1.1.0-beta.3 2>&1)"; then
  fail "accepted a prerelease older than the reachable prerelease"
fi
assert_contains "$output" "Tag must be newer than reachable tag v1.1.0-rc.2"
output="$(run_release --since v1.0.0 --dry-run)"
assert_contains "$output" "Tag          v1.1.0"

check_head="$(git -C "$repository" rev-parse HEAD)"
if output="$(RELEASE_TEST_FAIL_CHECK=1 run_release --since v1.0.0 --yes v1.1.0 2>&1)"; then
  fail "released with failing checks"
fi
assert_contains "$output" "Checks failed; aborting release."
if git -C "$repository" rev-parse -q --verify refs/tags/v1.1.0 >/dev/null; then
  fail "failed checks created a release tag"
fi

[[ "$(git -C "$repository" rev-parse HEAD)" == "$check_head" ]] || fail "checks created a release commit"
origin_head="$(git --git-dir="$origin" rev-parse refs/heads/main)"
checked_head="$(git -C "$repository" rev-parse HEAD)"
printf '#!/usr/bin/env bash\nexit 1\n' >"$origin/hooks/pre-receive"
chmod +x "$origin/hooks/pre-receive"
if output="$(run_release --since v1.0.0 --yes v1.1.0 2>&1)"; then
  fail "accepted a rejected atomic push"
fi
assert_contains "$output" "Push failed; removed the local tag"
[[ "$(git -C "$repository" rev-parse HEAD)" == "$checked_head" ]] || fail "failed push changed checked HEAD"
[[ "$(git --git-dir="$origin" rev-parse refs/heads/main)" == "$origin_head" ]] || fail "failed atomic push advanced origin"
if git -C "$repository" rev-parse -q --verify refs/tags/v1.1.0 >/dev/null; then
  fail "failed push left a local tag"
fi
if git --git-dir="$origin" rev-parse -q --verify refs/tags/v1.1.0 >/dev/null; then
  fail "failed atomic push left a remote tag"
fi
rm "$origin/hooks/pre-receive"

if output="$(RELEASE_TEST_MUTATE_TREE=1 run_release --since v1.0.0 --yes v1.1.0 2>&1)"; then
  fail "released a tree changed by checks"
fi
assert_contains "$output" "working tree changed while checks ran"
git -C "$repository" restore -- scripts/lib/ui.sh

if output="$(RELEASE_TEST_MUTATE_HEAD=1 run_release --since v1.0.0 v1.1.0 2>&1)"; then
  fail "tagged a HEAD changed by the check command"
fi
assert_contains "$output" "branch or HEAD changed while checks ran"

resume_args="$test_root/resume-args"
resume_run_output="$(RELEASE_TEST_GH_ARGS="$resume_args" run_release --resume v1.0.0 --yes)"
resume_output="$(cat "$resume_args")"
assert_contains "$resume_output" "workflow run release.yml --ref main -f tag=v1.0.0"
assert_not_contains "$resume_output" "run watch"
assert_contains "$resume_run_output" "release workflow dispatched for v1.0.0"
assert_contains "$resume_run_output" "https://github.com/test/repository/actions/workflows/release.yml"

release_args="$test_root/release-args"
release_output="$(RELEASE_TEST_GH_ARGS="$release_args" run_release --since v1.0.0 --yes v1.1.0)"
assert_contains "$release_output" "created and pushed tag v1.1.0"
assert_contains "$release_output" "release workflow dispatched for v1.1.0"
assert_contains "$release_output" "https://github.com/test/repository/actions/workflows/release.yml"
if [[ -e "$release_args" ]]; then
  fail "normal tag release unexpectedly invoked gh"
fi
[[ "$(git -C "$repository" rev-parse 'v1.1.0^{commit}')" == "$(git -C "$repository" rev-parse HEAD)" ]] || fail "tag missed checked HEAD"
[[ "$(git -C "$repository" status --porcelain)" == "" ]] || fail "release left dirty files"

# A dirty tree must fail before checks, tags, or pushes.
clean_head="$(git -C "$repository" rev-parse HEAD)"
printf '\n- A later fix.\n' >>"$repository/README.md"
if output="$(run_release --since v1.1.0 --yes patch 2>&1)"; then
  fail "accepted a dirty working tree"
fi
assert_contains "$output" "Release requires a clean working tree"
[[ "$(git -C "$repository" rev-parse HEAD)" == "$clean_head" ]] || fail "dirty tree caused preparation"

# Execute the workflow's retry step with real checksums and a mocked release API.
assets_step="$test_root/assets-step.sh"
python3 - "$ROOT/.github/workflows/release.yml" "$assets_step" <<'PY'
import pathlib
import sys

lines = pathlib.Path(sys.argv[1]).read_text().splitlines()
step = lines.index("        id: assets")
start = lines.index("        run: |", step) + 1
body = []
for line in lines[start:]:
    if line and not line.startswith("          "):
        break
    body.append(line[10:])
if not body:
    raise SystemExit("missing workflow asset retry step")
pathlib.Path(sys.argv[2]).write_text("\n".join(body) + "\n")
PY
assets_fixture="$test_root/assets-fixture"
assets_bin="$test_root/assets-bin"
mkdir -p "$assets_fixture" "$assets_bin"
for platform in darwin linux windows; do
  extension=tar.gz
  if [[ "$platform" == windows ]]; then extension=zip; fi
  for arch in amd64 arm64; do
    printf '%s %s\n' "$platform" "$arch" >"$assets_fixture/openapi-sdkgen_1.0.0_${platform}_${arch}.${extension}"
  done
done
(cd "$assets_fixture" && sha256sum openapi-sdkgen_* >checksums.txt)
cat >"$assets_bin/gh" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail
case "$1" in
  api)
    case "$RELEASE_TEST_ASSETS_CASE" in
      new) ;;
      draft) printf '{"draft":true,"immutable":false}\n' ;;
      mutable) printf '{"draft":false,"immutable":false}\n' ;;
      api-failure) exit 1 ;;
      *) printf '{"draft":false,"immutable":true}\n' ;;
    esac
    ;;
  release)
    cp "$RELEASE_TEST_ASSETS_FIXTURE/"* dist/
    case "$RELEASE_TEST_ASSETS_CASE" in
      corrupt) printf 'corrupt\n' >>dist/openapi-sdkgen_1.0.0_linux_amd64.tar.gz ;;
      missing)
        rm dist/openapi-sdkgen_1.0.0_windows_arm64.zip
        (cd dist && sha256sum openapi-sdkgen_* >checksums.txt)
        ;;
    esac
    ;;
  *) exit 1 ;;
esac
EOF
chmod +x "$assets_bin/gh"
run_assets_step() {
  local scenario="$1"
  local directory="$test_root/assets-$scenario"
  mkdir -p "$directory"
  (
    cd "$directory"
    export RUNNER_TEMP="$directory" GITHUB_OUTPUT="$directory/output"
    export TAG=v1.0.0 VERSION=1.0.0 GITHUB_REPOSITORY=test/repository
    export RELEASE_TEST_ASSETS_CASE="$scenario" RELEASE_TEST_ASSETS_FIXTURE="$assets_fixture"
    PATH="$assets_bin:$PATH" bash "$assets_step"
  )
}
for scenario in new draft published; do
  run_assets_step "$scenario" >/dev/null || fail "asset retry rejected $scenario release"
  expected_build=true
  if [[ "$scenario" == published ]]; then expected_build=false; fi
  assert_contains "$(cat "$test_root/assets-$scenario/output")" "build=$expected_build"
done
for scenario in mutable api-failure corrupt missing; do
  if run_assets_step "$scenario" >"$test_root/assets-$scenario.log" 2>&1; then
    fail "asset retry accepted $scenario release"
  fi
  [[ ! -s "$test_root/assets-$scenario/output" ]] || fail "failed asset retry enabled publication"
done
