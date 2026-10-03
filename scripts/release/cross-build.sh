#!/usr/bin/env bash
set -euo pipefail
source "$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)/scripts/lib/commands.sh"
set -- "${SCRIPT_ARGS[@]}"
if [[ "${1:-}" == --help || "${2:-}" == --help ]]; then
  printf '%s\n' 'cross-build.sh' 'Builds six macOS/Linux/Windows binaries under .tmp/runs. Does not publish.' >&2
  exit 0
fi
cd "$ROOT"
for target in darwin/amd64 darwin/arm64 linux/amd64 linux/arm64 windows/amd64 windows/arm64; do
  os="${target%/*}"
  arch="${target#*/}"
  output="$LOG_DIR/builds/openapi-sdkgen-${os}-${arch}"
  if [[ "$os" == windows ]]; then
    output+=".exe"
  fi
  mkdir -p "$(dirname "$output")"
  run_step "release build ${target}" env CGO_ENABLED=0 GOOS="$os" GOARCH="$arch" go build -o "$output" ./cmd/openapi-sdkgen
done
script_note "ok release check"
