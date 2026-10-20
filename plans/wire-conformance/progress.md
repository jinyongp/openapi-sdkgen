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
