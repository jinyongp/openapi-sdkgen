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
| CI partitioning | implemented and unit-reviewed: Go core, four Go consumer shards, TypeScript generator/native matrix, separate compiler consumers/declarations | core, consumer shard 2/4, compiler 7.0.2 declarations, four inventory tests, shell syntax and actionlint passed; committed-source Actions comparison pending |
| Releaseway refresh | upstream stable releases and migration contracts inspected | actions v0.6.3, npm-actions v0.7.1, Homebrew v0.4.3; source-tag, exact npm tag and diagnostics applicable |
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
