#!/usr/bin/env bash
source "$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)/scripts/lib/commands.sh"
set -- "${SCRIPT_ARGS[@]}"
if [[ "${1:-}" == --help ]]; then
  echo 'go-check-test.sh: real Go generation/consumer, no Node/TypeScript, failure propagation and temporary cleanup' >&2
  exit 0
fi
cd "$ROOT"
temporary="$(mktemp -d "$ROOT/.tmp/go-generation-test.XXXXXX")"
trap 'rm -rf "$temporary"' EXIT
mkdir -p "$temporary/work" "$temporary/bin"
run_step 'build Go verification generator' go build -o "$temporary/generator" ./cmd/openapi-sdkgen
run_step 'build Go input snapshot helper' go build -o "$temporary/snapshot" ./scripts/generate-check/snapshot
export SDKGEN_GENERATE_CHECK_SKIP_BUILD=1
export SDKGEN_GENERATE_CHECK_ACQUIRE_BIN="$temporary/snapshot"
export SDKGEN_GENERATE_CHECK_GENERATOR_BIN="$temporary/generator"
export TMPDIR="$temporary/work"
# The Go consumer branch must not discover Node, pnpm or TypeScript from PATH.
for tool in bash python3 dirname mkdir mktemp chmod cat rm cp find sed go; do
  ln -s "$(command -v "$tool")" "$temporary/bin/$tool"
done
input="$ROOT/test/fixtures/generation-selection.json"
run_step 'generated Go SDK without Node or TypeScript' env PATH="$temporary/bin" bash "$ROOT/scripts/generate/check.sh" "$input" golang -- --go-import-path example.test/consumer/internal/api
cat >"$temporary/generation.toml" <<'TOML'
target = "golang"
[golang]
package = "client"
import_path = "example.test/configured/internal/client"
TOML
run_step 'configured Go import and package' bash "$ROOT/scripts/generate/check.sh" "$input" golang -- --config "$temporary/generation.toml"
printf '#!/usr/bin/env bash\nexit 17\n' >"$temporary/compiler"
chmod 700 "$temporary/compiler"
status=0
SDKGEN_GENERATE_CHECK_GO_BIN="$temporary/compiler" bash "$ROOT/scripts/generate/check.sh" "$input" golang -- --go-import-path example.test/consumer/internal/api >"$temporary/failure.stdout" 2>"$temporary/failure.stderr" || status=$?
if [[ "$status" != 17 ]]; then
  script_error "Go compiler failure status: $status; expected 17"
  exit 1
fi
if [[ -n "$(find "$temporary/work" -mindepth 1 -print -quit)" ]]; then
  script_error 'Go generation check left disposable SDK files behind'
  exit 1
fi
script_note 'ok Go generation verification: no Node/TypeScript, compilation/runtime, compiler failure, cleanup'
