# Preimplementation follow-up: selected scoped construction design

This report supersedes the **adoption hold** in RESULTS.md for the old all-schema,
per-property-definition helper. RESULTS.md remains the historical evidence for
that rejected version. Production optimizer code is still unchanged.

## Reproducible candidate

Baseline revision: `883c41bd1f39ce4d2cac92ca852dc881b6c29b68`.
Baseline binary SHA-256: `66d280ee676713aceed81ae5d4a7a1ee6bf554070531932a2d1ae6f348e2b42d`.
Selected prototype version: `0.0.0-verification.facade2`.
Candidate binary SHA-256: `64a02582d8a82258c5a6ca71b8ab42133de303166eb91eb107e7ab55b5e830a7`.

`evidence/candidate-v43.json` preserves the eleven changed prototype source/test
files, their original checksums and candidate contents. It is an audit snapshot,
not a production patch or an automatic installer. The final semantic-section,
Link reference and complete Go allocator work has not been substituted by the
prototype's minimal fixed-name switch.

`evidence/preimplementation-v43.json` preserves the selected input/tool/binary
identities, raw paired cost samples, runtime summaries, constructor tests,
bundles, declaration results and migration outcomes. Full per-command logs and
transformed output remain in the referenced `.tmp` runs; essential evidence does
not depend exclusively on those temporary directories.

## Design selected by the experiments

The earlier defineOwnDataProperty-per-entry helper and indiscriminate helper
imports regressed unbundled initialization. A native Object.fromEntries helper
alone did not remove the import regression. Further output experiments isolated
construction mechanics from schema ownership and the extra operation import edge.

The selected real-generator prototype therefore uses:

1. Fixed operation-local type names such as `__sdkgen_Input` and `__sdkgen_Output`.
2. **Named schema modules:** keep explicit property wrapper literals and their
   original runtime dependency graph. Add an explicit `WireProperty` type
   argument to `Object.fromEntries`, with type-only imports, to bound inference.
3. **Inline operation/server schemas:** emit exact keys and child schema objects
   as two homogeneous arrays from one ordered producer list. A concrete helper
   checks equal lengths, constructs typed entries with an indexed loop and calls
   native `Object.fromEntries`. It returns the original readable descriptor shape.
4. **Ownership and imports:** the helper implementation stays in a tiny
   `wire-properties` module. The existing `callables` module re-exports that same
   function as `createWireProperties`; operations reuse their existing callables
   import instead of adding another runtime import per operation. The distinct
   facade name preserves the existing single-name runtime ownership invariant.
   No wrapper function or root public export is added. Server modules may import
   the helper directly.

This is an artifact-owner/lifetime rule, not an API-provider or property-count
threshold. It does not hoist per-client metadata, intern schemas, add a packed
runtime ABI, normalize user keys, alter public contexts or rebuild descriptors
on the request/response/stream-frame path.

## Completed verification

- 25 harness self-tests passed, including primitive root mismatch, deliberate
  key/prototype/name/alias corruption, exact export-addition permissions, manifest
  versions and symlink containment.
- Nine positive local fixtures plus GitHub and Stripe: unminified/unbundled
  contracts, all existing runtime value export names and callable reflection
  passed. Each case used ten A/A and twenty A/B fresh-process timing pairs.
- Four lifetime profiles (representation, inline lifecycle, GitHub, Stripe):
  three baseline/candidate pairs; 240 clients discarded per process; no flagged
  late retained-heap growth or cross-client configuration mixing. Helper
  invocations during measured requests/stream frames were zero. The inline
  lifecycle fixture required positive construction counts to prevent a vacuous
  zero-call result.
- All eleven generated source consumers, declaration emissions and downstream
  declaration consumers passed. GitHub/Stripe whole-source strict checks passed.
- Actual emitted helper tests passed eleven focused behaviors, 128 adversarial
  key vectors and the mismatched two-array length checks.
- Final-binary migration: three fixtures, eight checks each, all passed. Covered
  baseline identity invalidation, fresh/incremental byte equivalence, no-op
  nanosecond mtimes/content, diagnostic failure preserving old output, user-edit
  refusal, and unmanaged file/directory/symlink conflicts at the new helper path.
- Candidate Go tests and TypeScript runtime/conformance passed after restoring
  the snapshot's required documentation/package test context. Tests were not
  removed to make an incomplete snapshot appear valid. Runtime module inventory
  and JSDoc expectations were updated for the deliberate new helper/facade.

These are preimplementation/prototype checks. They do not mark the complete
97-row final-production matrix done.

## Strict baseline failures are not hidden

`collisions`, `transport-native-headers`, `baseline-oas32` and `lifecycle` each
expose an existing whole-source strict problem: stream-only public Call types
have no buffered-call signature, but `assignCallableProperties` requires a
callable constraint. Baseline and candidate produce matching TS2345 diagnostics.

