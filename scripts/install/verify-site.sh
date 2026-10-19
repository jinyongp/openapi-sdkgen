#!/usr/bin/env bash
set -euo pipefail
source "$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)/scripts/lib/commands.sh"
set -- "${SCRIPT_ARGS[@]}"
if [[ "${1:-}" == --help ]]; then
  printf '%s\n' 'verify-site.sh: verifies deployed install.sh/install.ps1 using PAGE_URL, SHELL_SHA and POWERSHELL_SHA; read-only HTTP checks with bounded retries.' >&2
  exit 0
fi
[[ "${PAGE_URL:-}" == https://* && "${SHELL_SHA:-}" =~ ^[0-9a-fA-F]{64}$ && "${POWERSHELL_SHA:-}" =~ ^[0-9a-fA-F]{64}$ ]] || { script_error 'Expected HTTPS PAGE_URL and both installer SHA-256 hashes'; exit 2; }
if command -v sha256sum >/dev/null 2>&1; then
  checksum_command=(sha256sum)
elif command -v shasum >/dev/null 2>&1; then
  checksum_command=(shasum -a 256)
else
  script_error 'sha256sum or shasum is required'
  exit 2
fi
scratch="$(mktemp -d "$LOG_DIR/site.XXXXXX")"
trap 'rm -rf "$scratch"' EXIT
verify_file() {
  local actual
  actual="$("${checksum_command[@]}" "$1")" || return
  actual="${actual%% *}"
  if [[ "${actual,,}" != "${2,,}" ]]; then
    script_error "Checksum mismatch for $(basename "$1")"
    return 1
  fi
}
verify_site() {
  local attempt
  for attempt in {1..6}; do
    if curl -fsSL --proto '=https' --proto-redir '=https' --connect-timeout 10 --max-time 20 --output "$scratch/install.sh" "${PAGE_URL%/}/install.sh" &&
       curl -fsSL --proto '=https' --proto-redir '=https' --connect-timeout 10 --max-time 20 --output "$scratch/install.ps1" "${PAGE_URL%/}/install.ps1" &&
       verify_file "$scratch/install.sh" "$SHELL_SHA" && verify_file "$scratch/install.ps1" "$POWERSHELL_SHA"; then
      return 0
    fi
    if ((attempt < 6)); then sleep 5; fi
  done
  script_error "Deployed installers do not match the release at $PAGE_URL. Check the Pages deployment and rerun verification."
  return 1
}
run_step 'deployed installer bytes match the release' verify_site
