#!/usr/bin/env bash
set -euo pipefail

directory="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cli="${SDKGEN_BIN:-openapi-sdkgen}"

rm -rf "$directory/src/generated/capabilities-sdk"
"$cli" generate \
  --input "$directory/openapi.json" \
  --target typescript \
  --with server \
  --output "$directory/src/generated/capabilities-sdk"
corepack pnpm@12.4.1 --dir "$directory" install --frozen-lockfile
corepack pnpm@12.4.1 --dir "$directory" run build