The artifact report remains **review / exit 2**, not pass. Original diagnostic
messages are retained; only directory locations, positions and private spellings
are normalized for the differential comparison. New candidate diagnostics fail.
Declaration consumers use the source as shipped, separately from strict copies;
no strict check is replaced with declaration checking or a weaker compiler flag.

The earlier ordinary-empty-map Webhook `template.startsWith` issue also remains
an explicitly recorded baseline bug. Both issues should receive independent
correctness fixes; neither should be disguised as an optimizer regression or
accepted as a green final strict suite. Do not expose a buffered call on a
stream-only operation merely to satisfy an internal generic constraint.

## Final candidate measurements

Local pinned inputs, Node 24.21.0. Generation/compiler values are medians of ten
interleaved independent pairs, with per-child GNU time CPU/RSS. Network and
binary-build time are excluded; these are not cold-filesystem measurements.
Median-of-paired changes and ratios of separate medians are different statistics;
the raw report records both and they must not be substituted for each other.

| Metric                                |   GitHub baseline -> candidate |   Stripe baseline -> candidate |
| ------------------------------------- | -----------------------------: | -----------------------------: |
| Managed TypeScript bytes              |       42,752,469 -> 37,157,721 |       32,083,701 -> 29,150,527 |
| Declaration bytes                     |       25,148,686 -> 19,567,347 |       18,962,839 -> 16,423,802 |
| Minified full client bytes            |         2,583,041 -> 2,489,155 |         3,592,758 -> 3,114,441 |
| gzip bytes                            |             217,267 -> 212,187 |             319,393 -> 287,243 |
| Brotli bytes                          |             140,153 -> 140,602 |             159,833 -> 161,320 |
| Fresh CLI wall ms                     |         1,299.746 -> 1,255.861 |           1,035.570 -> 972.467 |
| Fresh CLI CPU ms                      |                 2,205 -> 2,115 |                 1,980 -> 1,880 |
| Fresh CLI peak RSS bytes              |     243,767,296 -> 241,559,552 |     167,086,080 -> 160,192,512 |
| Strict compiler wall ms               |         3,169.889 -> 2,523.603 |         3,424.939 -> 2,111.052 |
| Strict compiler peak RSS bytes        | 1,987,401,728 -> 1,439,395,840 | 2,535,927,808 -> 1,254,871,040 |
| Normal source consumer wall ms        |             397.808 -> 379.166 |             380.970 -> 366.830 |
| Normal source consumer peak RSS bytes |     490,489,856 -> 483,049,472 |     458,842,112 -> 432,640,000 |

The declaration byte values in the machine-readable evidence are authoritative
if this human summary is subsequently reformatted or extended.

Unbundled runtime values below are medians from twenty A/B pairs, with ten A/A
controls. No forced GC or reflection occurs inside the timed first-client interval.

| Runtime metric                         | GitHub baseline -> candidate ms | Stripe baseline -> candidate ms |
| -------------------------------------- | ------------------------------: | ------------------------------: |
| Import                                 |              296.536 -> 291.614 |              262.448 -> 264.111 |
| First createClient                     |                40.195 -> 38.239 |                46.440 -> 40.315 |
| Contiguous import-through-first-client |              336.294 -> 329.037 |              309.730 -> 304.867 |
| Repeated createClient                  |                  5.649 -> 5.489 |                  8.822 -> 8.670 |

No selected-candidate timing crossed the predeclared review policy. This does
not prove every tiny delta is a statistically significant speedup. Old Vite
bundled import medians must not be mixed with these unbundled ESM measurements.

## Nonzero costs and rejected claims

The scoped constructor retains less compressed-size improvement than the earlier
all-schema helper. In exchange, its unbundled initialization did not reproduce
that helper's significant Stripe regression. The strongest measured gains are
source/declaration volume and full strict compiler cost, not a claim that every
startup or generator metric improves dramatically.

Brotli grew by 449 bytes on GitHub and 1,487 bytes on Stripe. Small full-client
fixtures using the helper grew by 51, 68, 79 or 80 gzip bytes; other local fixture
bundles were unchanged. These are explicit fixed-cost tradeoffs, not a universal
no-bundle-regression PASS or a retrospectively changed threshold. The design
recommendation accepts these small size costs while preserving exact semantics
and the measured large-corpus benefits. Final production approval still requires
reviewing its actual deltas against the recorded policy.

Standalone named schema leaves retain their literal representation and do not
acquire the helper's runtime dependency. Lexical-name changes alone do not solve
endpoint tree shaking, and this work does not implement endpoint selection.

## Evidence locations and integration closure

- Runtime/contracts: `.tmp/preimplementation/run-nWuLsA/report.json`.
- Artifact/strict/declaration/bundle: `.tmp/preimplementation/run-nWuLsA/artifacts-8d4Wio/report.json`.
- CPU/wall/RSS: `.tmp/representation-measure/run-NXbFkP/report.json`.
- Migration: `.tmp/representation-migration/run-eiS4fd/report.json`.
- Durable candidate sources and result data: `evidence/candidate-v43.json` and
  `evidence/preimplementation-v43.json`.

