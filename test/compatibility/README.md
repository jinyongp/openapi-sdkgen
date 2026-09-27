# OpenAPI compatibility holdout benchmark

This directory contains a frozen external compatibility benchmark for
`openapi-sdkgen`. It is intentionally separate from the real-world corpora
that were used while developing OpenAPI compatibility behavior.

## Cohorts

The existing regression evidence covers GitHub, Stripe, GitLab, Cloudflare,
Microsoft Graph beta, DigitalOcean, and Twilio. Those inputs are useful
regression tests, but they are not independent holdout evidence because they
influenced implementation work.

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

### Operation retention

The denominator is the compiler IR operation set. The benchmark subtracts
unique operation-scoped `omit-operation` effects from compiler semantic
restrictions and target diagnostics.

This avoids guessing operation ownership from raw source syntax and prevents
multiple diagnostics for one omitted operation from being counted multiple
times.

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

## Frozen result

The committed manifest SHA-256 is
`3d0f2255ed74d4b49a64a8bd958624f80ede02a452b6947bb2eaedcfd3edeb30`.

The committed report is 180,324 bytes with SHA-256
`7f15c11638405a75fb6572d656d7a9a427f273a34ce43b55f6cb9b526a5c0fbf`.
An independent rerun produced the exact same bytes.

| Metric | Result |
| --- | ---: |
| Holdout documents | 20 |
| Default client-only success | 17 / 20 (85%) |
| Capability-adjusted support | 19 / 20 (95%) |
| Default generated documents passing strict TypeScript | 17 / 17 (100%) |
| Default discovery complete | 17 / 20 |
| Operations retained | 2,829 / 2,830 (99.96%) |
| Compatibility findings preserved | 37 / 38 (97.37%) |
| Feature detectors observed | 20 / 31 (64.52%) |

The eleven unobserved feature detectors are:

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

## Remaining holdout failure

One document remains unsupported after capability adjustment.

### eos.local

The document uses local `$ref` values from Schema Object positions to nested
Schema Objects under response schemas, for example a reference into
`#/paths/.../responses/.../schema/properties/...`.

OpenAPI 3.0 defines Reference Objects using JSON Reference and permits a
Reference Object wherever a Schema Object can be used. The referenced targets
here are nested Schema Objects. See the OpenAPI 3.0.3 Schema Object and
Reference Object rules:

https://spec.openapis.org/oas/v3.0.3

The current TypeScript target accepts component-schema references for its schema
registry model but does not lower these nested schema references. The holdout
therefore records 14 blocking `SDKGEN-E501/E507` diagnostics. This is a
generic schema-reference support gap, not a provider-specific exception.

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
cross-section of real OpenAPI documents. It is not a claim that 95% of all
OpenAPI documents, or 95% of popular APIs, are supported.

Important limitations:

- the sample is size-stratified, not popularity-weighted;
- OAS 3.2 has no real holdout member;
- no selected holdout contains an external `$ref`;
- runtime requests are not sent to third-party APIs;
- strict TypeScript checking verifies generated source consistency, not live
  provider wire behavior;
- the existing seven-provider regression cohort remains important for
  multi-file references and provider shapes that this holdout did not sample.

Use the holdout success, operation retention, compatibility preservation,
feature breadth, and regression-corpus evidence together when making
compatibility claims.
