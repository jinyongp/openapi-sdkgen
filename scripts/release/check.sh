#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$ROOT"

if ! command -v just >/dev/null 2>&1; then
  echo "release checks require just" >&2
  exit 1
fi

just agent ts-install
just agent ci
just agent release-check

printf 'ok release checks\n'
