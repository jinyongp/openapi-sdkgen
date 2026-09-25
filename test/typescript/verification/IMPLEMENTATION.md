# Internal representation implementation

This ledger records production changes separately from the historical prototype
experiments in FOLLOWUP.md. The canonical design and 97-check verification matrix
remain in the Loki workstream `typescript-compact-internal-identifiers-dd2423c4ad`.

## Ordered production units

| Unit                        | Commit    | Result                                                                                                                                                                                      |
| --------------------------- | --------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| C1 reusable verification    | `98c8b89` | Shared fixture catalog, comparison tools, preserved prototype evidence; full CI passed.                                                                                                     |
| B1 stream-only capabilities | `67272ed` | Internal decoration accepts object-shaped capabilities without adding a public buffered call. Failing-before type regression, conformance and four formerly failing strict fixtures passed. |
| B2 own Webhook maps         | `9a80e26` | Registration and dispatch use own route/handler/method properties. Seven focused runtime cases, full CI and strict server collision fixture passed.                                         |

B2's runtime tests intentionally bridge untyped JavaScript host maps to the
existing exported types. An optional exact `constructor` key also intersects
TypeScript's inherited Function-typed property; no public typing rule was
loosened as part of the own-property runtime repair.

## Corrected baseline B-prime

Revision: `9a80e261de03add22bd435a21cecefcd05a9a89c`.
Binary SHA-256: `f4cfe0276a1f043d32695ba04586a3ec67d07cf2db96cfc8fb7ec19871404a03`.
Frozen local binary:
`.tmp/implementation-baseline/9a80e261de03add22bd435a21cecefcd05a9a89c/openapi-sdkgen`.
Durable identity/source hashes and per-fixture outcomes:
`evidence/baseline-bprime.json`.

All eleven positive fixture/corpus cases passed runtime contract comparisons,
whole-source strict checking, normal source consumers, declaration emission and
downstream declaration consumers. The earlier four stream-only strict failures
are no longer baseline exceptions. Empty ordinary Webhook maps now return 404.

Source reports:

- `.tmp/preimplementation/run-2CCCR6/report.json`
- `.tmp/preimplementation/run-2CCCR6/artifacts-SxDsLS/report.json`

These same-binary comparisons used one pair to establish reproducibility and a
corrected contract baseline, not to make a performance claim. Timing noise raised
review flags in the raw runtime report; correctness passed. The artifact report
is pass/exit 0 with no baseline-blocked fixture. Do not relabel the diagnostic
runtime run as a performance acceptance result.

Subsequent optimizations compare B-prime against B-prime plus the optimization.
Original B-to-B-prime changes are intentional correctness fixes and are not
silently normalized out of comparison results.

## C2 — semantic sections and explicit references

The C2 implementation stores typed input-section presence rather than generated
TypeScript name strings. Presence, requiredness and bound-path subtraction remain
separate. Header/cookie public properties and helper slots retain their distinct
mappings. Operation-local spelling is still the historical long form in this unit.

Link targets now resolve directly in the owner module; emitted private names are
not placeholders in a string-replacement pass. Schema reference replay reports a
missing exact projection instead of emitting an empty replacement.

Validation: new mapping/requiredness/optional-body/pagination/reference-negative
tests, `just agent test`, and full `just agent ci` passed (Go coverage 83.0%).
The eleven fixture/corpus outputs contain 6,003 managed files and are all
byte-identical to B-prime. Their runtime contract comparisons also passed.
`evidence/c2-equivalence.json` records the per-case tree hashes and source report.
The one-pair comparison is not a performance measurement; its raw review flag is
retained rather than relabelled as a performance pass.

## C3 — fixed operation-local type names

C2 was committed as `cf006a2`. C3 now renders each operation's private type family
under the fixed `__sdkgen_` prefix, without a route payload or redundant `op`
marker. There is no helper accepting a route while ignoring it. Existing module
exports and the semantic suffix family remain unchanged. Scope-sensitive tests
select the actual owning operation module rather than concatenated SDK text.

