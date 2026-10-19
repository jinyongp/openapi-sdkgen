#!/usr/bin/env bash
set -euo pipefail
source "$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)/scripts/lib/commands.sh"
set -- "${SCRIPT_ARGS[@]}"

if [[ "${1:-}" == --help || $# -lt 1 ]]; then
  echo "usage: check.sh INPUT [TARGET] [-- GENERATOR_OPTIONS...]; acquires input (possibly network), checks a temporary SDK, then removes it" >&2
  [[ "${1:-}" == --help ]] && exit 0
  exit 2
fi

input="$1"
shift
target="typescript"
if (($# > 0)) && [[ "$1" != "--" ]]; then
  target="$1"
  shift
fi
if (($# > 0)); then
  if [[ "$1" != "--" ]]; then
    echo "generate-check options must follow --" >&2
    exit 2
  fi
  shift
fi

generator_options=("$@")
snapshot_options=()
for ((index = 0; index < ${#generator_options[@]}; index++)); do
  argument="${generator_options[index]}"
  case "$argument" in
    --input|-input|--input=*|-input=*|--input-base|-input-base|--input-base=*|-input-base=*|--output|-output|--output=*|-output=*|--target|-target|--target=*|-target=*)
      script_error "generate-check owns $argument and does not allow overriding it"
      exit 2
      ;;
    --http-header-env|-http-header-env|--tls-client-cert|-tls-client-cert|--tls-client-key|-tls-client-key|--tls-ca-file|-tls-ca-file)
      if ((index + 1 >= ${#generator_options[@]})); then
        script_error "$argument requires a value"
        exit 2
      fi
      snapshot_options+=("$argument" "${generator_options[index + 1]}")
      ((index += 1))
      ;;
    --http-header-env=*|-http-header-env=*|--tls-client-cert=*|-tls-client-cert=*|--tls-client-key=*|-tls-client-key=*|--tls-ca-file=*|-tls-ca-file=*|--offline|-offline|--offline=*|-offline=*)
      snapshot_options+=("$argument")
      ;;
  esac
done

temporary="$(mktemp -d "${TMPDIR:-/tmp}/openapi-sdkgen-generate-check.XXXXXX")"
chmod 700 "$temporary"
cleanup() {
  rm -rf "$temporary"
}
trap cleanup EXIT

snapshot="$temporary/openapi.snapshot"
effective_base_file="$temporary/effective-base"
report="$temporary/snapshot.json"
generated="$temporary/generated"
checked="$temporary/checked"
mkdir -p "$checked"

snapshot_bin="${SDKGEN_GENERATE_CHECK_ACQUIRE_BIN:-$temporary/snapshot}"
generator_bin="${SDKGEN_GENERATE_CHECK_GENERATOR_BIN:-$temporary/generator}"
if [[ "${SDKGEN_GENERATE_CHECK_SKIP_BUILD:-0}" != "1" ]]; then
  run_step "build SDK generator" go build -o "$generator_bin" ./cmd/openapi-sdkgen
  run_step "build generate-check snapshot helper" go build -o "$snapshot_bin" ./scripts/generate-check/snapshot
fi

run_step "snapshot OpenAPI input" "$snapshot_bin" \
  --input "$input" \
  --data-output "$snapshot" \
  --base-output "$effective_base_file" \
  --report-output "$report" \
  ${snapshot_options[@]+"${snapshot_options[@]}"}

effective_base_with_sentinel="$(cat "$effective_base_file"; printf '\x1f')"
effective_base="${effective_base_with_sentinel%$'\x1f'}"
generation_status=0
set +e
run_step "generate checked SDK" "$generator_bin" generate \
  --input - \
  --input-base "$effective_base" \
  --target "$target" \
  --output "$generated" \
  ${generator_options[@]+"${generator_options[@]}"} <"$snapshot"
generation_status=$?
set -e

if [[ "$SCRIPT_VERBOSE" == 1 ]]; then
  python3 "$ROOT/scripts/lib/redact.py" <"$report" >&2
fi
script_note "generation exit: $generation_status"
if ((generation_status != 0)); then
  script_note "typecheck exit: not-run"
  exit "$generation_status"
fi

cp -R "$generated/." "$checked/"
case "$target" in
  typescript) bash "$ROOT/scripts/generate/verify-typescript.sh" "$checked" ;;
  golang) bash "$ROOT/scripts/generate/verify-golang.sh" "$checked" ;;
  *) script_error "no generated SDK verifier for target: $target"; exit 2 ;;
esac

script_note "ok generated $target SDK"
