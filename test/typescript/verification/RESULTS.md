# Preimplementation verification — 2026-09-25

## Decision

The reusable verification infrastructure is implemented. The scoped unbundled
runtime/descriptor/export checks passed on eight positive local fixtures and two
pinned real-world corpora. The new property-helper prototype is **not approved
for production adoption yet**: a repeatable unbundled Stripe import and repeated
client-construction regression remains. Fixed lexical type names and the planned
semantic/reference refactor are independent of this property-helper decision.

This is not a result for the full future Go alias allocator, nor completion of the
central workstream's 97-check production matrix. No production generator or runtime
source was changed in this work; the candidate lives in an isolated experiment.

## Reusable fixture ownership

`../fixtures/catalog.json` now supplies the existing conformance preparation and
the comparative runner. Existing fixture paths are retained. It registers eight
positive local cases, the existing negative diagnostics/golden case, and GitHub
and Stripe as explicit checksum-pinned external inputs. The new representation
fixture adds recursive/shared schemas, exact/prototype/Unicode/control keys,
opaque const data, empty shapes and invalid-request coverage.

The fixture README assigns responsibilities. The verification README defines the
measurement and comparison contracts. Existing protocol-specific conformance tests
remain in use; the comparative `surface` scenario does not claim HTTP coverage.

## Candidate provenance

Production baseline source revision: `883c41bd1f39ce4d2cac92ca852dc881b6c29b68`.

- Baseline binary: `.tmp/internal-review-v4/baseline-repo/.tmp/bin/openapi-sdkgen`.
- Baseline binary SHA-256: `66d280ee676713aceed81ae5d4a7a1ee6bf554070531932a2d1ae6f348e2b42d`.
- Candidate binary: `.tmp/preimplementation-candidate/.tmp/bin/openapi-sdkgen`.
- Candidate binary SHA-256: `61b2348df21bd38ce2972db41fa3207611a72244837d5e35d2476891f1e6d129`.
- Candidate changes: fixed operation-private type stems; identity WireProperty
  construction helper; dedicated `wire-properties` runtime module. This does not
  include the full planned semantic-section/Go allocator migration.
- The isolated setup driver is `.tmp/setup-preimplementation-candidate.mjs`; it
  reuses the earlier combined source snapshot, patches the helper boundary and
  now includes explicit runtime artifact registration. Its output is not a patch
  to the production worktree.

Inputs are pinned by the catalog, and the report records binary, input, harness,
catalog and lockfile hashes. The latest path-safety/fingerprint self-test hardening
did not alter the timing worker's measured interval. Raw earlier reports retain
the exact harness hashes used in those runs.

## Runs and evidence

| Run directory under `.tmp/preimplementation` | Scope                                                                                                              | Result                                                                                            |
| -------------------------------------------- | ------------------------------------------------------------------------------------------------------------------ | ------------------------------------------------------------------------------------------------- |
| `run-ey17oq`                                 | 8 local + GitHub/Stripe; 5 A/A and 10 A/B pairs each; 3 paired lifetime processes for representation/GitHub/Stripe | Contract checks passed; Stripe performance review flagged.                                        |
| `run-yh1oEN`                                 | Same baseline binary emitted to two directories; Stripe; 10 A/A + 20 interleaved comparison pairs                  | Control passed; no configured regression flags.                                                   |
| `run-oRFtRd`                                 | Independent replication on GitHub/Stripe; 10 A/A + 20 A/B pairs                                                    | Contract checks passed; Stripe regression reproduced. GitHub repeated-client signal was marginal. |

Each directory contains `report.json`, generator logs and separate source/ESM
outputs. Failed pilots are retained separately: `run-lyriSZ` (harness manifest v2
assumption), `run-daiIVP` (prototype omitted runtime artifact registration), and
`run-40AvNt` (pre-existing plain-map Webhook behavior). None is counted as a pass.

The performance result `review` is intentionally separate from correctness.
A zero process exit status does not turn `status: review` into an approval.

## Unbundled latency: independent 20-pair replication

All times below are milliseconds. These are nonminified, unbundled ES2022 ESM
outputs in fresh Node processes, not the older minified Vite bundle experiment.
There is no forced GC inside the measured interval. The first client is measured
directly; it is not replaced by a repeated-client median.

| Metric                                        | GitHub baseline | GitHub candidate | Stripe baseline | Stripe candidate |
| --------------------------------------------- | --------------: | ---------------: | --------------: | ---------------: |
| Module import median                          |         300.569 |          307.836 |         262.969 |          290.323 |
| First createClient median                     |          41.372 |           38.524 |          46.474 |           44.992 |
| Contiguous import-through-first-client median |         342.253 |          346.105 |         308.686 |          335.674 |
| Repeated createClient median                  |           5.739 |            5.965 |           8.652 |           10.404 |

Paired changes are computed from individual pairs, so they need not equal the
ratio/difference of the separately displayed medians:

- Stripe import: paired +33.185 ms / +12.77%; A/A-derived noise floor 17.071 ms.
- Stripe contiguous initialization: paired +31.178 ms / +10.33%; noise floor 19.939 ms.
- Stripe repeated client: paired +1.746 ms / +20.61%; noise floor 0.725 ms.
- GitHub repeated client: paired +0.365 ms / +6.63%, almost exactly its 0.365 ms
  review boundary; treat this as a marginal signal, not a decisive regression.
