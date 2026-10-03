#!/usr/bin/env bash
source "$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)/scripts/lib/commands.sh"
set -- "${SCRIPT_ARGS[@]}"
if [[ "${1:-}" == --help ]]; then
  echo 'check-test.sh: checks real SDK generation and compiler failure propagation; uses temporary artifacts' >&2
  exit 0
fi
cd "$ROOT"
temporary="$(mktemp -d "$ROOT/.tmp/generation-test.XXXXXX")"
trap 'rm -rf "$temporary"' EXIT
mkdir -p "$temporary/work"
input="$ROOT/test/fixtures/generation-selection.json"
bash "$ROOT/scripts/dev/go.sh" build
run_step 'snapshot helper' go build -o "$temporary/snapshot" ./scripts/generate-check/snapshot
export SDKGEN_GENERATE_CHECK_SKIP_BUILD=1
export SDKGEN_GENERATE_CHECK_ACQUIRE_BIN="$temporary/snapshot"
export SDKGEN_GENERATE_CHECK_GENERATOR_BIN="$ROOT/.tmp/bin/openapi-sdkgen"
export TMPDIR="$temporary/work"
run_step 'generated SDK compiles' bash "$ROOT/scripts/generate/check.sh" "$input"
# Only substitute the failing compiler; input acquisition and generation stay real.
printf '#!/usr/bin/env bash\nexit 17\n' >"$temporary/compiler"
chmod 700 "$temporary/compiler"
status=0
SDKGEN_GENERATE_CHECK_TSC_BIN="$temporary/compiler" bash "$ROOT/scripts/generate/check.sh" "$input" >"$temporary/failure.stdout" 2>"$temporary/failure.stderr" || status=$?
if [[ "$status" != 17 ]]; then
  echo "compiler failure status: $status; expected 17" >&2
  exit 1
fi
if [[ -n "$(find "$temporary/work" -mindepth 1 -print -quit)" ]]; then
  echo 'generation check left disposable SDK files behind' >&2
  exit 1
fi
script_note 'ok generation check: compilation, compiler failure, temporary cleanup'