Full `just agent ci` passed with Go coverage 83.0%. All eleven fixtures/corpora
passed unbundled runtime contract comparisons, whole-source strict typechecking,
ordinary source consumers, declaration emission and downstream declaration
consumers. Every minified client bundle was byte-identical to B-prime.

| Corpus | B-prime managed TS bytes | C3 TS bytes | Reduction | B-prime declaration bytes | C3 declaration bytes |
| ------ | -----------------------: | ----------: | --------: | ------------------------: | -------------------: |
| GitHub |               42,752,679 |  37,170,840 |    13.06% |                25,148,896 |           19,567,057 |
| Stripe |               32,083,911 |  29,544,374 |     7.92% |                18,963,049 |           16,423,512 |

Evidence: `evidence/c3-local-types.json` and its referenced source/artifact reports.
One-pair timing observations are not speed acceptance; their raw review status is
retained. The source reduction is from production fixed-name emission, not an AST
rewrite experiment. No property-construction optimization is included in these
measurements, and endpoint-level tree shaking is unchanged.

## C4 — owner-scoped property construction

C3 was committed as `cfebcf9`. C4 now uses an explicit per-owner wire render
context through nested schemas, response headers, multipart encodings and prepared
server definitions. The context reports actual import needs; no emitted-source
scan, provider heuristic, count threshold or global mode switch is used.

Named schema modules retain their literal wrappers under
`Object.fromEntries<WireProperty>` and import `WireProperty` only as a type, when
used. Inline operation/server schemas use the concrete two-array constructor.
The tiny helper is registered in the runtime artifact inventory. `callables`
directly re-exports the same function as `createWireProperties`; operations reuse
that existing import edge, and servers import the tiny helper only when needed.
No existing public export or descriptor shape changes.

Checks run on this production candidate, not the archived prototype:

- Final full CI passed (Go coverage 83.2%), including the additional inline-only
  prepared-server regression. Existing performance acceptance also passed across
  its generation/allocation/publication/process/incremental and stream gates.
- All eleven fixture/corpus runtime contracts passed with ten A/A and twenty A/B
  pairs each. No predeclared timing-review boundary was exceeded. Actual first
  client, contiguous startup and repeated construction are measured separately.
- Four lifetime profiles passed three pairs of finite 240-client discard checks.
  Inline lifecycle positively exercised construction and added zero helper calls
  during requests/stream frames. No late-retention or configuration-mixing flag.
- All eleven full strict/source/declaration checks passed. The helper passed
  eleven shared semantic checks plus 128 adversarial exact-key vectors, including
  manual nonidentity mappings. Template and emitted-helper tests also exercise
  array mismatch, facade identity and frozen/recursive child graphs.
- Twenty-four migration checks passed: three fixtures, eight checks each,
  including fresh equality, no-op nanosecond mtimes, edited files, and unmanaged
  file/directory/symlink conflicts at the newly generated helper path.
- The smallest and largest nonempty named schema source leaves sampled from each
  real-world corpus produced byte-identical direct wire-schema bundles. The
  unbundled representation leaf kept the original runtime dependency graph.

All essential raw samples, reports, source checksums and binary identities are
in `evidence/c4-property-construction.json`. Its report links distinguish runtime,
artifact, migration and process-cost executions. The artifact run uses one pair
for correctness/size; speed data below uses ten independent interleaved process
pairs and per-child GNU time CPU/RSS, without build/network time.

| Metric                                |           GitHub B-prime -> C4 |           Stripe B-prime -> C4 |
| ------------------------------------- | -----------------------------: | -----------------------------: |
| Managed TS bytes                      |       42,752,679 -> 37,123,103 |       32,083,911 -> 29,141,861 |
| Declaration bytes                     |       25,148,896 -> 19,567,705 |       18,963,049 -> 16,424,160 |
| Minified client bytes                 |         2,583,041 -> 2,489,156 |         3,592,758 -> 3,114,442 |
| gzip bytes                            |             217,267 -> 212,187 |             319,393 -> 287,241 |
| Brotli bytes                          |             140,153 -> 140,550 |             159,833 -> 161,342 |
| Fresh CLI median wall ms              |         1,311.966 -> 1,273.730 |           1,007.486 -> 972.981 |
| Strict compiler median wall ms        |         3,085.412 -> 2,540.557 |         3,438.022 -> 2,089.737 |
| Strict compiler median peak RSS bytes | 1,989,801,984 -> 1,440,389,120 | 2,564,788,224 -> 1,266,049,024 |