Final candidate `just agent perf-acceptance` and main-worktree verification-infrastructure
`just agent ci` both exited 0 in the closure run. The latter reported Go coverage
82.7% against the 78% threshold. These are separate commands with separate scopes:
this does not claim a full candidate CI run or completion of the production optimizer.
The production Go allocator/semantic-reference integration remains an implementation
work item. Detailed closure evidence follows below.

Declaration-accounting correction: raw artifact reports also emitted a probe consumer.d.ts. Durable evidence now records both the original total and the manifest-owned declaration count; only the latter appears in size tables. This is an aggregation correction, not a new compiler run or a change to archived timings. A negative control prevents future auxiliary-file inclusion.

## Verification closure — 2026-09-25

The v4.3 selected candidate was independently checked again without altering the
production generator or runtime. All eleven live candidate source/test files and
the baseline/candidate binaries matched the frozen SHA-256 records. The earlier
runtime, artifact, cost and migration report hashes also matched their retained
records. Node 24.21.0 was used for the execution comparisons.

| Closure check                               | Result                      | Evidence scope                                                                                                                   |
| ------------------------------------------- | --------------------------- | -------------------------------------------------------------------------------------------------------------------------------- |
| Current verification-infrastructure CI      | pass / exit 0               | Main worktree, all standard checks; Go coverage 82.7%                                                                            |
| Selected candidate performance acceptance   | pass / exit 0               | Existing compile/prepare/emit/publication/process/stream gate, candidate snapshot                                                |
| Unminified ESM runtime/contracts            | pass / exit 0               | All 11 cases; 10 A/A and 20 A/B pairs each; no timing-review flags                                                               |
| Object lifetime/configuration isolation     | pass under the finite probe | Four profiles, 3 pairs each, 240 discarded clients/process, zero added request/stream helper invocations                         |
| Artifacts/strict/source/declarations/bundle | review / exit 2             | Same four baseline strict failures; no new candidate diagnostics. GitHub/Stripe strict and all source/declaration consumers pass |
| Migration/publication                       | pass / exit 0               | 24 checks across collisions, representation and lifecycle                                                                        |
| Independent baseline-fix hypotheses         | pass                        | Eight strict checks and two Webhook probes in isolated generated copies only                                                     |

Latest source reports:

- Runtime: `.tmp/preimplementation/run-nt31TT/report.json`.
- Artifacts: `.tmp/preimplementation/run-nt31TT/artifacts-K6xuB2/report.json`.
- Migration: `.tmp/representation-migration/run-VjMZ4G/report.json`.
- Baseline-fix hypotheses: `.tmp/prerequisite-design-probes/run-1iy1Yw/report.json`.
- Durable closure evidence, raw paired runtime samples and hypothesis driver:
  `evidence/preimplementation-closeout-v44.json`.

The latest artifact run used one pair as a correctness/reproducibility check,
not a replacement performance benchmark. Its exact source/minified/gzip/Brotli
sizes reproduce the v4.3 sizes above. The ten-pair compiler/CLI measurements remain
separately identified in v4.3 evidence. Latest first-client/runtime timing has its
own twenty-pair samples; do not combine different-run medians into synthetic
end-to-end numbers or claim every small difference is significant.

### Bounded proof for the two independent baseline defects

For each of the four strict-blocked fixtures, baseline and candidate generated
sources were copied to new isolated check directories. Changing only the internal
`assignCallableProperties` generic constraint from a call signature to `object`
made all eight full strict checks pass. Negative consumers still reject invoking
stream-only public operations and passing primitive helper targets; ordinary
callable inference is preserved. This is a verified repair direction, not a
production change or a waiver of the original strict failures.

For the collision fixture, both versions reproduced the ordinary-empty-route-map
Webhook error. An own-key route lookup in separate ESM copies made empty maps
return 404, kept all four exact registered Webhook names dispatching to 204, and
did not evaluate an inherited route getter. This proves a narrow own-route fix;
production B2 must additionally review registration/handler lookup consistency
and preserve authentication/invalid-route/duplicate-route diagnostics. Do not
replace user maps with null-prototype requirements or erase protected keys.

### Implementation entry and baseline policy

No further broad representation experiment is required before staged implementation.
The next work is production code and its accompanying regression tests, not
repeating successful design experiments until every possible future check is green.

Implement B1/B2 correctness fixes in independent commits first, then freeze a new
corrected baseline **B-prime**. Compare B-prime against B-prime plus optimization
with the existing strict equality comparators. Compare original B against B-prime
separately as explicitly scoped correctness fixes. Do not weaken the comparison
harness to tolerate arbitrary changed diagnostics or routing behavior, and do not
mix B/B-prime samples in a benchmark pair.

Full Go allocator, semantic sections/reference ownership, final IDE/declaration,
allocation profiling, publisher fault injection and the remaining applicable
97-row production matrix are still implementation/release gates. This closure
is implementation readiness, not release approval. Small Brotli/full-client
fixed costs remain disclosed above; no zero-cost claim is introduced.
