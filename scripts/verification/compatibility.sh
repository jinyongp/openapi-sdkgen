#!/usr/bin/env bash
source "$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)/scripts/lib/commands.sh"
set -- "${SCRIPT_ARGS[@]}"
cd "$ROOT"
if [[ "${1:-}" == --help ]]; then
  echo 'compatibility.sh [all|5.7.3|5.9.3|6.0.3|7.0.2] [all|consumers|declarations]: checks representative SDK consumer compatibility; writes disposable SDKs' >&2
  exit 0
fi
versions=(5.7.3 5.9.3 6.0.3 7.0.2)
if [[ "${1:-all}" != all ]]; then
  case "$1" in 5.7.3|5.9.3|6.0.3|7.0.2) versions=("$1");; *) script_error "unsupported compiler version: $1"; exit 2;; esac
fi
group="${2:-all}"
case "$group" in all|consumers|declarations) ;; *) script_error "unsupported compiler group: $group"; exit 2;; esac
require_system_node
temporary="$(mktemp -d "$ROOT/.tmp/compatibility.XXXXXX")"
trap 'rm -rf "$temporary"' EXIT
if [[ "$group" != declarations ]]; then
  run_step 'compatibility generator' go build -o "$temporary/generator" ./cmd/openapi-sdkgen
  run_step 'compatibility snapshot helper' go build -o "$temporary/snapshot" ./scripts/generate-check/snapshot
fi
printf '#!/usr/bin/env bash\nexec node "$SDKGEN_COMPAT_TSC_PATH" "$@"\n' >"$temporary/compiler"
chmod 700 "$temporary/compiler"
export SDKGEN_GENERATE_CHECK_SKIP_BUILD=1
export SDKGEN_GENERATE_CHECK_GENERATOR_BIN="$temporary/generator"
export SDKGEN_GENERATE_CHECK_ACQUIRE_BIN="$temporary/snapshot"
export SDKGEN_GENERATE_CHECK_TSC_BIN="$temporary/compiler"
check_sdk() { bash "$ROOT/scripts/generate/check.sh" "$@"; }
for version in "${versions[@]}"; do
  case "$version" in 5.7.3) package=typescript-5-7;; 5.9.3) package=typescript-5-9;; 6.0.3) package=typescript-6;; 7.0.2) package=typescript;; esac
  export SDKGEN_COMPAT_TSC_PATH="$TYPESCRIPT_ROOT/node_modules/$package/lib/tsc.js"
  if [[ "$group" != declarations ]]; then
  for typecheck in false true; do
    run_step "TypeScript $version wildcard body (typecheck=$typecheck)" check_sdk test/fixtures/wildcard-request-body.json typescript -- --typecheck="$typecheck"
  done
  run_step "TypeScript $version server/metadata" check_sdk test/fixtures/support-gaps/oas32-normative.json typescript -- --with server --with metadata --typecheck=true
  run_step "TypeScript $version named/selected Link SDK" check_sdk test/fixtures/generation-selection.json typescript -- --config "$ROOT/test/fixtures/named-clients-linked.toml" --operation getTask --with metadata
  run_step "TypeScript $version named errors" check_sdk test/fixtures/named-clients-errors.json typescript -- --config "$ROOT/test/fixtures/named-clients-errors.toml"
  SDKGEN_GENERATE_CHECK_MODULE_PROFILE=bundler run_step "TypeScript $version Bundler" check_sdk test/fixtures/generation-selection.json typescript -- --operation getTask --with metadata
  SDKGEN_GENERATE_CHECK_MODULE_PROFILE=commonjs SDKGEN_GENERATE_CHECK_CHECKING_PROFILE=relaxed run_step "TypeScript $version CommonJS consumer" check_sdk test/fixtures/named-clients-errors.json typescript -- --config "$ROOT/test/fixtures/named-clients-errors.toml"
  SDKGEN_GENERATE_CHECK_CHECKING_PROFILE=isolated run_step "TypeScript $version isolated declarations" check_sdk test/fixtures/support-gaps/oas32-normative.json typescript -- --with server
  fi
  if [[ "$group" != consumers ]]; then
    run_step "TypeScript $version declaration matrix" go test -count=1 -timeout=20m -run "^TestRuntimeFeatureDeclarationMatrixRegression$/^${package}$" ./internal/target/typescript
  fi
  script_note "ok TypeScript $version consumer compatibility"
done
