#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "${BASH_SOURCE[0]}")"
"${SDKGEN_BIN:-openapi-sdkgen}" generate --input openapi.json --target golang \
  --go-import-path example.com/sdkgen-go-stream-example/generated/api \
  --output generated/api --incremental
(cd server && go mod download)
