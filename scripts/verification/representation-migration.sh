#!/usr/bin/env bash
set -euo pipefail
source "$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)/scripts/lib/commands.sh"
set -- "${SCRIPT_ARGS[@]}"
if [[ "${1:-}" == --help || "${2:-}" == --help ]]; then
  printf '%s\n' 'representation-migration.sh --baseline BINARY --candidate BINARY [--fixtures IDS]' 'Checks incremental migration/publication using disposable SDKs under .tmp/representation-migration. stdout is JSON.' >&2
  exit 0
fi
run_data "representation-migration" ts_node node "$TYPESCRIPT_ROOT/verification/migration.mjs" "$@"
