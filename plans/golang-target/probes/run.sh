#!/usr/bin/env bash
set -euo pipefail
source "$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd)/scripts/lib/commands.sh"
run_step 'Execute and bind Go design evidence' python3 -B "$ROOT/plans/golang-target/record_evidence.py"
run_step 'Verify plan and executed evidence' python3 -B "$ROOT/plans/golang-target/verify_plan.py"
