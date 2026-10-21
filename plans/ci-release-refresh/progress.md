# CI and Releaseway refresh

Current version decision: the next release is **1.3.1**, explicitly selected by
the maintainer after the automatic 1.4.0 plans below. `bump: patch` overrides
Conventional Commit inference for this upcoming release; restore automatic bumps
after it. Historical plans remain unchanged. Publication is still excluded.

Scope: shorten independent CI feedback without dropping checks; update the three
Releaseway integrations and adopt applicable upstream changes. Work directly on
`main`, preserve commit history without merge commits, review each unit and the
combined result, and commit. Publication is explicitly excluded.

The earlier release request run `39526581214` was cancelled before tag creation
or any publishing job. Read-only planning selected v1.4.0; that version was not
published. Current public release remains v1.3.0. PR #22 was closed unmerged;
`main` advanced by fast-forward to `e003236`.

Baseline: source `e003236`, Actions CI run `39524695228` on ubuntu-24.04,
Go 1.27.2. Go's suite step ran 16:52 (01:00:35–01:17:27 UTC); its complete job
took 17:22. TypeScript runtime took 4:03, tools 1:20, compiler consumer steps
between 2:04 and 4:47. These are observed Actions durations, not local timings.

| Unit | State | Validation |
| --- | --- | --- |
| CI partitioning | complete: four Go consumer shards, two TypeScript generator shards and independent compiler/runtime groups | source `f0e4a7d`, run `39529704679` passed all 22 jobs; exact prior inventories retained, all 157 Go and 515 TypeScript generator tests passed |
| Releaseway refresh | complete: actions v0.6.3, npm-actions v0.7.1, Homebrew v0.4.3; source-tag plan/resume, exact npm tag and operation diagnostics | pinned releases/contracts, mocked dispatch/CI-reuse tests, actionlint and actual read-only plan run `39528351091` passed; all tag/publication jobs skipped |
| Combined review | complete; no additional eligible defect found | coverage union, failure propagation, gating, retries, source/tag identity, diagnostics and read-only release validation reviewed |

Runner setup and compilation may repeat across jobs. The measured feedback times
below retain their actual shard inventories and runner/cache limitations. No live
release or package publication is authorized.

The actual Go target inventory contains 157 top-level runnable tests; generated
test source embedded in fixtures is not mistaken for repository tests. Shards
include fuzz seed/example entries if discovered. Their final gate checks missing,
duplicate, changed and unexecuted inventories. Each measured Go CI group executes
tests with `-count=1` while retaining build caches. Local shard 1 completed all
40 selected tests, but its initial shell wrapper failed after that script was
edited while running; that invocation is not a passing command. The stable wrapper
and full second shard subsequently passed. No SDK source changed.

## First Actions measurement and follow-up

The second same-source baseline, run `39526526870`, completed its Go job in 12:39
(test suite step 12:18). The first baseline's 17:22 and this 12:39 show real runner
and cache variability. Both use the same SDK source and pinned Go 1.27.2.

At `e00fd40`, run `39528328681` passed all 21 jobs. Its longest Go shard job took
5:47; Go core took 1:44. The complete CI jobs finished in 6:57 from first job start
to the source gate. All 157 Go target tests were accounted for by the gate. Raw
timing artifacts were downloaded under `.tmp/ci-release-refresh/39528328681`.
These are observed Actions feedback times, not a normalized compiler benchmark.

The remaining longest job was TypeScript generator tests: 6:47 for 515 top-level
tests. The follow-up applies the same two-way partitioning to those tests and
checks both inventories at the final gate. The native/declaration matrix tests
retain their independent owners; no generated SDK checks are omitted.

The new Releaseway read-only plan is schema 2, strategy `source-tag`, version
1.4.0, and source/commit both `e00fd405949b0fc27c64f1288da3e62f365e27be`.
The plan includes the policy digest and proposed tag object identity, but no
remote v1.4.0 tag exists. GitHub and npm still report 1.3.0. Source-tag creation,
package publication and live delivery remain intentionally unexecuted.

## Completed combined validation

[Final configuration CI run 39529704679](https://github.com/jinyongp/openapi-sdkgen/actions/runs/39529704679)
passed all 22 jobs at `f0e4a7ded20fdcc02962d3db50973a073ca7fc3e`.
The source gate verified four Go inventories and two TypeScript inventories.
An independent comparison with the first Actions artifacts confirmed the exact
same 157 Go and 515 TypeScript generator test identities, without duplicates;
all 672 top-level outcomes were `pass`. Timings and inventories remain under
`.tmp/ci-release-refresh/39529704679` and in that run's artifacts.

| Observed Actions interval | Before | First partition | Final partition |
| --- | --- | --- | --- |
| Longest Go job | 17:22 / 12:39 | 5:47 | 5:57 |
| TypeScript generator job | part of the Go job | 6:47 | 5:14 (longest of two) |
| First CI job start to completed source gate | 17:27 / 12:44 | 6:57 | 6:08 |

Baseline runs are `39524695228` and `39526526870`; first partition run is
`39528328681`. All use ubuntu-24.04 and Go 1.27.2 with unchanged SDK source.
Runner/cache conditions vary, so these are feedback-time observations rather
than normalized speedup ratios. The two Go partition runs give repeated evidence
for its shorter critical path. Final runtime/native-matrix and compiler groups
also passed; no additional split is needed to shorten this measured critical path.
Individual full-provider compatibility compilation budgets are unchanged.

Final review covered every changed workflow/script and their callers: discovered
test selection, local aggregate ownership, fuzz/example inclusion, missing/overlap
and completion rejection, final matrix-result gating, and report replacement on
retry. Core Go, representative consumer/declaration groups, five partition/coverage
tests, shell syntax, actionlint 1.7.12 and mocked release/CI-reuse tests passed.
The official source-tag plan/resume contract replaces the deleted local converter;
annotated tag commit identity, exact npm tag and immutable plan artifact/digest
bindings were reviewed. Source-tag creation and delivery are excluded by the user's
explicit no-publication instruction; only the real read-only plan was exercised.

The final record changes no SDK, test runner, workflow or release configuration.
No pre-existing user edits were present. Unit commits were made directly on `main`
and pushed without a PR or merge commit. `git diff --check` passed, and the final
delivery includes only this completion record after the verified configuration.