These are cumulative C2/C3/C4 versus the corrected B-prime, not C4-only speed
attribution. Generation improvements are small and some paired samples are
slower; normal source-consumer timing is essentially flat. Raw paired statistics
are not ratios of the separate medians. Do not claim zero cost: Brotli grew by
397/1,509 bytes, and small helper-using full-client fixtures retain fixed costs.
This work does not solve endpoint selection or endpoint-level tree shaking.

The measured candidate binary is preserved at
`.tmp/implementation-checkpoints/c4/7d627797708044f3e667afb7e0b32cc7eb839e8042079a00db322624ce39b27d/openapi-sdkgen`.
Its source hashes are in the durable evidence; subsequent builds need not retain
that working `.tmp/bin` filename or binary identity.

## C5 — artifact-owned local aliases

C4 was committed as `9722fad`. C5 replaces schema type-import, resource-builder
and Link-group aliases with one deterministic local plan per emitted artifact.
The plan reserves fixed/readable/protected names, collects exact structured keys,
freezes the result and resolves all references from that same owner. Repeated uses
are idempotent; ownerless/late requests, unknown roles and missing lookups fail.
No SDK-global counter, hash or mutable allocator is introduced in this unit.

Named schema reference replay checks the exact schema and input/output projection,
including self references, rather than only occurrence counts. Operation reference
localization visits the referenced keys instead of scanning all component plans.
A module's schema aliases and Link groups share the same frozen reservation domain;
Link `byStatus` direct-arrow names remain unchanged. Non-path parameters no longer
allocate unused private bindings; their exact wire properties and readable path
selectors remain unchanged.

The shared catalog now contains the `aliases` fixture. It covers repeated imports,
normalization/Unicode-sensitive schemas, input/output projections, private-looking
opaque data, separate resource builders and a module combining Links and streaming.
Tests identify imported targets and owner modules, not historical alias spellings.
Go tests include capture/reservation, permutation, frozen/missing-reference errors,
24 parallel isolated plans, 100,000 unique keys, unrelated-entity locality and
server-addon isolation.

Twelve fixture/corpus cases passed unbundled runtime contracts and complete
strict/source/declaration/helper/bundle checks. The runtime pilot uses one A/A and
one A/B pair: its timing-review flags remain recorded and are not speed acceptance.
Final CI and existing performance acceptance are recorded separately in
`evidence/c5-local-aliases.json` before this work unit is committed.

| Corpus | B-prime managed TS | C4 managed TS | C5 managed TS | C4 declarations | C5 declarations |
| ------ | -----------------: | ------------: | ------------: | --------------: | --------------: |
| GitHub |         42,752,679 |    37,123,103 |    36,442,803 |      19,567,705 |      19,221,075 |
| Stripe |         32,083,911 |    29,141,861 |    28,332,704 |      16,424,160 |      15,757,873 |

GitHub and Stripe minified bundles are byte-identical to C4. C5's additional
680,300/809,157 source bytes removed are a source/declaration benefit, not new
endpoint tree shaking. No C6 aggregate-token savings are included.

The production local allocator's collection/freeze/lookup benchmark used five
samples of three iterations per size. Medians were 0.399 ms / 529,472 allocated
bytes for 1,000 keys; 4.487 ms / 4,386,693 bytes for 10,000; and 51.154 ms /
36,483,978 bytes for 100,000. These are allocator-only costs, not whole-generation
latency or peak RSS. `just agent identifier-perf` reproduces the measurement.

One comparison was correctly rejected when the `aliases` input changed during
execution; another failed the free-space precondition before generation. Both are
preserved as non-passing attempts. Only disposable TypeScript/JavaScript outputs
from the completed research runs `run-nt31TT` and `run-nWuLsA` were pruned to recover
space. Their reports, manifests, configurations and logs remain, and report hashes
were verified unchanged. The older C4 evidence file received formatting only;
parsed JSON equality to its committed version was checked.

