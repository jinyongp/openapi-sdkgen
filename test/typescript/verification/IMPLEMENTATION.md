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

## Remaining implementation

C3 fixed operation type names, C4 scoped property construction, C5/C6 scoped
aliases and C7 integrated validation remain pending. Production changes are
implemented and tested by work unit, not installed from archived prototype files.
