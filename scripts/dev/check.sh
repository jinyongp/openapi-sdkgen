#!/usr/bin/env bash
source "$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)/scripts/lib/commands.sh"
set -- "${SCRIPT_ARGS[@]}"
if [[ "${1:-}" == --help ]]; then
  echo 'check.sh: full CI suites, release cross-builds and example clients. Writes disposable artifacts and starts temporary loopback example servers. Does not publish.' >&2
  exit 0
fi
bash "$ROOT/scripts/dev/ci.sh"
bash "$ROOT/scripts/release/cross-build.sh"
TODO_API_PORT="${TODO_API_PORT:-$((20000 + RANDOM % 10000))}" bash "$ROOT/scripts/examples/example-todo.sh"
WIDGET_API_PORT="${WIDGET_API_PORT:-$((30000 + RANDOM % 10000))}" bash "$ROOT/scripts/examples/example-advanced.sh"
CAPABILITIES_API_PORT="${CAPABILITIES_API_PORT:-$((50000 + RANDOM % 10000))}" CAPABILITIES_WEBHOOK_PORT="${CAPABILITIES_WEBHOOK_PORT:-$((51000 + RANDOM % 10000))}" bash "$ROOT/scripts/examples/example-capabilities.sh"
script_note 'ok integrated checks'
