#!/usr/bin/env bash
set -euo pipefail
source "$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)/scripts/lib/commands.sh"
set -- "${SCRIPT_ARGS[@]}"
if [[ "${1:-}" == --help || "${2:-}" == --help ]]; then
  printf '%s\n' 'audit.sh' 'May install pinned scanners and query vulnerability databases. Reads tracked dependency inventories; exit 0=clean, 1=findings, 2=incomplete.' >&2
  exit 0
fi

GOVULNCHECK_VERSION="v1.8.0"
OSV_SCANNER_VERSION="v2.6.0"

work_root="$(mktemp -d "$ROOT/.tmp/vuln.XXXXXX")"
cleanup() {
  rm -rf "$work_root"
}
trap cleanup EXIT

install_tool() {
  local label="$1"
  local package="$2"
  local version="$3"
  local output="$4"
  local override="$5"

  if [[ -n "$override" ]]; then
    if [[ ! -x "$override" ]]; then
      echo "vulnerability audit setup failed: $label override is not executable" >&2
      return 2
    fi
    printf '%s\n' "$override"
    return 0
  fi

  if [[ ! -x "$output" ]]; then
    local log="$work_root/install-$label.log"
    mkdir -p "$(dirname "$output")"
    if ! GOBIN="$(dirname "$output")" run_step "install $label" go install "$package@$version" >"$log" 2>&1; then
      echo "vulnerability audit setup failed: could not install $label $version" >&2
      if [[ "$SCRIPT_VERBOSE" == 1 ]]; then cat "$log" >&2; else script_diagnostic "$log"; fi
      return 2
    fi
    if [[ "$SCRIPT_VERBOSE" == 1 ]]; then cat "$log" >&2; fi
  fi
  printf '%s\n' "$output"
}

tool_root="$ROOT/.tmp/security-tools"
govulncheck_bin="$(
  install_tool \
    govulncheck \
    golang.org/x/vuln/cmd/govulncheck \
    "$GOVULNCHECK_VERSION" \
    "$tool_root/govulncheck-$GOVULNCHECK_VERSION/govulncheck" \
    "${OPENAPI_SDKGEN_TEST_GOVULNCHECK_BIN:-}"
)" || exit $?
osv_scanner_bin="$(
  install_tool \
    osv-scanner \
    github.com/google/osv-scanner/v2/cmd/osv-scanner \
    "$OSV_SCANNER_VERSION" \
    "$tool_root/osv-scanner-$OSV_SCANNER_VERSION/osv-scanner" \
    "${OPENAPI_SDKGEN_TEST_OSV_SCANNER_BIN:-}"
)" || exit $?

findings=0
audit_failed=0

gov_sarif="$work_root/govulncheck.sarif"
gov_stderr="$work_root/govulncheck.stderr"
if ! run_data "govulncheck" "$govulncheck_bin" -format sarif ./... >"$gov_sarif" 2>"$gov_stderr"; then
  echo "vulnerability audit incomplete: govulncheck execution failed" >&2
  script_diagnostic "$gov_stderr"
  audit_failed=1
else
  set +e
  node "$ROOT/scripts/security/sarif-report.mjs" "$gov_sarif"
  gov_status=$?
  set -e
  case "$gov_status" in
    0) ;;
    1) findings=1 ;;
    *)
      echo "vulnerability audit incomplete: govulncheck report could not be interpreted" >&2
      audit_failed=1
      ;;
  esac
fi

if [[ "$SCRIPT_VERBOSE" == 1 ]]; then cat "$gov_sarif" "$gov_stderr" >&2; fi

mapfile -t pnpm_locks < <(git -C "$ROOT" ls-files '*pnpm-lock.yaml')
if ((${#pnpm_locks[@]} == 0)); then
  echo "vulnerability audit incomplete: no tracked pnpm lockfiles found" >&2
  audit_failed=1
else
  osv_args=(scan source --format=vertical --verbosity=error -L "$ROOT/go.mod")
  for lockfile in "${pnpm_locks[@]}"; do
    osv_args+=(-L "$ROOT/$lockfile")
  done
  osv_stdout="$work_root/osv.stdout"
  osv_stderr="$work_root/osv.stderr"
  set +e
  run_data "osv-scanner" "$osv_scanner_bin" "${osv_args[@]}" >"$osv_stdout" 2>"$osv_stderr"
  osv_status=$?
  set -e
  if [[ "$SCRIPT_VERBOSE" == 1 ]]; then cat "$osv_stdout" "$osv_stderr" >&2; fi
  case "$osv_status" in
    0)
      script_note "ok osv-scanner: no known vulnerabilities in tracked dependency inventories"
      ;;
    1)
      echo "osv-scanner found known dependency vulnerabilities:" >&2
      script_diagnostic "$osv_stdout"
      script_diagnostic "$osv_stderr"
      findings=1
      ;;
    *)
      echo "vulnerability audit incomplete: osv-scanner failed with exit code $osv_status" >&2
      script_diagnostic "$osv_stderr"
      audit_failed=1
      ;;
  esac
fi

if ((audit_failed != 0)); then
  exit 2
fi
if ((findings != 0)); then
  exit 1
fi

script_note "ok vulnerability audit"
