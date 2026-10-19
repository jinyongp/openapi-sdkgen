#!/usr/bin/env bash
source "$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)/scripts/lib/commands.sh"
set -- "${SCRIPT_ARGS[@]}"
if [[ "${1:-}" == --help ]]; then
  printf '%s\n' 'smoke-go.sh native EXECUTABLE [VERSION] | npm VERSION: verify Go generation and a local HTTP call through the selected distribution.' >&2
  exit 0
fi
mode="${1:?distribution mode required}"
shift
if [[ "$mode" == npm ]]; then
  package_version="${1:?version required}"
  [[ "$package_version" =~ ^[0-9]+\.[0-9]+\.[0-9]+(-[a-zA-Z0-9.-]+)?$ ]] || { script_error 'Invalid npm version'; exit 2; }
  launcher=(npx --yes "openapi-sdkgen@$package_version")
  expected_version="$package_version"
elif [[ "$mode" == native ]]; then
  launcher=("${1:?executable required}")
  expected_version="${2:-}"
else
  script_error 'Expected native or npm distribution'; exit 2
fi
scratch="$(mktemp -d "$ROOT/.tmp/go-distribution-smoke.XXXXXX")"
trap 'rm -rf "$scratch"' EXIT
cp "$ROOT/examples/golang-todo-app/go.mod" "$scratch/"
sed -i.bak 's|example.com/sdkgen-go-example|example.com/consumer|' "$scratch/go.mod"
awk '/^```go$/{copy=1;next} /^```$/{if(copy)exit} copy {print}' "$ROOT/docs/golang/guide/getting-started.md" > "$scratch/main.go"

verify_version() {
  local actual
  actual="$("${launcher[@]}" --version)"
  if [[ -n "$expected_version" && "$actual" != "openapi-sdkgen $expected_version" ]]; then
    script_error "Unexpected distribution version: $actual"; return 1
  fi
}
verify_call() {
  local actual
  actual="$(GOWORK=off GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off CGO_ENABLED=0 GOMODCACHE="$scratch/module-cache" go -C "$scratch" run -mod=readonly .)" || return
  [[ "$actual" == 'todo-1 Ship the SDK' ]] || { script_error "Unexpected Go call result: $actual"; return 1; }
}
run_step 'distribution version' verify_version
run_step 'distribution Go generation' "${launcher[@]}" generate --input "$ROOT/docs/public/examples/golang-todo.json" --target golang --go-import-path example.com/consumer/internal/api --output "$scratch/internal/api" --with server
run_step 'distribution Go SDK local call' verify_call
