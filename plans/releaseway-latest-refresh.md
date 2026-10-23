# Releaseway refresh — 2026-10-23

## Changes

| Integration | Previous | Selected stable release | Immutable commit |
| --- | --- | --- | --- |
| actions | v0.6.3 | [v0.8.0](https://github.com/releaseway/actions/releases/tag/v0.8.0) | `b6f17f2ffe2c8fd92bdf6be4729e1e56d151bef3` |
| npm-actions | v0.7.1 | [v0.7.5](https://github.com/releaseway/npm-actions/releases/tag/v0.7.5) | `a663ac41be77d1345fd9f4023f11a02e341c62f9` |
| homebrew-actions | v0.4.3 | [v0.5.3](https://github.com/releaseway/homebrew-actions/releases/tag/v0.5.3) | `7e351bd4f3ea1aa0070e4d6556bb5130df2401e6` |

All ten remaining workflow calls use these commits. Official latest-release API
responses and tag commits were checked before editing. The initial worktree was
clean. No package dependency or lockfile changed.

The actions update adds release-set distribution and same-run documentation
baseline support. `.github/distribution/installers.yml` now delegates release
discovery, empty-release handling, required installer pairs, version directories
and snapshot verification to Releaseway. The local API inventory, per-version
matrix, artifact restoration and manual completeness/merge steps were removed.
Policy changes participate in the documentation workflow path filters.

The subsequent v0.8.0 release adds filename shorthand, optional schema selection
and source-located policy errors. The YAML policy now omits `schema` and lists
`install.sh` and `install.ps1` directly. Releaseway normalizes these to required
identity mappings; the consumer adds no parser or compatibility layer. The new
Pages workflow changes input descriptions without changing its calling interface.
npm and Homebrew latest-release/tag identities were rechecked and are unchanged.

Root installers still come from the exact requested release and commit through
the existing single-release action. The release-set policy has no alias, preserving
that choice when rebuilding an older stable release. Canonical `v` SemVer releases,
including prereleases, retain versioned URLs. Historical releases without either
installer are skipped; partial installer sets fail. Documentation-only builds and
the existing stable-only deployment gate remain supported.

The npm update changes upstream release verification, documentation and CI;
its shipped runtime and action inputs are unchanged. The actions publication and
preparation runtime contracts also remain compatible. Their new upstream approval
checks govern Releaseway's own releases; no consumer approval engine is needed.
The repository's schema 1 source-tag release configuration remains valid.

Homebrew adds optional `spec-commit`, packaging revisions, explicit platforms and
native receipt outputs. The existing spec stays compatible: source and spec use
the same commit, revision defaults to zero, and `version_scheme: 1` is retained.
Native receipt generation and aggregation belong to Releaseway's reusable
workflow; no local copy was added.

## Initial validation at actions v0.7.3

- `devtools run test:release`: release dispatch fixtures and CI reuse tests passed.
- `devtools run source-check:npm`: source packaging and ownership checks passed.
- `devtools run validate:docs` and `devtools run format-check:docs`: passed.
- actionlint v1.7.12: all three changed workflows passed.
- Every Releaseway call was checked against the selected action/workflow's input
  declarations, required arguments and exact commit.
- The shipped `dist/distribute.cjs` was exercised against actual public Releases
  and the existing built documentation. All 615 documentation files were preserved
  byte-for-byte, with eight verified installers across v1.3.0, v1.3.1, v1.4.0 and
  v1.4.1. Root scripts remained byte-identical to v1.3.1 despite v1.4.1 being
  available. Releases predating installers were skipped. A separate partial-pair
  fixture failed with `Release is missing required assets`, as required.
  This exercises local preparation, not GitHub artifact transport or Pages hosting.
- The selected Homebrew generator accepted the repository's actual formula spec
  in read-only spec mode. Generated Ruby syntax passed; the platform model retained
  legacy macOS arm64 publication validation and revision zero.
- `git diff --check` passed. Task changes comprise three workflow edits, the new
  distribution policy, the internal script guide and this review record; there
  were no pre-existing changes or staged changes.

## Follow-up validation at actions v0.8.0

The preceding uncommitted refresh was preserved. Seven actions references were
advanced from v0.7.3 to v0.8.0, the YAML policy was shortened to filename strings
without a schema marker, and this record and the script guide were updated.

- Rechecked latest releases for all three integrations and checked all ten calls
  against the selected action/workflow input contracts and immutable pins.
- Release dispatch/CI reuse tests, npm source checks, documentation validation,
  documentation formatting, actionlint and `git diff --check` passed again.
- Ran the actual v0.8.0 `dist/distribute.cjs` against public Releases with the
  repository's shortened YAML policy and the same documentation/root-installer
  baseline used above. The complete output content digest and normalized report
  policy matched the v0.7.3 object-form result exactly. Documentation bytes and
  the requested v1.3.1 root installers remained unchanged.
- A schema-free YAML fixture containing filename strings still failed when one
  required installer was missing, despite `on-empty: skip`.
- Reports remain schema 2 with canonical object mappings. The schema-free input
  contract does not change the report contract or require consumer normalization.

The unchanged npm/Homebrew contracts retain their earlier validation evidence.
No live delivery or native-installation claim is added by these checks.

## Final review

Reviewed all six changed files and the affected publication/documentation callers:
immutable pins and action contracts, job gates and permissions, release/source
identity, ordinary builds with skipped resolution, historical stable rebuilds,
artifact run/attempt identity, release-set selection and missing-file failures,
Homebrew compatibility, and the internal documentation. No runtime defect was
found. Corrected the script guide's obsolete reference to per-version artifacts;
the workflow now uploads one documentation baseline per run/attempt.

The final runtime/configuration matches the v0.8.0 validation above. Those test
results remain applicable; live publication and native platform gaps remain open.

## Remaining gaps and follow-up boundaries

- Actual GitHub Pages deployment, npm publication and native Homebrew installation
  have not been executed for this patch. Those require the normal release/CI
  environment; local checks do not establish their success.
- Existing Homebrew source publication validates only macOS arm64. Linux and Intel
  installation evidence remains absent. If those platforms become part of the
  Homebrew support contract, declare `platforms` in the spec and use Releaseway's
  native matrix; do not implement another local matrix or receipt verifier.
- Release-set range selection excludes noncanonical version tags and build metadata.
  All currently published installer releases use canonical `v` SemVer tags.
  Changing the tag convention would require a policy decision.
- Keep the single-release root installer call: a static `highest-stable` alias
  would change historical-release rebuild behavior. It is not a replacement for
  an exact caller-selected release.

No confirmed compatibility defect remains from the reviewed upstream changes.
No push, tag, publication or deployment was performed. A local commit was
authorized after final review.