- First client construction itself did not regress in these samples. The relevant
  Stripe cost is unbundled import and repeated client construction.

The first full run also flagged Stripe (contiguous +9.58% and repeated +15.78%).
The same-binary/different-directory control did not: contiguous paired +1.81%,
repeated -0.37%, both below the review thresholds. These controls do not establish
a universal performance guarantee, but they justify keeping C4 behind a gate
instead of dismissing the repeatable candidate result as noise.

## Runtime boundary and lifetime results

- Existing module runtime **value export names** matched. The added helper module
  is explicitly allowed. This is not a complete TypeScript type-export comparison.
- Public graph checks include own keys/descriptors, object prototypes, callable
  name/length, Link byStatus leaves and alias sharing, in actual unbundled ESM.
- Representative fixture requests, ID-less Links, pagination, NDJSON, callbacks
  and exact-key recursive roundtrips matched their baseline. Existing conformance
  covers other protocol combinations.
- The representation Node wire leaf loads only its schema plus candidate
  `objects.js` and `wire-properties.js`; it does **not** load `codecs.js`. Its
  exported descriptor graph fingerprint matched baseline exactly.
- Instrumentation used separate memory workers. Ten rounds of scenario calls
  added **zero wireProperties calls** for representation, GitHub and Stripe.
- Construction remains at existing ownership boundaries: representation schemas
  initialize at module load; GitHub and Stripe inline descriptors also construct
  when clients bind. No per-request graph reconstruction was observed.
- Each memory process discarded 240 clients over six batches, with GC between
  batches; three baseline/candidate process pairs ran per lifetime corpus.
- Median late post-GC heap growth, baseline -> candidate: representation
  24,688 -> 25,120 B; GitHub -14,512 -> -14,752 B; Stripe 19,728 -> 2,928 B.
  No configured late-retention review flag occurred. This finite observation is
  not proof of the absence of every possible leak.
- Two live clients using different origins and synthetic credentials passed
  isolation checks. No provider network requests were made.

## Failures found and disposition

### Dedicated helper artifact registration

Adding a runtime source file alone does not publish it. The generator has an
explicit `runtimeTemplateArtifacts` list. The prototype initially omitted the new
helper and actual ESM import failed with ERR_MODULE_NOT_FOUND although generation
returned success. Registration was repaired only in the isolated prototype and
all later runs passed. Production C4 must change the template, artifact list,
imports and tests together; bundle-only output rewriting did not cover this.

### Existing prototype-sensitive Webhook lookup

With collision-fixture Webhook names, passing ordinary empty handler/route maps
can read an inherited `constructor`/`__proto__` value as a route and throw
`TypeError: template.startsWith is not a function`. Baseline and candidate match.
The runner records this result explicitly; it is not counted as successful routing
or as a new optimization regression. The valid unmatched-route test uses empty
null-prototype host maps and returns 404. This pre-existing runtime issue requires
a separate fix, not a silent change in a representation-verification task.

### Fixture type-test corrections

The new typed representation test supplies an own string `constructor` field,
including in its recursive child. Omitting it from the literal conflicts with the
inherited Function-typed constructor under the existing generated shape. The
negative Leaf test's `@ts-expect-error` is placed on the actual invalid property,
not above a multiline initializer. No generator typing rule was loosened.

### Verification harness hardening

The manifest audit now accepts the repository's supported v1/v2 formats with the
matching presence of generation identity, instead of assuming v1. Symlink tests
include live/dangling/root cases. Descriptor fingerprints do not invoke own
getters or a custom prototype constructor-name getter. Negative controls prove
that wrong keys, aliases, prototypes, names and descriptors fail comparison.

## Verification infrastructure status

`just agent ci` completed with exit0 after the final fixture type-test fixes.
Go formatting/vet/tests/build/module checks, TypeScript format/lint/runtime
checking/conformance, catalog-driven fixture generation, generate-check tests and
both coverage paths passed. Go coverage was82.7% against78%.

The final harness self-test run reported15 tests,15 passes,0 failures. Logs:
`.tmp/agent-logs/20260925T011829Z-verification-harness-self-tests.log` and
`.tmp/agent-logs/20260925T011926Z-verification-harness-self-tests.log`.

The hardened final smoke on collisions, baseline-oas31 and representation passed
its unbundled contract and lifetime checks: `run-98RCns/report.json`. It used one
pair for diagnostic execution only; its timing review flag is not a performance
conclusion and does not replace the10/20-pair results above.

Infrastructure status: CI PASS. Scoped prototype correctness: PASS. Current
property-helper performance adoption: REVIEW/HOLD. Final production approval: NOT
GRANTED. No production optimization source, commit or push was made.

## What may proceed

- C1 reusable fixture/harness work can be reviewed and committed independently.
- C2/C3 semantic/reference separation and fixed private type names remain separate
  from the property-helper runtime cost.
- C4 helper adoption is held for a targeted lifecycle/construction-cost adjustment
  and rerun, or an explicit reviewed acceptance of this new nonbundled tradeoff.
  Do not silently widen the gate because old bundled results improved.
- Full Go allocator, strict/declaration/IDE checks on its final implementation,
  publication/migration and the remaining production matrix still apply.
