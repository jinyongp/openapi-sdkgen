#!/usr/bin/env bash
source "$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)/scripts/lib/commands.sh"
set -- "${SCRIPT_ARGS[@]}"
cd "$ROOT"
command="${1:-help}"
shift || true
case "$command" in
  help|--help|-h)
    printf 'Go commands: build, test, vet, fmt, fmt-check, mod-tidy, mod-tidy-check, mod-verify, race, fuzz, coverage\nChecks are read-only except fmt/mod-tidy. Build/coverage write .tmp artifacts. --verbose prints full redacted diagnostics.\n' >&2
    exit 0 ;;
esac
if [[ "${1:-}" == --help ]]; then
  script_error "go.sh $command [tool arguments...]" 'Artifacts: .tmp/bin, .tmp/coverage. fmt/mod-tidy modify source/module files.'
  exit 0
fi
case "$command" in
  build)
    mkdir -p "$ROOT/.tmp/bin"
    run_step 'Go build' go build -o "$ROOT/.tmp/bin/openapi-sdkgen" "$@" ./cmd/openapi-sdkgen ;;
  test|vet|fmt) run_step "Go $command" go "$command" "$@" ./... ;;
  fmt-check)
    files=()
    while IFS= read -r -d '' file; do [[ -f "$file" ]] && files+=("$file"); done < <(git ls-files -co --exclude-standard -z -- '*.go')
    unformatted="$(gofmt -l "${files[@]}")"
    if [[ -n "$unformatted" ]]; then script_error "unformatted Go files:" "$unformatted"; exit 1; fi
    script_note 'ok Go format' ;;
  mod-tidy) run_step 'Go module tidy' go mod tidy "$@" ;;
  mod-tidy-check) run_step 'Go module tidy check' go mod tidy -diff "$@" ;;
  mod-verify) run_step 'Go module verify' go mod verify "$@" ;;
  race) run_step 'Go race detector' go test -race "$@" ./internal/compiler ./internal/output ./scripts/compat-benchmark ;;
  fuzz)
    duration="${FUZZ_TIME:-3s}"
    for target in './internal/diagnostic FuzzSafeSourceDisplay' './internal/output FuzzSafeArtifactPath' './internal/compiler FuzzJSONPointerTokenRoundTrip' './internal/compiler FuzzRemoteReferenceURLSyntax'; do
      read -r package name <<<"$target"
      run_step "$name" go test "$package" -run='^$' -fuzz="^$name$" -fuzztime="$duration"
    done ;;
  coverage)
    mkdir -p "$ROOT/.tmp/coverage"
    run_step 'Go test with coverage' go test -covermode=atomic -coverprofile="$ROOT/.tmp/coverage/go.out" "$@" ./...
    go tool cover -func="$ROOT/.tmp/coverage/go.out" | tail -1 >&2 ;;
  *) script_error "unknown Go command: $command"; exit 2 ;;
esac
