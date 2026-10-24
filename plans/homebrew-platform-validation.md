# Homebrew native validation — 2026-10-24

## Contract and implementation

Declare `macos-arm64`, `macos-x86_64`, `linux-arm64` and `linux-x86_64` in
`.github/homebrew/formula.yml`. The pinned Homebrew Actions v0.5.3 commit
`7e351bd4f3ea1aa0070e4d6556bb5130df2401e6` owns the native runner matrix,
runtime identity checks, Formula byte checks and receipt aggregation. No local
matrix or receipt parser is added.

Preserve source installation, the Go dependency, `version_scheme: 1`, version/help
tests and the existing macOS arm64 post-publication Go consumer check. npm keeps
automatic packing. Packaging revisions and separate spec/source commits remain
outside this change.

## Review coverage

| Area | State | Evidence or remaining work |
| --- | --- | --- |
| Product spec and source build contract | Reviewed | Only four platform IDs added; existing install/test contract retained. |
| PR caller | Reviewed | `check.yml` selects spec mode: Ubuntu render/audit, no native installation. |
| Release caller | Reviewed | Existing exact source/version inputs feed upstream release mode and its derived native matrix. |
| Native identity and Formula binding | Reviewed | Upstream checks OS, CPU, Homebrew process and Formula digest before installation. |
| Failure and publication ordering | Reviewed | Upstream requires audit/install/test receipts for all declared targets before tap push. |
| Generated Formula and matrix | Reviewed | Pinned renderer produced identical Formula bytes in spec and release modes, with the expected mode-specific matrix. |
| Documentation and final diff | Reviewed | Updated script guide and superseded arm64-only follow-up; final diff and callers rechecked. |
| Four-platform hosted installation | Pending | Requires a release workflow run containing this spec. |

## Local validation and review loop

The actual product spec before/after the declaration was rendered by the pinned
upstream `internal/formula/render.rb`. Both spec and release modes produced
byte-identical Formula files for the same published v1.4.1 source commit. Release
mode downloaded the real source to calculate its checksum. Ruby syntax checks
passed for both baseline and selected output in each mode. Spec mode retained
Ubuntu-only audit; release mode selected the four expected native runners.
Generated context retained version scheme 1 and revision zero.

The installed Homebrew loader parsed the selected product Formula under all four
simulated OS/architecture conditions. Source URL, version, version scheme, Go build
dependency and absence of extra host requirements all matched expectations. This
is parsing/requirements evidence, not native installation. Direct arbitrary-path
loading was initially rejected by Homebrew's tap requirement; the final check used
the pinned upstream test's `Formulary.from_contents` approach without registering
or changing a tap.

Review pass one covered the product spec, PR/release callers, upstream identity
and receipt gates, publication ordering and docs. Corrected the earlier review
record's now-stale arm64-only follow-up. Pass two rechecked the final spec and
documentation, unchanged callers and validation output; no in-scope defects remain.
Actionlint v1.7.12 passed for the release, docs and Homebrew workflows, and
`git diff --check` passed. Native hosted execution remains an explicit pending gate.

## Execution and recovery

PR audit success is a merge check; it does not establish native installation.
The first release containing this spec must pass all four native audit/install/test
jobs and aggregate their matching receipts before Homebrew publication is complete.
Each native job has an upstream 45-minute timeout. Keep the run URL, source SHA,
Formula SHA and aggregate artifact ID when hosted execution occurs.

Intel macOS is currently Homebrew Tier 3. Homebrew Go 1.27.3 has bottles for Apple
Silicon and both Linux architectures, but none for Intel macOS. Source-building
the Go dependency may affect Intel validation duration; this has not been measured.
Do not skip a failed target or describe cross-compilation as installation evidence.

If native validation fails, the tap remains unchanged. GitHub/npm publication can
already have succeeded because those jobs are independent. The current release
workflow waits for Homebrew verification before publishing matching documentation.
Retry transient failures at the same tag. A source/spec fix requires a new release
under the retained same-source/spec policy; editing main does not change an old tag.

## Sources

- [Pinned platform model](https://github.com/releaseway/homebrew-actions/blob/7e351bd4f3ea1aa0070e4d6556bb5130df2401e6/internal/formula/platforms.rb)
- [Pinned publish workflow](https://github.com/releaseway/homebrew-actions/blob/7e351bd4f3ea1aa0070e4d6556bb5130df2401e6/.github/workflows/publish.yml)
- [Pinned spec/release validation](https://github.com/releaseway/homebrew-actions/blob/7e351bd4f3ea1aa0070e4d6556bb5130df2401e6/internal/scripts/validate-formula.sh)
- [Homebrew support tiers](https://docs.brew.sh/Support-Tiers)
- [Homebrew Go metadata](https://formulae.brew.sh/api/formula/go.json)