### Independent C5 follow-up

`evidence/c5-local-aliases-followup.json` preserves a second, separately identified
validation run without overwriting the main C5 record. The final `aliases` fixture
uses repeated `EventValue` projections in its Link/stream operation; a Go assertion
requires the corresponding generated import alias and both factories in that owner.
That strengthened case passed runtime, strict/source/declaration and bundle checks
using the same frozen production binary. The earlier full twelve-case run remains
historical, with the follow-up superseding only its `aliases` input.

The independent external-corpus measurement used ten paired fresh processes.
Its medians are cumulative C2-C5 against B-prime, not C5-only attribution:

| Metric                         |           GitHub B-prime -> C5 |           Stripe B-prime -> C5 |
| ------------------------------ | -----------------------------: | -----------------------------: |
| Fresh CLI wall ms              |         1,520.176 -> 1,544.001 |         1,062.185 -> 1,065.594 |
| Strict compiler wall ms        |         3,790.416 -> 3,084.084 |         4,021.306 -> 2,703.980 |
| Strict compiler peak RSS bytes | 1,960,232,960 -> 1,438,445,568 | 2,534,785,024 -> 1,245,554,688 |

Generation is essentially flat (the GitHub median is slightly slower), while
strict checking uses less time and memory. Raw paired samples and exact input,
compiler and binary identities are retained. These numbers must not be mixed with
the main report's separate run or presented as a universal speed guarantee.

### C5 pinned unit validation

The earlier C5 pilot and shared-log measurements above are historical observations,
not the current acceptance result. The final unit evidence is
`evidence/c5-local-aliases.json`, bound to candidate binary SHA-256
`14498213d00f40e7185947325ddc2602e523c6c9a30c77d273b64862c64d98e3`.
All production source hashes were checked unchanged after the measurements.

- Runtime comparison `run-w8T0l5`: all 12 positive cases, 10 A/A and 20 A/B
  pairs each, no timing-review flags. Four lifetime profiles retained three
  independent pairs and 240 discarded clients per process; helper-positive
  profiles had zero additional request/frame construction calls.
- Artifact checks `artifacts-bR7Nzi`: all 12 whole-source strict, normal source
  and declaration consumers passed. All 11 cases shared with the C4 archive
  retained byte-identical minified client bundles. The new aliases fixture also
  passed source/declaration/runtime checks against B-prime.
- Migration `run-hd9PN6`: four fixtures, including aliases, passed all 32 checks
  for identity migration, fresh equality, no-op hashes/mtimes, diagnostic rollback,
  user edits and unmanaged helper-file/directory/symlink conflicts.
- Final `just agent ci`: session `lvOYxc4yKiJ4HXoW`, exit 0, Go coverage 83.3%.
  Existing `just agent perf-acceptance`: session `qN-lGjCKhPJM8hMc`, exit 0.
  The performance gate's historical baseline is not the same as the paired
  B-prime corpus baseline, and its historical gains are not attributed to C5.

Ten-pair independent CLI/compiler measurements in `run-mQOuXu`:

| Metric                       |           GitHub B-prime -> C5 |           Stripe B-prime -> C5 |
| ---------------------------- | -----------------------------: | -----------------------------: |
| Fresh CLI median wall ms     |         1,440.557 -> 1,385.821 |         1,132.386 -> 1,102.260 |
| Strict median wall ms        |         3,728.971 -> 2,959.708 |         4,139.892 -> 2,710.289 |
| Strict median peak RSS bytes | 1,978,064,896 -> 1,437,034,496 | 2,560,512,000 -> 1,272,229,888 |

These are cumulative C2-C5 costs versus corrected B-prime. Raw paired differences
are retained and are not ratios of these separate medians. Individual slow pairs
remain in the report; no universal speed improvement is asserted. Deterministic
C5-only source savings relative to C4 are 680,300 and 809,157 bytes.

