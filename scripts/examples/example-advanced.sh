#!/usr/bin/env bash
set -euo pipefail
source "$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)/scripts/lib/commands.sh"
set -- "${SCRIPT_ARGS[@]}"
if [[ "${1:-}" == --help || "${2:-}" == --help ]]; then
  printf '%s\n' 'example-advanced.sh' 'Builds the SDK/example, starts temporary loopback servers, runs the client and stops the servers. Port values come from devtools or the caller.' >&2
  exit 0
fi

example="$ROOT/examples/typescript-advanced-app"
port="${WIDGET_API_PORT:-18788}"
bash "$ROOT/scripts/dev/go.sh" build
run_step "prepare advanced TypeScript example" env SDKGEN_BIN="$ROOT/.tmp/bin/openapi-sdkgen" "$example/setup.sh"
server_log="$LOG_DIR/server.log"
WIDGET_API_PORT="$port" run_live diagnostic "example server" node "$example/dist/server.js" >"$server_log" 2>&1 &
server_pid=$!
trap 'kill "$server_pid" 2>/dev/null || true; wait "$server_pid" 2>/dev/null || true' EXIT
for _ in {1..20}; do
  if grep -q "Widget API listening" "$server_log"; then
    break
  fi
  sleep 0.1
done
if ! grep -q "Widget API listening" "$server_log"; then
  script_diagnostic "$server_log"
  exit 1
fi
WIDGET_API_BASE_URL="http://127.0.0.1:$port/v1" run_step "run advanced TypeScript client" ts_pnpm --dir "$example" run client
