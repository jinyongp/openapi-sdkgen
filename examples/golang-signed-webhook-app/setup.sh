#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "${BASH_SOURCE[0]}")"
"${SDKGEN_BIN:-openapi-sdkgen}" generate --input openapi.json --target golang \
  --output generated/api --with server --incremental
