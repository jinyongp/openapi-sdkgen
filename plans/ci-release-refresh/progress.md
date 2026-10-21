# CI and Releaseway refresh

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
| CI partitioning | initial unit passed all 21 Actions jobs; follow-up splits the observed TypeScript generator bottleneck into two shards | source `e00fd40`, run `39528328681`; Go inventory 157/157 verified; updated gate tests, exact previous 515-test TypeScript inventory comparison, first TypeScript shard, shell syntax and actionlint passed; full follow-up Actions run pending |
| Releaseway refresh | complete: actions v0.6.3, npm-actions v0.7.1, Homebrew v0.4.3; source-tag plan/resume, exact npm tag and operation diagnostics | pinned releases/contracts, mocked dispatch/CI-reuse tests, actionlint and actual read-only plan run `39528351091` passed; all tag/publication jobs skipped |
| Combined review | pending | coverage union, failure propagation, gating, source/tag identity, read-only release validation |

Runner setup and compilation may repeat across jobs. No timing improvement is
claimed until the new Actions critical path is observed; retain actual shard
inventories and timings. No live release or package publication is authorized.

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
