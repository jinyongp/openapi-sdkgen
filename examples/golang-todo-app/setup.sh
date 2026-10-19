#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "${BASH_SOURCE[0]}")"
"${SDKGEN_BIN:-openapi-sdkgen}" generate --input openapi.json --target golang --go-import-path example.com/sdkgen-go-example/generated/api --with server --output generated/api --incremental
