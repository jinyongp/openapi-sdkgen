#!/usr/bin/env bash
set -euo pipefail
source "$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)/scripts/lib/commands.sh"
set -- "${SCRIPT_ARGS[@]}"
if [[ "${1:-}" == --help ]]; then
  printf '%s\n' 'provider-size.sh OUTPUT BASELINE_BINARY CURRENT_BINARY MANIFEST' 'Optional pinned-input comparison of full, loadOperations and root/named selection for one and two APIs. Sums every deployed JS/gzip chunk; executes browser bundles without network requests. Writes owner-only SDKs, logs and JSON reports. Set manifest private=true to keep error details local. Exit 1 if JS or gzip exceeds the baseline by 5%.' >&2
  exit 0
fi
if (( $# != 4 )); then
  script_error 'usage: provider-size.sh OUTPUT BASELINE_BINARY CURRENT_BINARY MANIFEST'
  exit 2
fi
provider_args=()
for provider_arg in "$@"; do
  if [[ "$provider_arg" != /* ]]; then provider_arg="$ROOT/$provider_arg"; fi
  provider_args+=("$provider_arg")
done
run_step 'provider release size comparison' ts_node node "$TYPESCRIPT_ROOT/verification/provider-size-measure.mjs" "${provider_args[@]}"
script_note "ok provider size comparison: ${provider_args[0]}/comparisons.json"