The final allocator probe uses its own immutable invocation log
`.tmp/perf/local-identifiers-D7T3E5.log` (session `SS05ST8wSIK__ykx`). Five
samples of three iterations yielded medians 0.395435 ms / 529,472 allocated bytes
for 1,000 bindings, 4.830016 ms / 4,388,514 bytes for 10,000, and 51.394997 ms /
36,484,010 bytes for 100,000. These are collection/freeze/lookup costs, not full
SDK generation, peak heap, or aggregate hash allocation. The wrapper now emits a
unique log path so subsequent invocations cannot overwrite completed samples.

A space preflight stopped this execution before generation. Completed B-prime,
C2 and C3 generated copies (`run-2CCCR6`, `run-4vdozh`, `run-GRHm68`) were
then archived under `.tmp/verification-archives`; every archived file was read
back and SHA-256 verified before its working copy was removed. Original reports,
logs and bundled probes remained in place; input corpora and frozen binaries were
not removed. Archive hashes and exact recovery paths are embedded in C5 evidence.
The successful rerun retained the original space and correctness thresholds.

## C6 — checked aggregate identifiers

C5 was committed as `c99b70f`. C6 replaces the remaining aggregate registry,
schema-wire, enum and server private aliases with explicitly planned names.
Semantic keys remain exact and are never replaced by compact tokens.

The canonical identity encoder preserves domain, field count, byte lengths and
individual decoded field bytes. Callback origin, source, component, callback
name, expression and method remain separate fields. SHA-256/Base32 chooses only
an emitted spelling: prefix collisions extend deterministically; a forced full
digest collision switches to injective canonical-payload encoding. Final
reservations include derived `Context`, `Response`, `Handlers` and
`PathParameters` declarations. An entity shares one token across its requested
roles, and names resolve only after the artifact plan freezes.

Registry, schema-wire and enum owners reject duplicate declarations. Registry
planning also rejects a missing or mismatched compiled operation. Server names
are assigned on an emission-local copy, without mutating reusable prepared
callback/webhook definitions. Historical callback ordering is retained separately
from exact lexical identity. Protected Link leaf function names and all existing
public keys, exports, prototypes and callable reflection remain unchanged.

### C6 unit evidence

Pinned measured binary SHA-256:
`094b4d9dc37e668ac1fd97bd16c5d2fcb7dc2f4ecc412df355b61b9983c3d25c`.
Evidence: `evidence/c6-aggregate-identifiers.json`, including raw reports, source
hashes, benchmark samples and bounded failure history.

- Integrated `just agent ci` passed (`SXj0SrORYSNbgR19`), Go coverage 83.4%.
- Runtime `run-MgnasE` passed all 12 correctness comparisons, but its early
  timings overlapped an independent review CI. Those samples remain historical.
  Authoritative runtime recheck `run-7XozBH` was run without other scheduled
  CPU-heavy verification: all 12 cases passed with 10 A/A and 20 A/B pairs each,
  no timing-review flags, and three independent pairs for each of four finite
  lifetime profiles. Generated-tree hashes and public contracts match the first
  run. This does not claim a globally idle host or cold filesystem.
- `artifacts-839asX` passed all 12 strict/source/declaration consumers and
  bundle/helper checks. Its single timing pair is correctness evidence only.
- Migration `run-a40bhP` passed 32 checks across four fixtures: distinct identity,
  fresh equality, no-op content/mtime preservation, diagnostic rollback, user
  edits and new helper file/directory/symlink conflicts.
- Forced prefix/full collisions, derived reservations, fallback reservations,
  invalid/late requests, request permutation, owner isolation, exact source
  fields, long keys, repeated generation and unrelated-entity locality passed.
  The large unit test resolves 100,000 entities to 300,000 distinct bindings.

The enum regression originally pinned an incidental long private alias. It now
checks that the exact enum key references the actual declared constructor result
and value array, while retaining all enum-value/order/type assertions. An
unformatted concurrently added test interrupted one early CI run; ownership was
coordinated and the final formatted integration passed. Neither earlier failure
was relabelled as a pass.

### C6 size and measured costs

