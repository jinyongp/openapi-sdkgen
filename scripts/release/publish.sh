#!/usr/bin/env bash
set -euo pipefail
source "$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)/scripts/lib/commands.sh"
set -- "${SCRIPT_ARGS[@]}"

mode=plan
ref=main
prerelease=""
dry_run=0
while (($#)); do
  case "$1" in
    --help|-h)
      printf '%s\n' 'publish.sh [--dry-run | --yes] [--prerelease ID] [--resume TAG]' \
        'Requests the Releaseway workflow for remote main. Defaults to a checked release plan; --yes prepares and publishes. --resume runs the workflow at an existing tag. Local edits and commits are not pushed.' >&2
      exit 0 ;;
    --dry-run|-n) dry_run=1 ;;
    --yes|-y) mode=release ;;
    --prerelease|--resume)
      if (($# < 2)) || [[ -z "$2" || "$2" == -* ]]; then
        script_error "$1 requires a value"; exit 2
      fi
      if [[ "$1" == --prerelease ]]; then
        prerelease="$2"
      else
        ref="$2"
        mode=release
        if [[ "$ref" != v* ]]; then
          script_error '--resume requires a release tag starting with v'; exit 2
        fi
      fi
      shift ;;
    --) ;;
    *) script_error "Unknown release argument: $1. Version policy lives in .github/releaseway.yml."; exit 2 ;;
  esac
  shift
done
if ((dry_run)) && [[ "$mode" == release ]]; then
  script_error '--dry-run cannot be combined with --yes or --resume'; exit 2
fi
if [[ "$ref" != main && -n "$prerelease" ]]; then
  script_error '--resume cannot be combined with --prerelease'; exit 2
fi
cd "$ROOT"
dispatch=(gh workflow run release.yml --ref "$ref")
if [[ "$ref" != main ]]; then
  dispatch+=(-f "tag=$ref")
else
  dispatch+=(-f "mode=$mode" -f "prerelease-id=$prerelease")
fi
run_step "request release $mode on $ref" "${dispatch[@]}"
script_note 'Releaseway runs checks and release operations in GitHub Actions. Inspect the release workflow for results.'
