#!/usr/bin/env bash
# Shared script I/O. Execution environment belongs to devtools or the workflow.
set -euo pipefail
umask 077
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
TYPESCRIPT_ROOT="$ROOT/test/typescript"
SCRIPT_DEPTH=$(( ${SCRIPT_DEPTH:-0} + 1 ))
export SCRIPT_DEPTH
SCRIPT_VERBOSE="${SCRIPT_VERBOSE:-0}"
if [[ "$SCRIPT_DEPTH" == 1 ]]; then SCRIPT_VERBOSE=0; fi
SCRIPT_ARGS=()
script_separator=0
for script_arg in "$@"; do
  if [[ "$script_arg" == -- ]]; then script_separator=1; fi
  if [[ "$script_arg" == --verbose && "$script_separator" == 0 ]]; then
    if [[ "$SCRIPT_VERBOSE" != 1 ]]; then
      printf 'warning: --verbose can produce large output; prefer the default command for routine work. Secrets remain redacted.\n' >&2
    fi
    SCRIPT_VERBOSE=1
  else
    SCRIPT_ARGS+=("$script_arg")
  fi
done
export SCRIPT_VERBOSE
mkdir -p "$ROOT/.tmp/runs"
LOG_DIR="$(mktemp -d "$ROOT/.tmp/runs/$(basename "${BASH_SOURCE[1]:-script}").XXXXXX")"
TAIL_LINES=30

script_error() {
  printf '%s\n' "$*" | python3 "$ROOT/scripts/lib/redact.py" >&2
}

script_note() {
  if [[ "$SCRIPT_DEPTH" == 1 || "$SCRIPT_VERBOSE" == 1 ]]; then
    script_error "$*"
  fi
}

script_diagnostic() {
  local log="$1"
  # Prefer actual errors, then include the start and end when a tool has no marker.
  python3 - "$log" <<'PY' >&2
import re, sys
from collections import deque
first, last, errors = [], deque(maxlen=8), deque(maxlen=16)
with open(sys.argv[1], errors='replace') as f:
    for line in f:
        line = re.sub(r'\x1b\[[0-?]*[ -/]*[@-~]', '', line).rstrip()
        if not line: continue
        line = line[:1000]
        if len(first) < 4: first.append(line)
        last.append(line)
        if re.search(r'error|fatal|panic|fail|exception|not found|denied|invalid|timeout', line, re.I): errors.append(line)
lines = list(dict.fromkeys([*first, *errors, *last]))
if lines: print('\n'.join(lines)[:12000])
PY
}

script_step() {
  local mode="$1" label="$2"
  shift 2
  local stage out_fd err_fd out_pid err_pid status=0
  stage="$(mktemp -d "$LOG_DIR/step.XXXXXX")"
  printf '%s\n' "$label" | python3 "$ROOT/scripts/lib/redact.py" >"$stage/label"
  exec {out_fd}> >(python3 "$ROOT/scripts/lib/redact.py" >"$stage/stdout.log")
  out_pid=$!
  exec {err_fd}> >(python3 "$ROOT/scripts/lib/redact.py" >"$stage/stderr.log")
  err_pid=$!
  "$@" >&$out_fd 2>&$err_fd || status=$?
  exec {out_fd}>&-
  exec {err_fd}>&-
  wait "$out_pid" || status=1
  wait "$err_pid" || status=1
  if [[ "$mode" == data ]]; then cat "$stage/stdout.log"; fi
  if [[ "$SCRIPT_VERBOSE" == 1 ]]; then
    if [[ "$mode" != data ]]; then cat "$stage/stdout.log" >&2; fi
    cat "$stage/stderr.log" >&2
  fi
  if ((status)); then
    printf 'failed (%s): ' "$status" >&2
    cat "$stage/label" >&2
    if [[ "$SCRIPT_VERBOSE" != 1 ]]; then
      script_diagnostic "$stage/stderr.log"
      script_diagnostic "$stage/stdout.log"
    fi
    printf 'diagnostics: %s\n' "$stage" >&2
    return "$status"
  fi
  if [[ "$mode" != data || "$SCRIPT_VERBOSE" == 1 ]]; then script_note "ok $label"; fi
}

run_step() { script_step summary "$@"; }
run_data() { script_step data "$@"; }

# Servers stream their readiness message; logs are still redacted before storage.
run_live() {
  local mode label stage out_fd err_fd out_pid err_pid grouped child status
  mode="$1"; label="$2"; shift 2
  stage="$(mktemp -d "$LOG_DIR/live.XXXXXX")"
  if [[ "$mode" == data ]]; then
    exec {out_fd}> >(python3 -u "$ROOT/scripts/lib/redact.py" | tee "$stage/stdout.log")
  elif [[ "$SCRIPT_VERBOSE" == 1 ]]; then
    exec {out_fd}> >(python3 -u "$ROOT/scripts/lib/redact.py" | tee "$stage/stdout.log" >&2)
  else
    exec {out_fd}> >(python3 -u "$ROOT/scripts/lib/redact.py" | tee "$stage/stdout.log" | { grep -Ei --line-buffered 'listening|ready in|Local:' || [[ $? == 1 ]]; } >&2)
  fi
  out_pid=$!
  if [[ "$SCRIPT_VERBOSE" == 1 ]]; then
    exec {err_fd}> >(python3 -u "$ROOT/scripts/lib/redact.py" | tee "$stage/stderr.log" >&2)
  else
    exec {err_fd}> >(python3 -u "$ROOT/scripts/lib/redact.py" >"$stage/stderr.log")
  fi
  err_pid=$!
  grouped=0
  if command -v setsid >/dev/null 2>&1; then setsid "$@" >&$out_fd 2>&$err_fd & grouped=1
  else "$@" >&$out_fd 2>&$err_fd & fi
  child=$!
  stop() {
    if [[ "$grouped" == 1 ]]; then kill -TERM -- "-$child" 2>/dev/null || true
    else kill -TERM "$child" 2>/dev/null || true; fi
    for _ in {1..30}; do
      kill -0 "$child" 2>/dev/null || break
      sleep 0.1
    done
    if [[ "$grouped" == 1 ]]; then kill -KILL -- "-$child" 2>/dev/null || true
    else kill -KILL "$child" 2>/dev/null || true; fi
    wait "$child" 2>/dev/null || true
  }
  trap 'stop; exit 129' HUP
  trap 'stop; exit 130' INT
  trap 'stop; exit 143' TERM
  status=0
  wait "$child" || status=$?
  exec {out_fd}>&-
  exec {err_fd}>&-
  wait "$out_pid" || status=1
  wait "$err_pid" || status=1
  if ((status)); then
    script_error "failed ($status): $label; diagnostics: $stage"
    if [[ "$SCRIPT_VERBOSE" != 1 ]]; then
      script_diagnostic "$stage/stderr.log"
      script_diagnostic "$stage/stdout.log"
    fi
  fi
  trap - HUP INT TERM
  return "$status"
}

require_system_node() {
  local expected actual
  expected="$(tr -d '[:space:]' < "$ROOT/.node-version")"
  actual="$(node --version 2>/dev/null || true)"
  if [[ "$actual" != "v$expected" ]]; then
    printf 'Node %s required; active version: %s. Select the project toolchain before running this script.\n' "$expected" "${actual:-missing}" >&2
    return 1
  fi
}
ts_node() { require_system_node || return $?; (cd "$TYPESCRIPT_ROOT" && "$@"); }
ts_pnpm() {
  require_system_node || return $?
  (cd "$TYPESCRIPT_ROOT" && corepack pnpm "$@")
}
