# OpenAPI compatibility holdout benchmark

This directory contains a frozen external compatibility benchmark for
`openapi-sdkgen`. It is intentionally separate from the real-world corpora
that were used while developing OpenAPI compatibility behavior.

## Cohorts

`modern.json` adds ten preselected inputs independently of the original holdout:
one official Resend OpenAPI 3.1.2 snapshot at commit
`8916b099d7f52b8552a2e1ef38a7f16fcd17864c`, three normative media/3.2 inputs,
four reference inputs, and two target-boundary inputs. Evidence kinds are
reported separately; these authored regression inputs are not a production
3.2 sample or independent holdout. The normative inputs follow the
[OpenAPI 3.2 specification](https://spec.openapis.org/oas/v3.2.0.html).
The snapshot comes from [Resend's official specification repository](https://github.com/resend/resend-openapi).

Membership, input SHA-256, exact OpenAPI version, revision, and trust policy are
fixed before benchmark execution. Auxiliary Link source/target files also have
pinned hashes. Compilation uses local snapshots and the default empty remote
allowlist. Locked remote trust/cache behavior is exercised separately by
`TestExternalResponseLinkLockedRemoteClosure`.

```sh
just agent compatibility-verify test/compatibility/modern.json test
just agent compatibility-benchmark test/compatibility/modern.json test .tmp/compatibility-modern.json 3m
```

The verifier checks pinned local files without a fetched-corpus receipt. Existing
source-backed holdout receipts retain their repository/commit/manifest checks.
`TestOpenAPI32NormativeCohortRuntime` covers QUERY, querystring, reusable media,
JSONL/JSON-seq framing, positional multipart, discriminator default mapping, and
XML node types. Security metadata has detector/compile evidence. The existing
SSE/reference fixtures have their own strict/runtime probes. A webhook remains
a default-client blocker with a server-addon profile, and forbidden Fetch
methods remain operation omissions; those declared boundaries are reported.
The expanded feature catalog reports missing observations as coverage gaps.

`modern-results.json` records the first completed implementation measurement:
three OpenAPI 3.1 inputs and seven OpenAPI 3.2 inputs, default-client success
8/10, and capability-adjusted success 10/10 with the server addon for Resend
and the webhook boundary. Default emission is 23 operations with three
forbidden-method omissions. IR retention remains 136/139; the two webhook
blockers explain why it is larger than emitted coverage. These proportions
describe this selected mixed evidence cohort, not general ecosystem support.

The existing regression evidence covers GitHub, Stripe, GitLab, Cloudflare,
Microsoft Graph beta, DigitalOcean, and Twilio. Those inputs are useful
regression tests, but they are not independent holdout evidence because they
influenced implementation work.

### Major provider results

`regression.json` makes those seven provider families available as a separate
published cohort. Official upstream revisions, entry hashes, and DigitalOcean's
multi-file inputs are pinned before execution. GitHub, Microsoft Graph beta,
and Cloudflare retain the exact input hashes from the earlier regression
evidence; the other providers use the upstream revisions listed in the manifest.
The generated `regression-results.json` records generation, strict typechecking,
operation emission, and SDK generation time for these inputs.

Materialize each entry's `sourceUrl` at its `input` path under
`.tmp/compatibility-regression`. For DigitalOcean, use the official repository
revision in its entry and preserve the specification tree at
`digitalocean/specification`; `files` pins the auxiliary JSON and YAML files.
Then verify and measure:

```sh
just agent compatibility-verify test/compatibility/regression.json .tmp/compatibility-regression
just agent compatibility-benchmark test/compatibility/regression.json .tmp/compatibility-regression test/compatibility/regression-results.json 10m
```

The public page lists this cohort first, in manifest order, and links to each
immutable upstream document. The independent holdout retains its own membership
and measurements.

### Provider-published OpenAPI 3.2 documents

`production32.json` freezes both downloadable specifications from Zenith
Payments' official [Merchant API reference](https://docs.zenithpayments.support/docs/integration-options/rest-api/openapi/v2/merchant-apis)
and [Customer API reference](https://docs.zenithpayments.support/docs/integration-options/rest-api/openapi/v3/customer-apis).
Their top-level `openapi` is `3.2.0`; service versions `v2` and `v3` are
separate. These are unchanged provider-published bytes captured on 2026-10-01,
with source URLs, byte counts, SHA-256, and trust policy frozen before generation.
The live download URLs are mutable; committed snapshots and hashes are the
reproduction inputs. No local conversion or version rewrite is applied.

```sh
just agent compatibility-verify test/compatibility/production32.json test
just agent compatibility-benchmark test/compatibility/production32.json test .tmp/compatibility-production32.json 3m
```

`production32-results.json` records generation and strict TypeScript success for
both documents, with 45 emitted operations and no operation/helper omissions.
This adds production-document evidence independently of authored fixtures:
**two documents from one provider**, rather than a two-provider holdout. The
20-provider holdout and its membership remain separate.

The 298,755-byte Merchant document contains API-key and HTTP Basic security;
the 27,093-byte Customer document contains HTTP Bearer security. Both contain
local references and Links. Their detectors observe 9/44 features. Neither
contains external `$ref`, sequential media, or the new 3.2 QUERY/querystring
constructs. Production evidence for documents at least 1 MB, multi-file reference
closures, and combined OAuth/streaming contracts remains a gap. Normative
fixtures exercise supported constructs separately and do not fill that
production-evidence gap. Further providers must have verifiable public source
and exact top-level version, frozen input/closure hashes and trust policy before
their generation results are inspected.

`holdout.json` freezes a separate 20-provider sample from
`APIs-guru/openapi-directory` commit
`f04b8d0bcd39c52e1cf3ad7a5fe744709832ae49`.

The holdout was selected before running `openapi-sdkgen`:

1. enumerate the complete `APIs` tree at the pinned commit;
2. keep `openapi.yaml` and `openapi.json` blobs;
3. exclude the seven regression-provider families above;
4. keep one lexicographically first OpenAPI file per provider;
5. stratify by input byte size:
   - small: < 50,000 bytes;
   - medium: 50,000–249,999 bytes;
   - large: 250,000–999,999 bytes;
   - xlarge: >= 1,000,000 bytes;
6. choose five evenly spaced lexicographic positions in each stratum with
   `floor((i + 0.5) * N / 5)`.

The candidate population was 482 providers. The final holdout contains five
documents in each size stratum. Membership is not changed after observing
generation results.

The pinned holdout contains 19 OpenAPI 3.0 documents and one OpenAPI 3.1
document. It contains no real OpenAPI 3.2 document, so this benchmark does not
claim external OAS 3.2 evidence.

## Reproduce

Fetch the exact upstream blobs and verify their Git blob IDs and byte sizes:

```sh
just agent compatibility-fetch
```

After the corpus exists, verify it without network access:

```sh
just agent compatibility-verify
```

Run the heavyweight benchmark offline:

```sh
just agent compatibility-benchmark
```

The default paths are:

- manifest: `test/compatibility/holdout.json`
- materialized corpus: `.tmp/compatibility-holdout`
- report: `.tmp/compatibility-benchmark.json`

The committed evidence is `holdout-results.json`.

## Metrics

The report deliberately has no composite score, grade, or pass threshold.

### Default document success

A document succeeds only when the default TypeScript client profile has:

1. completed diagnostic discovery;
2. no blocking diagnostics;
3. successful emission; and
4. successful strict TypeScript checking after generated `@ts-nocheck`
   directives are removed.

This answers: "Does the document generate with the default client-only target?"

### Capability-adjusted support

Documents containing top-level `webhooks` or operation `callbacks` receive a
second, provider-neutral verification using the TypeScript `server` add-on.
The default result is preserved separately.

This answers: "Can the current TypeScript target support the document when the
document's inbound contract requires the existing server capability?"

No provider name or corpus identity participates in this decision.

### Emitted operations

Use emitted operations as the primary operation coverage metric, alongside
document success and strict verification.

New reports use `schemaVersion: 2`; corpus manifests remain at version 1.
`operationEmission.count` counts visible exact routes in the manifest used by
successful TypeScript emission. Blocked documents have zero emitted operations.
Emission failures mark the metric unavailable. Typecheck and document success
remain separate, so emitted source that fails typechecking is still counted as
emitted, with its failed verification status visible.

`operationOmissions` counts diagnosed omitted operations. `helperOmissions`
counts distinct capability omissions and does not subtract their owning
operations. The same metrics appear per support profile and in the overall/cohort
summary. Profile counts are separate; capability-adjusted support does not add
server-profile operations to default-client coverage.

Consumers branch on the report version. A missing emission field in a historical
version 1 report means no measurement was made; it does not mean zero operations.

### Historical IR operation retention

The denominator is the compiler IR operation set. The benchmark subtracts
unique operation-scoped `omit-operation` effects from compiler semantic
restrictions and target diagnostics. It includes operations from documents whose
generation is blocked. Use it to explain compiler retention, not delivered SDK
coverage. Historical version 1 reports retain this meaning and their values.

### Compatibility semantic preservation

Only explicit `COMP-*` compatibility findings participate:

- preserved: `preserve`, `preserve-extension`, `normalize`, `ignore`;
- rejected: `reject`.

A document with no compatibility findings reports no preservation percentage
instead of an artificial 100%.

### Feature breadth

A fixed detector catalog records which OpenAPI/JSON Schema features are actually
present in the holdout. This measures input diversity, not correctness.

The catalog covers version lines, local/external references, request bodies,
callbacks/webhooks/links, media types and encodings, security scheme families,
and material schema composition/dialect constructs.

## Current measurement of the frozen holdout

The committed manifest SHA-256 is
`3d0f2255ed74d4b49a64a8bd958624f80ede02a452b6947bb2eaedcfd3edeb30`.

The post-v9 measurement on 2026-10-01 uses TypeScript 7.0.2. The committed report
is 236,314 bytes with SHA-256
`f3ad5b5297b2d196fda59b31631f7801a34bbc3838b3859b344b0c92f7ee14c3`.
The original version 1 report is preserved as `holdout-results-pre-v9.json`
(180,324 bytes, SHA-256
`7f15c11638405a75fb6572d656d7a9a427f273a34ce43b55f6cb9b526a5c0fbf`).

| Metric | Result |
| --- | ---: |
| Holdout documents | 20 |
| Default client-only success | 18 / 20 (90%) |
| Capability-adjusted support | 20 / 20 (100%) |
| Default generated documents passing strict TypeScript | 18 / 18 (100%) |
| Default discovery complete | 18 / 20 |
| Default emitted operations | 2,647 |
| Diagnosed operation omissions | 1 |
| Capability/helper omissions | 48 |
| Historical IR retention metric | 2,829 / 2,830 (99.96%) |
| Compatibility findings preserved | 37 / 38 (97.37%) |
| Feature detectors observed | 20 / 44 (45.45%) |

The detector catalog expanded from 31 to 44 entries; the lower feature proportion
does not mean a compatibility regression. The 24 unobserved detectors are:

- `operation.query`
- `parameter.querystring`
- `document.media-types`
- `media.item-schema`
- `media.jsonl`
- `media.json-seq`
- `media.multipart-positional-encoding`
- `schema.discriminator-default-mapping`
- `security.oauth2-metadata`
- `security.device-authorization`
- `security.deprecated`
- `security.requirement-uri`
- `schema.xml-node-type`
- `oas.version.3.2`
- `reference.external`
- `media.encoding`
- `security.openid-connect`
- `schema.anyOf`
- `schema.not`
- `schema.type-array`
- `schema.const`
- `schema.null-type`
- `schema.prefix-items`
- `schema.unevaluated-properties`

## Capability-required documents

Two default failures are supported by an existing add-on rather than representing
general incompatibility.

The two blocked default documents contribute zero emitted operations, although
their 182 operations remain in IR retention. Their separate server profiles
emit 24 and 158 operations. The 100% adjusted document result still permits
local helper omissions; it does not promise every optional helper is present.

### Listen Notes

The default client-only profile reports a document-blocking `SDKGEN-E505`
because the document contains top-level webhooks. It also reports one
capability-scoped `SDKGEN-W509` response-link omission.

With `server` enabled, diagnostic discovery is complete, generation passes,
and strict TypeScript passes. The one Link warning remains local to that helper.

### UniCourt

The default client-only profile reports four `SDKGEN-E505` findings for
callback contracts.

With `server` enabled, diagnostic discovery is complete, generation passes,
and strict TypeScript passes with no diagnostics.

## Resolved arbitrary Schema-reference gap

### eos.local

The document uses local `$ref` values from Schema Object positions to nested
Schema Objects under response schemas, for example a reference into
`#/paths/.../responses/.../schema/properties/...`.

OpenAPI 3.0 defines Reference Objects using JSON Reference and permits a
Reference Object wherever a Schema Object can be used. The referenced targets
here are nested Schema Objects. See the OpenAPI 3.0.3 Schema Object and
Reference Object rules:

https://spec.openapis.org/oas/v3.0.3

The v9 reference implementation lowers these arbitrary Schema locations while
preserving stable identities. The unchanged document now completes discovery,
generation, and strict TypeScript checking with no diagnostics and four emitted
operations. Its previous 14 `SDKGEN-E501/E507` blockers are retained only in the
historical report. No holdout document remains unsupported after capability
adjustment.

## Resolved holdout defect

The holdout previously exposed a generic TypeScript emitter defect on a resource
whose generated example contained `api.taxContract.delete()`. Operation-module
post-processing qualified schema references with a global `Contract.` string
replacement, which also rewrote the `Contract.` substring inside the ordinary
`taxContract.delete()` identifier chain.

Schema qualification now targets only generated
`Contract.ComponentInput<...>` and `Contract.ComponentOutput<...>`
references. The unchanged `gerermesaffaires.com` holdout document now
completes generation and strict TypeScript validation without provider-specific
logic.

## Interpretation boundaries

This holdout is evidence that the generator works across an independent
cross-section of real OpenAPI documents. It is not a claim that 100% of all
OpenAPI documents, or 100% of popular APIs, are supported.

Important limitations:

- the sample is size-stratified, not popularity-weighted;
- OAS 3.2 has no real holdout member;
- no selected holdout contains an external `$ref`;
- runtime requests are not sent to third-party APIs;
- strict TypeScript checking verifies generated source consistency, not live
  provider wire behavior;
- the existing seven-provider regression cohort remains important for
  multi-file references and provider shapes that this holdout did not sample.

Use the holdout success, emitted operations, compatibility preservation,
feature breadth, and regression-corpus evidence together when making
compatibility claims.
