# Wire conformance work

Baseline: `65558c4`. Scope: query send/receive, URLencoded bodies, schema references,
XML/JSON-seq, format assertions, provider evidence and unsupported operation refs.
Preserve documented unsupported features and OpenAPI's ambiguous space/pipe data delimiters.

| Unit | Implementation | Review | Validation | Commit |
| --- | --- | --- | --- | --- |
| Query boundaries | complete | complete: callers, codec precedence, union order and error mapping | TypeScript full; generated Go query/inbound suites passed | `d31671e` |
| Form encoding modes | complete | complete: mode presence, style defaults, codec precedence, shared escaping and multipart callers | TypeScript full; generated Go form/query and multipart suites passed | `6647cac` |
| Reference resources and diagnostics | complete | complete: resource identity, opaque data, source immutability, containment, remote locks, direct and CLI callers | compiler/IR/CLI suites; file/stdin/URL and both diagnostic modes passed | `c1a16a2` |
| XML and JSON-seq | complete | complete: XML escaping order/CDATA, JSON whitespace and RS boundaries, NDJSON EOF and cleanup | TypeScript full; generated Go XML independent parser and protocol suites passed | `30c411b` |
| Format assertions | complete | complete: syntax without URL repair, UTS46/A-label/context/Bidi checks, mailbox grammar, selected runtime dependencies and licensing | 369 pinned official cases in both targets; TypeScript full/lint and Go format/URL/IDNA suites passed | `e8d89cd` |
| Provider evidence | complete: pinned authoring preparation, independent Go compilation, source-bound measurement gates and historical manifest preservation | reviewed: containment, opaque data, operation overrides, evidence integrity and failure semantics | preparation/runner/merge and Python/docs evidence tests passed; DigitalOcean TypeScript generated and strictly typechecked all 739 contracts, Go explicitly blocked by its invalid Unicode pattern; Zenith Go generated and compiled 40 routes | `f4d5f63` |
| Provider follow-up | Go rejects empty request media contracts before emission; required/optional cases covered | complete: shared request planner, union and server callers | focused regressions and pinned Spoonacular input passed: explicit input rejection replaces invalid generated code | `6d31919` |
| Measurement stability | isolate Go compilation from inherited 12GiB heap budget; serial builds, explicit soft heap/concurrency/deadline policy and offline installed toolchain | reviewed: subprocess isolation, timeout failure reporting, evidence policy and cancellation | actual invalid/valid compiler outcomes, pinned Zenith and policy-tamper tests | `3fd1f3c` |
| Evidence recovery | TypeScript and Go generation checkpoints are uploaded before compilation; missing compiler output is recorded as unavailable only after verifying the exact failed Actions job | reviewed: checkpoint cannot pass verification, artifact identity, source/run/job provenance and reader labels | independent Zenith generation/compiler hashes matched; checkpoint tampering and job-provenance tests; docs validation/build passed | `4e532d6` |
| Legacy pins and report validation | bind legacy Git blob pins to actual SHA256 input bytes; revalidate archived Actions data without relabeling its SDK source | reviewed: original bytes, source equivalence, immutable artifact lookup and separate validation provenance | 32 Python and 32 docs evidence tests; archived source `4e532d6` validated successfully in Actions run `39389191303` | `1abb894`, `a49aa4e` |
| Multipart binary diagnostic | reject unsupported complete binary multipart contracts before emitting calls to absent model encoders; retain named-field and positional-part support | complete: required/optional, union/single media, OpenAPI 3.0/3.1/3.2 and server planner | pinned Loket input now reports the exact schema location; generated multipart and versioned binary regression suites passed | `488815a` |
| URI boundary follow-up | keep scheme and IPvFuture syntax ASCII; require full fragment and port validation, preserving valid Unicode IRI fragments | complete: Unicode case folding, final LF/U+2028, authority tails and matching Go behavior | 373 TypeScript format checks; full TypeScript verification, formatting/lint and generated Go format regression passed | `a764d6b`, `2042d8b` |
| Combined review | fixed shared serialization layer dependency and Go referenced/allOf query shape handling; malformed query object tokens now fail explicitly | all six areas reviewed; affected query callers rechecked; no outstanding confirmed defects in this scope | TypeScript full/runtime boundaries/tools, Go query, vet/format, Python and docs checks passed; complete Go suite exposed three stale assertion/documentation failures, all rechecked and passed after correction; local `ci:dev go` passed at `488815a`, added Go URI checks passed at `2042d8b`; final source CI run `39391330184` passed all 11 jobs; provider results below | `91e2b1e`, `56808ef`, `0c156e9` |

