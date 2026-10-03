#!/usr/bin/env bash
set -euo pipefail
source "$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)/scripts/lib/commands.sh"
set -- "${SCRIPT_ARGS[@]}"
if [[ "${1:-}" == --help || "${2:-}" == --help ]]; then
  printf '%s\n' 'sdk-delivery-check.sh [--sizes 100,1000,10000]' 'Locks the shared SDK delivery run, builds a generator, measures generated SDKs and writes reports. Existing run lock exits 75.' >&2
  exit 0
fi
cd "$ROOT"
# Lock before self-tests or building the shared generator, not only one source tree.
lock="$ROOT/.tmp/sdk-delivery-check.lock"
if ! mkdir "$lock" 2>/dev/null; then
  echo "SDK delivery verification is already locked: $lock (inspect the owner before recovery)" >&2
  exit 75
fi
release_lock() {
  status=$?
  trap - EXIT
  rm -f -- "$lock/owner"
  rmdir -- "$lock" || true
  exit "$status"
}
trap release_lock EXIT
trap 'exit 129' HUP
trap 'exit 130' INT
trap 'exit 143' TERM
printf 'pid=%s\n' "$$" > "$lock/owner"
mkdir -p "$ROOT/.tmp/bin"
run_step "build SDK delivery generator" go build -o "$ROOT/.tmp/bin/sdk-delivery-generator" ./cmd/openapi-sdkgen
run_step "generated SDK delivery verification" ts_node node "$TYPESCRIPT_ROOT/verification/sdk-delivery.mjs" --generator "$ROOT/.tmp/bin/sdk-delivery-generator" "$@"
run_step "named clients delivery verification" ts_node node "$TYPESCRIPT_ROOT/verification/named-clients.mjs" --generator "$ROOT/.tmp/bin/sdk-delivery-generator"
