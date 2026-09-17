#!/usr/bin/env bash
set -euo pipefail
umask 077

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
TYPESCRIPT_ROOT="$ROOT/test/typescript"
LOG_DIR="$ROOT/.tmp/agent-logs"
TAIL_LINES="${AGENT_TAIL_LINES:-80}"

mkdir -p "$LOG_DIR" "$ROOT/.tmp/go-build" "$ROOT/.tmp/go-mod" "$ROOT/.tmp/pnpm-store" "$ROOT/.tmp/corepack" "$ROOT/.tmp/node-cache"
chmod 700 "$LOG_DIR" "$ROOT/.tmp/go-build" "$ROOT/.tmp/go-mod" "$ROOT/.tmp/pnpm-store" "$ROOT/.tmp/corepack" "$ROOT/.tmp/node-cache"

export GOCACHE="$ROOT/.tmp/go-build"
export GOMODCACHE="$ROOT/.tmp/go-mod"
export COREPACK_HOME="$ROOT/.tmp/corepack"
export npm_config_cache="$ROOT/.tmp/node-cache"
export npm_config_store_dir="$ROOT/.tmp/pnpm-store"
export CI=true

NODE_VERSION="24.21.0"
PNPM_VERSION="12.4.1"

require_system_node() {
  local actual
  actual="$(node --version 2>/dev/null || true)"
  if [[ "$actual" != "v$NODE_VERSION" ]]; then
    echo "Node $NODE_VERSION is required when fnm is unavailable (found ${actual:-none})" >&2
    return 1
  fi
}

ts_node() {
  if command -v fnm >/dev/null 2>&1; then
    (cd "$TYPESCRIPT_ROOT" && fnm exec --using "$NODE_VERSION" "$@")
    return
  fi
  require_system_node
  (cd "$TYPESCRIPT_ROOT" && "$@")
}

ts_pnpm() {
  if command -v fnm >/dev/null 2>&1; then
    (cd "$TYPESCRIPT_ROOT" && fnm exec --using "$NODE_VERSION" corepack "pnpm@$PNPM_VERSION" --config.store-dir="$ROOT/.tmp/pnpm-store" "$@")
    return
  fi
  require_system_node
  (cd "$TYPESCRIPT_ROOT" && corepack "pnpm@$PNPM_VERSION" --config.store-dir="$ROOT/.tmp/pnpm-store" "$@")
}

agent_run() {
  local label="$1"
  shift
  local safe_label
  safe_label="$(printf '%s' "$label" | tr -cs 'A-Za-z0-9_.-' '-')"
  local log_file="$LOG_DIR/$(date -u +%Y%m%dT%H%M%SZ)-${safe_label}.log"

  if [[ "${VERBOSE:-0}" == "1" ]]; then
    echo "run $label"
    if "$@"; then
      echo "ok $label"
      return
    else
      local status=$?
      echo "failed $label"
      return "$status"
    fi
  fi

  if "$@" >"$log_file" 2>&1; then
    echo "ok $label"
    return
  else
    local status=$?
    echo "failed $label"
    printf 'command: %s\n' "$*"
    echo "log: $log_file"
    tail -n "$TAIL_LINES" "$log_file" || true
    return "$status"
  fi
}

agent_note() {
  printf '%s\n' "$*"
}
