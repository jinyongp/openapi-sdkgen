#!/usr/bin/env bash
source "$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)/scripts/lib/commands.sh"
set -- "${SCRIPT_ARGS[@]}"
if [[ "${1:-}" == --help ]]; then
  echo 'example-go.sh: builds the generator and runs the Go SDK example in a disposable module with loopback HTTP servers' >&2
  exit 0
fi
cd "$ROOT"
temporary="$(mktemp -d "$ROOT/.tmp/go-example.XXXXXX")"
trap 'rm -rf "$temporary"' EXIT
cp "$ROOT/examples/golang-todo-app/"{main.go,openapi.json,go.mod,go.sum} "$temporary/"
run_step 'build Go example generator' go build -o "$temporary/generator" ./cmd/openapi-sdkgen
run_step 'generate Go example SDK' "$temporary/generator" generate --input "$temporary/openapi.json" --target golang --go-import-path example.com/sdkgen-go-example/generated/api --with server --output "$temporary/generated/api"
GOWORK=off GOTOOLCHAIN=local run_step 'run Go example' go -C "$temporary" run -mod=readonly .