Each unit requires relevant generated SDK checks, caller review and no outstanding
confirmed defects before committing. Final checks apply to the complete revision.

## Assessment and delivery order

The supplied report's six defect categories were valid at the baseline: resource-aware
schema references were diagnosed too late; query delimiters lost input boundaries;
URLencoded styles were dropped; XML whitespace was normalized after emission;
JSON-seq accepted incomplete scalars; hostname assertions validated extracted URL hosts.
The query-only URL reference and silently ignored Operation Object references were
also confirmed. The report's historical provider measurements were not evidence for
the current source or for Go compilation.
The `v1.3.0` tag resolves to `623dffd47029a8856d7b1cf4aade0c8dcdc77aac`;
its `internal`/`cmd` difference from the baseline is limited to a Go test file.
The reported product defects therefore also apply to that release.

Work followed the requested priorities: preserve query/form data, unify reference
resolution and entry-point diagnostics, add independent media/format conformance
checks, then rebuild provider evidence. Broader inspection also covered inbound
query boundaries and referenced/composed schemas, URI/IRI/email assertions,
provider contract preservation, generated Go compilation, and evidence recovery.
Final review found the additional multipart binary diagnostic and URI boundary
issues recorded above. Unsupported whole-body binary multipart inputs remain an
explicit Go limitation; they are not reported as working provider support.

The final provider run measured source `2042d8b`, which includes the URI
follow-ups. Run `39390230568` was cancelled because it measured the preceding
source; its partial results must not become the final compatibility snapshot.