| Corpus | B-prime TS bytes | C5 TS bytes | C6 TS bytes | C6-only bytes removed | C6 declaration bytes |
| ------ | ---------------: | ----------: | ----------: | --------------------: | -------------------: |
| GitHub |       42,752,679 |  36,442,803 |  34,947,005 |             1,495,798 |           19,221,075 |
| Stripe |       32,083,911 |  28,332,704 |  27,348,623 |               984,081 |           15,757,873 |

For these two corpora C6 declaration bytes and minified client hashes are
unchanged from C5. This unit's additional gain is generated source size, not new
bundle tree shaking or additional declaration reduction. C2-C6 cumulative source
reductions versus B-prime are approximately 18.26% and 14.76%.

Ten independent paired processes in `run-5S5cWC` measured cumulative C2-C6 costs:

| Metric                                |           GitHub B-prime -> C6 |           Stripe B-prime -> C6 |
| ------------------------------------- | -----------------------------: | -----------------------------: |
| Fresh CLI median wall ms              |         1,320.719 -> 1,245.173 |             996.665 -> 934.119 |
| Strict compiler median wall ms        |         3,140.122 -> 2,551.258 |         3,399.704 -> 2,122.185 |
| Strict compiler median peak RSS bytes | 1,980,792,832 -> 1,443,082,240 | 2,571,952,128 -> 1,268,496,384 |
| Generator median peak RSS bytes       |     241,633,280 -> 243,535,872 |     161,957,888 -> 162,441,216 |

Generator RSS is slightly higher in this run; do not claim every memory metric
improved. Raw paired differences, CPU samples and ordinary consumer costs are
preserved. Timings are not C6-only attribution and are not cross-machine promises.
Existing C4 Brotli/small-client fixed costs remain recorded; aggregate renaming
has not erased them.

`just agent identifier-perf` now covers both local and aggregate allocators.
The aggregate probe uses three bindings per entity; key encoding is prepared
before timing. Five samples of three iterations yielded 1.569 ms / 1,862,922
allocated bytes for 1,000 entities, 17.617 ms / 22,426,640 bytes for 10,000 and
243.220 ms / 198,394,144 bytes for 100,000. These totals include request, hash,
freeze and resolution, not full generation or peak RSS. Local and aggregate
workloads have different role counts and must not be compared as equivalent work.

### Additional review and space recovery

A fixed independent review snapshot passed Go tests and full CI at the same
backend/test source hashes. Its exact compiled-operation ownership finding was
verified with an actual negative control: a Go source overlay removing only the
new guard fails `TestRegistryIdentifiersRequireExactCompiledOwner`; the guarded
source passes. The bounded review receipt is embedded in C6 evidence and does not
replace producer acceptance or C7.

C5 comparisons require identical input hashes. In particular the aliases row uses
its strengthened input from `c5-local-aliases-followup.json` (375,042 -> 371,331
source bytes), not the older main C5 row. All twelve minified hashes are identical
to their matching C5 inputs.

Before the uncontended runtime recheck, 36,480 generated TS/JS/map files from this
session's completed `artifacts-839asX` copy were compressed into
`.tmp/verification-archives/c6-artifacts-1790319026835.jsonl.gz`.
Archive SHA-256: `1dcc0edf8cd3b6de6808ad66063df3305e61c1a49b247a7f2889fed8ac1e2dcc`.
Every archived record was decoded, length/hash checked and compared with its
original before the corresponding working copy was removed. Only this invocation's
372,018,900 reproducible source bytes were removed; all reports, configurations,
manifests, logs, original inputs and frozen binaries remain. The receipt records
repository-relative paths and base64 content in compressed JSONL, so those copies
are recoverable. The raw-file receipt and archive identity are linked from the
durable C6 evidence. This cleanup did not lower a space or correctness threshold.

## Remaining implementation

C6 unit-local checks are passing; its final commit and existing performance-gate
receipt are recorded in the central workstream and evidence. C7 remains pending:
remove only superseded scaffolding and close the applicable 97-check matrix
against one final integrated candidate, including full phase allocations,
publication fault injection, packaging and unresolved matrix-specific checks.
No push, release or overall workstream completion is implied.
