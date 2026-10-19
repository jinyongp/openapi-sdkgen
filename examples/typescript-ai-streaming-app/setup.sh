#!/usr/bin/env bash
set -euo pipefail
directory="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
"${SDKGEN_BIN:-openapi-sdkgen}" generate \
  --input "$directory/openapi.yaml" \
  --target typescript \
  --output "$directory/src/generated/api" --incremental
corepack pnpm@12.14.0 --dir "$directory" install --frozen-lockfile
corepack pnpm@12.14.0 --dir "$directory" run build