Final source validation: [CI run 39391330184](https://github.com/jinyongp/openapi-sdkgen/actions/runs/39391330184)
passed all 11 jobs at `2042d8b2207574824360bc5d30a79ad757bd6281`, including the full
Go suite, TypeScript runtime/tools, four consumer compiler versions, installers,
and documentation. Local Python and documentation evidence suites each passed
32 tests. The pinned official format corpus contains 369 cases; the TypeScript
format file now runs 373 tests including additional boundary checks.

## Final provider evidence and remaining limits

[Measurement run 39391326397](https://github.com/jinyongp/openapi-sdkgen/actions/runs/39391326397)
measured the same source as the successful CI above. Its aggregation/publication
job passed and preserved all outcomes. The overall workflow failed because the
Cloudflare job exited with code 143; this is not reported as a successful workflow
or successful Cloudflare compilation.

- TypeScript: 38/38 supported-profile documents generated and strictly checked;
  DigitalOcean retained all 739 operations after pinned preparation.
- Graph: full and selected generation completed; no consumer compiler check.
- Go: 33/38 documents generated and compiled, including GitHub's 1,221 operations,
  GitLab's 1,860 and DocuSign's 402; webhook/callback handlers are counted separately.
- Go blocked inputs: Spoonacular's empty media key, DigitalOcean's invalid
  JavaScript Unicode regular expression, and Loket's unsupported whole-body binary
  multipart contract. These remain failures with explicit diagnostics.
- Stripe: generation completed; its 559-operation SDK exceeded the independently
  recorded 20-minute compiler budget. This does not establish a source-code error
  or successful compilation.
- Cloudflare: generation evidence was preserved, but no completed compiler report
  was recovered. Job `118524005930` was independently verified against the exact
  source/run before the aggregate marked compilation unavailable.

These are generation/compilation measurements, not live provider API exercises.
Future expansion of Go provider coverage requires separately investigating large
SDK compiler cost and defining support for complete binary multipart bodies.
Invalid provider schemas require corrected inputs; the compatibility preparation
does not silently repair them. Existing evidence is not relabelled as newer code.

## Delivery review

The eight files under `test/compatibility/ci` were regenerated by the measurement
exporter from the final Actions aggregate. Their uncompressed contents and
provenance exactly match the published Actions artifact. The rendered Go table
contains all 38 documents with 33 pass, three blocked, one failed and one
unavailable outcome. The updated snapshot passed all 32 documentation evidence
tests, `validate:docs` and `build:docs`.

The final delivery changes only this record and measurement data after the tested
SDK source `2042d8b`. No pre-existing user changes were present. Relative to the
baseline, task-owned repository changes add 37 files, modify 77 and delete none;
temporary diagnostics and generated test modules remain ignored under `.tmp`.
All requested areas and fixes have completed their unit and combined review loops.

## Follow-up: resolve provider limitations

The user's follow-up expands completion beyond recording failures: resolve all
five remaining Go provider cases, then repeat unit/combined review and commits.
Live provider calls are not required. Independent local HTTP servers will check
the generated clients against separately specified request/response contracts.

| Area | Completion criterion | State |
| --- | --- | --- |
| Spoonacular | explicitly pinned correction of invalid parameter-body metadata; complete Go generation/compilation and HTTP contract checks | input unit complete: 99 operations compiled; independent HTTP contract test passed |
| DigitalOcean | explicitly pinned tag-pattern correction without loosening the Unicode regex contract; 739 operations and successful Go compilation | input unit complete: 739 operations compiled; independent HTTP contract test passed; compiler cost remains in the performance unit |
| Loket | usable whole-body binary multipart representation with independent server checks; no unsupported-input shortcut | multipart unit complete; pinned upload operation compiled and passed independent HTTP checks; full SDK compilation belongs to the performance unit |
| Stripe / Cloudflare | complete ordinary Go compilation of the full SDKs under a recorded bounded profile; correct root cause rather than increasing the passing count | implementation and targeted review complete; committed-source full compiler checks pending |
| Delivery | refresh actual Actions evidence, review all five areas together, commit and leave a clean tree | pending |

Stripe baseline output at `fa9e685`: 17,448 files / 248.7 MB, including 153.1 MB
models and 81.7 MB validators. A local compiler run using the recorded 3 GiB soft
heap / two CPU / serial package profile completed the schema package in about
270 seconds while its heap reached about 7.8 GB. The model compiler's RSS then
exceeded 13 GB after seven minutes; that task-owned compiler was terminated to
preserve host responsiveness. This is an interrupted diagnostic baseline, not a
successful full compile or a completed benchmark. Raw diagnostic output remains
under `.tmp/provider-resolution-20261020/stripe-baseline.*`.

The input unit uses `digitalocean-tags-v1` and `spoonacular-parameter-bodies-v1`,
retains original source hashes and pins corrected output hashes. Normalization is
explicit and preserves opaque examples/extensions. Empty-media bodies are removed
only after checking that their fields already exist as equivalent path/query
parameters; real request bodies remain unchanged. New provider rules require a
completed HTTP contract test in the compiler report. A skipped test cannot pass
this check. Historical measured manifests are archived with the CI results so
old evidence is not validated against corrected inputs.

Unit validation: targeted Go preparation/compiler-verifier tests, 32 Python
evidence tests, 33 documentation evidence tests, documentation validation/build,
and actual full Go modules with `TestSpoonacularHTTPContract` and
`TestDigitalOceanHTTPContract` passed. Those HTTP servers independently assert
URL/body/header behavior and HTTP errors; no provider service was called.

Multipart unit review covers required/optional and single/union bodies across
OpenAPI 3.0, 3.1 and 3.2, including the server addon's generated declarations.
Complete binary multipart bodies retain their framing and metadata boundary.
Missing/mismatched media, malformed boundaries and invalid header bytes fail
before opening a source. Replayable sources open and close once per execution.
Named-field multipart retains its existing encoder. The 12 generated-module
combinations passed, as did existing versioned binary/open-media checks. The
pinned Loket upload operation (one selected route, not a full-corpus pass) passed
`TestLoketHTTPContract` against an independently parsed multipart HTTP request.
Full SDK compiler evidence is still pending; interrupted diagnostics are not
counted as successful measurements.

HTTP evidence unit review: the existing DigitalOcean/Spoonacular full-module
probes are joined by pinned Loket upload, Stripe balance and Cloudflare token
verification probes. All three new selected-route probes compiled and passed
against independent local HTTP servers; these selected results do not increase
full-corpus coverage. Stripe/Cloudflare probes also reject HTTP errors and invalid
response enums. Successful published full-provider results require the exact
completed test name, with missing/wrong names rejected by Python and documentation
readers. The compiler verifier records route/hash evidence before releasing its
plan; a real emitted-module regression check covers that ordering. The review
checks passed: targeted Go verifier tests, 22 Python evidence tests and 33
documentation evidence tests. Complete SDK compiler checks remain in progress.

Compiler layout unit: packages of at least 8 MiB are split along strongly
connected file dependencies, with 4 MiB target batches and intact recursive
components. Shared runtime state lives in a common implementation package.
Public model aliases, forwarding functions, constants and method sets preserve
consumer imports; model backing fields remain private. Internal schema state
fields, including XML's nested validation state, are rewritten consistently.
Only one package's rendered source is buffered during layout. This adds facade
source rather than claiming a smaller download. Stripe's largest measured model
implementation group is 9.25 MB versus the previous 153.1 MB model package;
Cloudflare's largest group is 3.97 MB. Facades still need complete compiler proof.

Review found and fixed private-symbol allocation collisions and XML state-field
ambiguity. Targeted generated-module checks pass for deterministic names,
recursive/dynamic references, enum/error validation, multipart composition/binary
ownership and XML HTTP encoding/decoding. Full local runs interrupted with
SIGTERM have no passing evidence; no speedup ratio is claimed. The final gate is
fresh Actions CI and all 38 full Go provider compilations at the committed source,
including completed independent HTTP probes for the five repaired providers.

The combined local Go run also exposed a historical holdout identity test still
reading the active manifest. It now checks the archived manifest measured by
that report, retaining the exact SHA/document identity checks. This regression
test and the final XML/dynamic/partition checks pass. Fresh complete CI is still
required for the final committed state.

Compiler follow-up review: local codecs remain private unless referenced across
implementation packages. Stripe's exported private helper functions fall from
85,573 to 2,393; 83,180 local codecs no longer inflate import metadata. The
four-MiB batches reduce its implementation package count from 141/78 to 40/22
(models/schema). Public facades retain the full API. Deterministic collision,
private-helper visibility, recursive/dynamic, XML and multipart regression checks
pass, along with 33 documentation evidence checks and the Python Go evidence gate.

With these layout changes, the complete local Stripe module and independent HTTP
contract passed under the original serial/two-CPU/3-GiB-soft profile in 12:35.80,
with peak RSS 6,288,928 KiB. This instrumented diagnostic is not an Actions result
or a matched speedup benchmark. Cloudflare still exceeded that profile's 20-minute
compiler timeout; its generation succeeded but compilation is a recorded failure.

The compiler budget for new measurements is serial packages, four CPU threads,
6 GiB soft Go memory and the unchanged 20-minute compiler timeout, within the
standard public Linux runner. Generation retains its separate two-CPU/3-GiB
budget. The compiler's soft limit does not cap RSS. Reports record their exact
policy; the current aggregate rejects older policies rather than combining them.
Archived evidence retains its original budgets, source and failed outcomes.
The full Cloudflare check is in progress under the new policy.

At the preceding committed source `3eb502b`, Actions CI run `39414230062` passed
all 11 jobs. Compatibility run `39414236568` completed all three repaired inputs
(Spoonacular 99 operations, DigitalOcean 739, Loket 663) and their exact independent
HTTP tests. Stripe and Cloudflare exceeded the old compiler timeout; successful
workflow status alone is not compiler success. Fresh complete Actions evidence is
required for the updated layout and policy before delivery.

At `9dc84f6`, Actions CI run `39425215741` passed all 11 jobs. Full Actions
reports from run `39425219559` confirm Spoonacular 99, Loket 663, DigitalOcean
739 and Stripe 559 operations compiled and completed their exact HTTP contract
tests at that clean source. Cloudflare's local complete-module check still
exceeded 20 minutes at the public model facade; one observed compiler RSS was
12,445,128 KiB. Its orphaned task-owned compiler was stopped after timeout.
The subsequent extended diagnostic was superseded and interrupted, not passed.

Review traced another duplicate import path: model state and generated calls
loaded the complete internal schema facade, which forwards to the same physical
validators. Generated callers now bind directly to the validator implementation
packages. Both public model imports and the internal facade remain available;
no schema, model or operation is omitted. Emission resolves schema bindings first
while still buffering only one rendered package. Import aliases reserve existing
identifiers and preserve locally shadowed names. The new shadowing regression
found and fixed stale import retention in the formatter. Targeted generated
modules pass for binding/collisions, recursive/dynamic references, XML HTTP and
multipart composition/binary alternatives. Full Cloudflare compilation under the
unchanged four-CPU/6-GiB-soft/20-minute policy is in progress for this candidate.

The final layout also omits unused internal schema facades after binding all
generated callers. This supersedes their temporary retention above: internal
validator forwarding declarations are implementation details; public model types
and functions retain their facades. Forced-partition regression checks reject an
unused schema facade and exercise dynamic validation through the public model
decoder. Binding/shadowing, recursive/dynamic, XML and multipart generated-module
checks pass. The superseded local diagnostic and queued compatibility run
`39430043575` were cancelled, not counted as passing evidence. Final source CI and
complete provider measurements will be repeated with this smaller layout.

At `66a3025`, Actions CI run `39430780768` passed all 11 jobs. The complete local
Cloudflare output retained 3,595 operations and fell from 134,707 files /
1,053,570,400 bytes to 104,203 / 984,831,507 bytes. This is generated-source
inventory, not a timing comparison. Compilation still exceeded the 20-minute
measurement deadline at its public model facade; one observed compiler RSS was
12,862,124 KiB after eight minutes in that package. The 20-minute case remains a
failure. Run `39430785327` was superseded and cancelled, not published as passing.

The final compiler profile uses the same serial packages, four CPU threads and
6 GiB soft memory, with a recorded 40-minute deadline for the complete large
public API. This is an explicit measurement-budget change after removing
avoidable duplication, not a speedup claim or a route/validation omission.
TypeScript's diagnostic compiler retains its 20-minute budget. The Actions Go
step leaves 50 minutes for generation plus compilation, within the existing job
budget. Current aggregation rejects the earlier 20-minute profile; archived data
keeps its original source, policy and failed outcomes.

Repeated timeout diagnostics also confirmed that killing only the Go driver
left an orphan compiler. Linux measurements now give each compiler invocation
its own process group and cancel the complete group. A real parent/child test
verifies descendants stop after cancellation; interrupts also cancel that group
and are reported separately from timeouts. The optional non-Linux diagnostic
retains its platform default and its Windows test binary cross-compiles. Go verifier tests, 22 Python evidence tests,
33 documentation evidence tests and documentation validation passed. Fresh full
compiler/HTTP evidence is required under the final profile before delivery.

At `0cfc699`, Actions CI run `39435508738` passed all 11 jobs. Run
`39435513932` confirmed full Go compilation and exact HTTP tests for Loket,
DigitalOcean and Stripe under the 40-minute policy. Cloudflare still exceeded
40 minutes. Separate jobs failed to finalize artifacts (HTTP 404) or download
pnpm; these infrastructure failures are not counted as passes. The local
Cloudflare run failed when the installed Go 1.27.2 `vet` executable disappeared
during a system update to 1.27.3; it is not a successful compiler measurement.

The next layout binds generated operations and other callers directly to model
implementation packages as well as validators. Public model aliases and codec
functions remain available to consumers; generated callers avoid loading their
duplicate facade metadata. The forced-partition check now rejects a generated
caller importing the complete public model facade, while external consumers
still use the public models package to verify type and function compatibility.
Recursive/dynamic, XML HTTP and multipart/binary generated-module checks pass.
The import-only assertion also skips license assets, and the import rewriter is
restricted to Go source. The compiler policy stays at four CPU threads, 6 GiB
soft memory and 40 minutes. Full committed-source evidence remains required.

At `3849b9c`, Actions CI run `39446228834` passed all 11 jobs. The local
instrumented full Cloudflare run completed the generated client/compiler paths
and `TestCloudflareHTTPContract`, but its `-toolexec` Bash wrapper exceeded the
argument-list limit when starting the public models package. This is a failed
diagnostic run, not a full SDK pass. Normal Go uses a compiler response file for
that package. A subsequent ordinary `go test ./...` completed all 189 packages
and the HTTP contract with no failures on Go 1.27.3. It reused private-package
build cache from the instrumented run (444.74 seconds, peak RSS 14,274,660 KiB);
these conditions do not support a cold-build speedup claim. Package diagnostics
remain under `.tmp/provider-resolution-20261020/cloudflare-direct-model-*`.
Fresh Go 1.27.2 Actions evidence is still pending for Cloudflare. The other four
repaired providers have passed complete Go compilation and exact HTTP tests at
the same clean `3849b9c` source in run `39446233980`.

The fixed Go 1.27.2 Actions Cloudflare run also exceeded 40 minutes. This does
not invalidate the completed local Go 1.27.3 module/HTTP checks, but it leaves
that CI compilation unverified. The final measurement budget is therefore
60 minutes, still using serial packages, four CPU threads and 6 GiB soft memory.
The Actions step allows 75 minutes for generation plus compilation within the
existing 120-minute job. This is a disclosed budget calibration for the complete
large API, not an omission or a claim of faster compilation. Earlier budgets and
their failed outcomes remain in history and cannot enter the current aggregate.
The SDK source is unchanged from the locally verified `3849b9c` layout. Go
verifier tests, 22 Python evidence tests and documentation validation passed for
the revised policy. Final Actions compiler/HTTP evidence remains required.
