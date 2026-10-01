# Compatibility results

These results show which fixed OpenAPI documents generated a TypeScript SDK and
passed strict typechecking. Real API documents and purpose-built feature checks
answer different questions, so their results are shown separately.

For the supported contract of each feature, see [OpenAPI support](./capabilities.md).

## Results at a glance

<CompatibilityResults />

**Default client** means generation without add-ons. **Required add-on** also
checks documents with Webhooks or Callbacks using `--with server`; documents that
already pass retain their default result. **Emitted operations** counts operations
actually delivered to the default SDK. A blocked document contributes zero, and
server-profile operations are counted separately in the downloadable reports.

A passing document completed diagnostic discovery, had no blocking diagnostics,
generated successfully, and passed TypeScript **7.0.2** in strict mode after
removing generated `@ts-nocheck` directives. This checks generated source, not
calls to the provider's live API. Consumer compiler support is a separate contract;
see [TypeScript requirements](../guide/getting-started.md).

The holdout was remeasured on **October 1, 2026**, against source changes after
v9.0.0. These are stored measurements, rather than results attributed to the
v9.0.0 release binary.

## What the document results tell you

### Independent holdout: general document compatibility

The 20-provider set was selected before generation from a pinned APIs.guru
snapshot. It excludes the seven providers used during development and takes five
documents from each of four file-size groups. Membership stays fixed across
reruns. It contains OpenAPI 3.0 and 3.1 documents, so it measures general document
compatibility across different sizes rather than new OpenAPI 3.2 features.

Listen Notes and UniCourt require the server add-on for their Webhooks and
Callbacks. They block default client-only generation and pass with
`--with server`. The other documents pass the default check, including `eos.local`
after the Schema `$ref` fix.

### Provider-published 3.2 documents: ordinary API shapes

The second set contains Zenith Payments' two publicly published API documents,
each declaring `openapi: 3.2.0`. Their original bytes were preserved. Both generate
and typecheck, covering ordinary API shapes such as authentication, local
references, and Links.

They do not use new 3.2 constructs such as `QUERY`, `querystring`, or `itemSchema`.
Their results establish compatibility with these published documents; they do
not establish production use of the new 3.2 semantics. One provider's two documents
are also too small a set to estimate compatibility across providers.

## How new 3.2 behavior is checked

The feature set combines a real Resend 3.1 document with nine authored documents
for normative behavior, reference regressions, and target boundaries. It exercises
3.2 features, streaming, and reference handling deliberately rather than waiting
for those combinations to appear in public API documents.

The corpus result above covers generation and strict typechecking. Separate
positive and negative conformance fixtures, serialization checks, and streaming
runtime tests check accepted inputs, rejected inputs, and actual behavior. The
authored normative fixtures derive from the OpenAPI specification; they are
project-written test cases.

Resend and the Webhook boundary fixture need `--with server`. The Fetch boundary
fixture diagnoses and omits `TRACE`, `CONNECT`, and `TRACK`, while its supported
operations generate. A document can therefore pass with a reported local omission;
document success does not mean every operation or helper was emitted.

Public production documents using new 3.2 constructs can add evidence as they
become available. The normative and runtime checks provide the current evidence
for those features. These differently selected sets are kept separate rather
than combined into one compatibility percentage.

## Inspect the evidence

Download the original reports and input manifests to inspect document-level
results, input identities, and detailed diagnostics.

<CompatibilityResults evidence />
