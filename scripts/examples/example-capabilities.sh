#!/usr/bin/env bash
set -euo pipefail
source "$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)/scripts/lib/commands.sh"
set -- "${SCRIPT_ARGS[@]}"
if [[ "${1:-}" == --help || "${2:-}" == --help ]]; then
  printf '%s\n' 'example-capabilities.sh' 'Builds the SDK/example, starts temporary loopback servers, runs the client and stops the servers. Port values come from devtools or the caller.' >&2
  exit 0
fi

example="$ROOT/examples/typescript-openapi-capabilities-app"
api_port="${CAPABILITIES_API_PORT:-18790}"
webhook_port="${CAPABILITIES_WEBHOOK_PORT:-18791}"
bash "$ROOT/scripts/dev/go.sh" build
run_step "prepare OpenAPI capability example" env SDKGEN_BIN="$ROOT/.tmp/bin/openapi-sdkgen" "$example/setup.sh"
server_log="$LOG_DIR/server.log"
CAPABILITIES_API_PORT="$api_port" CAPABILITIES_WEBHOOK_PORT="$webhook_port" run_live diagnostic "example server" node "$example/dist/server.js" >"$server_log" 2>&1 &
server_pid=$!
trap 'kill "$server_pid" 2>/dev/null || true; wait "$server_pid" 2>/dev/null || true' EXIT
for _ in {1..20}; do
  if grep -q "Capability inbound host listening" "$server_log"; then
    break
  fi
  sleep 0.1
done
if ! grep -q "Capability API listening" "$server_log" || ! grep -q "Capability inbound host listening" "$server_log"; then
  script_diagnostic "$server_log"
  exit 1
fi
CAPABILITIES_API_BASE_URL="http://127.0.0.1:$api_port/v1" \
CAPABILITIES_WEBHOOK_BASE_URL="http://127.0.0.1:$webhook_port" \
  run_step "run OpenAPI capability client" ts_pnpm --dir "$example" run client
