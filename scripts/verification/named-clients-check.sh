#!/usr/bin/env bash
set -euo pipefail
source "$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)/scripts/lib/commands.sh"
set -- "${SCRIPT_ARGS[@]}"
if [[ "${1:-}" == --help || "${2:-}" == --help ]]; then
  printf '%s\n' 'named-clients-check.sh [--graph-source PINNED_OPENAPI_FILE]' 'Builds measurement tools and verifies generated named clients; writes reports under .tmp.' >&2
  exit 0
fi
cd "$ROOT"
mkdir -p "$ROOT/.tmp/bin"
generator="$ROOT/.tmp/bin/named-clients-generator"
benchmark="$ROOT/.tmp/bin/named-clients-benchmark"
run_step "build named clients generator" go build -o "$generator" ./cmd/openapi-sdkgen
run_step "build named clients phase measurement driver" go build -o "$benchmark" ./scripts/clients-benchmark
run_step "named clients delivery verification" ts_node node "$TYPESCRIPT_ROOT/verification/named-clients.mjs" --generator "$generator" --benchmark "$benchmark" "$@"
