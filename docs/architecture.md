# openapi-sdkgen Architecture

This document describes the current implementation boundaries of
`openapi-sdkgen` for maintainers. The canonical user-visible capability contract
remains the
[OpenAPI feature manifest](openapi-feature-manifest.json), with the
[feature matrix](openapi-feature-matrix.md) and
[feature inventory](openapi-feature-inventory.md) as readable views.

## Product boundary

`openapi-sdkgen` accepts OpenAPI 3.0.x, 3.1.x, and 3.2.x documents and generates
application-owned TypeScript source. The generated source includes its source runtime and is compiled by the
consumer's existing TypeScript toolchain.

TypeScript is the active output target. The optional `server` add-on generates Fetch-native Webhook and Callback
contracts alongside the client output. Host applications connect those
contracts to their framework and HTTP stack.

Supported OpenAPI semantics are generated or preserved as documented metadata.
A construct that cannot be represented safely is rejected with a source-aware
diagnostic.

## Compatibility evidence boundary

The feature manifest and feature matrix are the canonical feature-level
contracts. Real-document corpora provide separate empirical evidence that those
contracts compose across complete inputs.

Two corpus classes are intentionally kept distinct:

- seven provider corpora are regression probes because they participated in
  compatibility development;
- `test/compatibility/holdout.json` is an independent 20-provider,
  size-stratified APIs.guru holdout selected deterministically from one pinned
  upstream tree before sdkgen pass/fail results are inspected.

The current corrected holdout has 90% default client-only end-to-end success and
100% capability-adjusted support when existing server generation is enabled for
documents containing inbound Webhooks or Callbacks. Those numbers are not a
population-wide compatibility claim. See
[OpenAPI compatibility architecture](openapi-compatibility-architecture.md#independent-20-provider-holdout-benchmark)
for selection, metrics, limitations, and the resolved arbitrary Schema-reference
gap. A separate provider-published OpenAPI 3.2 cohort has two documents from
Zenith Payments, both passing generation and strict verification with 45 emitted
operations. Large production 3.2 documents, external reference closures and
combined security/streaming contracts remain unobserved in that cohort.

Compatibility report version 2 separates compiler IR retention from emitted
operation coverage. Emitted counts come from the visible route manifest after
successful emission; blocked documents contribute zero. Operation and helper
omissions are reported separately. The current holdout emits 2,647 operations
under the default profile, with one diagnosed operation omission and 48 helper
omissions. The 2,829 retained IR operations also include the two blocked documents,
so emitted counts are the primary operation coverage measure. The original
version 1 report is stored separately as `holdout-results-pre-v9.json`.

## Pipeline

```text
input source
   │
   ▼
source loading and reference policy
   │
   ▼
OpenAPI decode, validation, resolution, normalization
   │
   ▼
target-neutral IR
   │
   ▼
TypeScript target preparation
   │
   ▼
semantic module plan and source emission
   │
   ▼
transactional output publication
```

The pipeline separates expected author errors from internal failures. Compiler
and target preflight findings are accumulated as structured diagnostics. Source emission begins after a safe compiler document and target plan exist.
Publication begins after target preparation succeeds.

## Input and reference layer

The compiler owns input acquisition and reference policy. Inputs may come from a
local file, `file:` URL, HTTP(S) URL, or standard input. Relative local
references are resolved within the permitted input root. Remote-reference network access requires an explicit trust policy.

Root HTTP(S) inputs trust the exact origin selected by the user. Root redirects
must remain on that origin (scheme, host, and port); they do not transfer trust
to another origin. Credential-bearing root requests keep credentials scoped to
the same boundary. Same-origin references may reuse the root transport policy.

Cross-origin remote references use a separate exact HTTPS-origin allowlist,
public-address validation, integrity lock, bounded fetching, a content-addressed
cache, and offline mode. Protected cache entries require owner-only protection
on platforms where that policy can be enforced.

Custom required JSON Schema vocabularies are compile-time extensions. Each
extension is registered from a trusted local manifest, integrity tracked, and
returns normalized schema data. Generated application code consumes the lowered
schema semantics.

## Compiler and normalized IR

The compiler owns OpenAPI-version semantics and reusable meaning shared by all
output targets. Its IR retains the lossless source document for metadata and
exceptional scans. Common HTTP semantics are normalized before target
preparation.

The normalized operation model includes:

- inherited and operation-level parameters, including resolved reusable
  Parameter Objects, style defaults, content media, and stable source pointers;
- resolved request bodies and request media;
- resolved responses and response media, while retaining the original response
  occurrence for source provenance such as Link diagnostics;
- effective Security Requirement Objects plus normalized component Security
  Schemes;
- effective root, path, or operation Servers with normalized variables and
  stable source-pointer IDs;
- operation extensions that require target-specific validation, such as
  visibility, pagination, envelope, and sort plans.

The raw OpenAPI tree is used for lossless metadata,
feature-preflight scans, custom extensions, and Webhook/Callback operation
surfaces outside the ordinary path-operation model. New common HTTP behavior
belongs in typed IR fields so targets consume one normalized representation.

Schema resources have their own normalized registry. Resource URI, dialect,
pointer identity, boolean schemas, anchors, dynamic references, and compiled
custom-vocabulary results are established before TypeScript lowering. Type and
wire projection can therefore share one reference model.

## Diagnostics

Diagnostics are structured data throughout compilation and target preparation.
A diagnostic can contain severity, stable code, pipeline phase, source and JSON
Pointer location, related locations, target, route, operation, message, hint,
and a sanitized cause.

The CLI renders human-readable diagnostics by default and can emit the versioned
JSON report for CI and tool integration. Source sanitization removes URL
credentials, queries, fragments, and arbitrary transport details before
rendering.

Skipped pipeline phases are reported when an earlier phase prevents safe
continuation.

## TypeScript target

The TypeScript target has two phases. `Prepare` validates target-specific
semantics and builds an immutable source plan. `Emit` or `EmitTo` turns that plan
into generated artifacts. Expected OpenAPI authoring problems belong in
preparation diagnostics; emission errors indicate internal failures.

Preparation owns target-specific concerns such as:

- public naming and collision handling;
- visibility and extension semantics;
- resource-tree and operation-ID call surfaces;
- schema input/output projections;
- error contracts, pagination, Links, and streams;
- semantic module ownership and import direction;
- optional Webhook and Callback generation.

Emission is split into schema, operation, route, resource, client-registry,
metadata, enum/error, runtime, and optional server artifacts. Generated paths
are validated for portability and collision safety before they reach the output
publisher.

The generated runtime is application source compiled with the rest of the SDK.
Client runtime modules keep transport, configuration, security, codecs,
pagination, Links, request construction, and HTTP execution behind internal
generated entry points. The server add-on uses its own Fetch-native runtime
surface and shares schema/wire semantics where applicable.

### Schema programs and runtime ownership

`schema/plan` lowers normalized schema resources into typed semantic nodes. The
same nodes drive descriptors and contract-specific validation/transformation in
`schema/emit`. Preparation interns programs by lowered meaning and execution
policy, shares equivalent input/output projections, and freezes their source and
dependencies. Reference target names remain in each contract's descriptor;
the same dispatch algorithm can be shared across different targets because it
reads the target from the owning descriptor. Slot identities apply this same
rule. Other semantic differences remain part of the execution identity.
Descriptors and program bindings are cached separately and frozen together.
Emission cannot add a previously unseen contract. Program modules
live under `internal/schema-programs/shared/`; their identities use the complete
SHA-256 digest encoded with portable base64url names.

Generated JSON contracts contain direct checks for their value types, numeric
bounds, lengths, required properties and children. They call selected canonical
operators for composition, patterns, dependencies, evaluation, content, formats
and dynamic scope. The generated executor dispatches these prepared programs;
it does not fall back to interpreting arbitrary schemas. XML/header/inbound
coercion uses prepared representation views and explicit child conjunctions.
The optional server routes prepare their own schema closure without changing
client artifacts or selective execution identity.

Authored runtime and generated `internal/runtime` use matching directories:

| Directory | Ownership and permitted dependencies |
| --- | --- |
| `shared` | JSON values, object utilities, transport metadata and common errors; shared only |
| `schema` | Schema types, execution ports, call-local state and schema operators; schema/shared |
| `stream` | Framing, abort and protocol contracts; stream/shared |
| `media` | Body, XML, multipart and codec contracts; media/schema/stream/shared |
| `security` | Authentication schemes; security/shared |
| `http` | Request, response and execution configuration; lower runtime layers |
| `client` | Callables, selection, loading, pagination and Links; HTTP and lower layers |
| `compatibility` | Explicit arbitrary-schema/full-capability helper composition |

Every internal object contract has one canonical type owner; forwarding modules
reuse that owner. Boundary checks include value imports, erased type imports,
re-exports, import types, dynamic imports and ownership cycles. They run on
authored runtime and actual feature-generated runtime/program/server artifacts.
The established generic operation loader's `import(url)` and public arbitrary
schema `server/runtime.ts` facade have explicit, tested roles.

Buffered callable imports also follow prepared input facts. The compiler chooses
one canonical binder for required input, no input, or optional input; emission
and runtime closure use the same decision. `client/callable-types.ts` owns the
callable contracts, `operation-binding.ts` owns method identity registration,
and separate modules own resource binding, stream binding, optional argument
resolution and namespace decoration. The generic `client/callables.ts` facade
composes these same owners for compatibility helpers. Generated buffered
operations reference their selected binder directly. Execution providers derive
their internal request type from the selected `bindBase` or `bindStream` parameter,
preserving its buffered or streaming contract without a repeated type import.
Public callable aliases, signature help and HTTP error provenance remain intact.

Resource calls select their remaining input binder independently from the exact
operation's input. Stream, Link and pagination owners follow prepared
capabilities. Path merging and method registration have canonical owners;
decoded, raw and stream calls share the same bound path object. Providers carry
the exact binder in their resource placement metadata. Generated named and
selective clients use it through the shared client assembly core. Generic
helpers retain metadata-based dispatch for arbitrary providers. Provider ABI 1
and generation identity checks are preserved.

Ordinary JSON entries omit the generic `wire-core` interpreter, branch selection
and evaluation collection when their selected contracts do not require them.
Shared call-local validation caches and finite-number checks remain necessary.
Preparation also reuses complete schema-reference closures across strongly
connected components, while preserving input/output projection boundaries.
Semantic-only analysis does not build unused TypeScript descriptor strings.
Supported arbitrary-schema helpers retain the canonical interpreter behind
their explicit compatibility roots. This is generated execution code, with no
runtime `eval`, generated-source AST pruning or new consumer dependency.

HTTP preparation also freezes path shape, structured-sort use, server variables
and the number of security requirements. The client root merges these facts;
named clients and execution providers use their own selected closure. Known
simple string paths, ordinary schema parameters, static server URLs and single
Bearer requirements select narrow policies. Unknown or compound contracts keep
the general policy. Buffered JSON response services retain actual undeclared
text, XML, binary and custom-codec fallback decoding.

`http/request` owns parameter encoding, URL construction and the buffered
executor. `http/response` owns body/error decoding and the optional streaming-raw
policy. HTTP security has a common provider/credential/collision orchestrator
with separate source, credential-dispatch and requirement-selection policies.
The existing general helpers compose these same canonical implementations.
Canonical port types own each injected policy; narrow modules do not import the
general facade. Differential tests compare emitted narrow and general graphs,
including required general policies in mixed root/named/provider scopes.

Repeated operation providers with the same prepared runtime features and stream
capability share assembly functions under `internal/execution-compositions/`.
Preparation assigns their paths from these semantic facts and registers only
groups used by multiple providers. A single provider retains inline assembly.
These generated client-layer modules import only selected canonical runtime
owners and initialize no module state. Each provider calls the factory at its
own initialization, preserving independent service instances; request contexts,
credentials, codecs and projected schema closures remain outside the factory.
Default and named client factories retain their existing composition path.
Emission consumes the frozen registry, including the identity-neutral artifacts
used to fingerprint selective loading. The execution ABI remains version 1;
the generation fingerprint changes with the emitted execution code.

Boundary checks also cover these generated assembly modules, rejecting captured
client/schema artifacts, reverse runtime dependencies and module initialization.
Regression tests exercise selection-stable paths, strict declarations, independent
instances, asynchronous credentials, custom codecs, mutable input revalidation
and concurrent cancellation.

The optional `schema-programs:perf` measurement checks shared programs on a
1,000-operation fixture, strict source, fresh declarations and actual browser
bundle calls. Runtime size measurements and their limits are published in the
[selection cost guide](guide/selection-benchmarks.md#small-api-runtime).

## Generated-output publication

The dedicated output subsystem owns publication. Target emission streams
artifacts into a rollback-safe staging area, which keeps publication memory
bounded for large generated trees.

A fresh generation publishes atomically into a previously absent output
directory. Managed incremental generation uses
`.openapi-sdkgen-manifest.json` to record generated-file hashes and, when
available, a generation fingerprint. Before replacing a managed file, the publisher verifies that its current
contents still match the previous manifest. Edited generated files fail closed
before publication.

Incremental publication preserves unchanged file identities and timestamps,
removes stale manifest-owned files, and leaves unmanaged files untouched.
Changes are staged and backed up so a failed commit can restore the previous
managed output.

Concurrent incremental writers use a non-blocking operating-system advisory
lock. Ownership follows the live file descriptor, so stale lock files remain
reusable after an abnormal process exit.

`generate --check` reuses the same compiler, target, artifact validation, and
managed-output comparison semantics without publishing changes. Without an
output directory it is a no-write compiler/target preflight. With a managed
output directory it also fails when generated content, paths, or generation
fingerprints have drifted.

## Capability contract

`docs/openapi-feature-manifest.json` is the canonical field-level capability
register. Every entry has a version scope, state, and executable evidence.
Conditional support, such as Webhooks and Callbacks under `--with server`, is
recorded in the capability contract. Source-only metadata fields declare
`requiresAddons: ["metadata"]`; generated JSDoc and version exports remain
available in the base client. This condition does not imply an operation omission
or a blocking diagnostic.

The manifest tests enforce feature IDs, supported states, version scopes, and
references to executable evidence. The manifest remains the source of truth;
the inventory and matrix provide readable views.

## Validation and performance

Repository validation uses separate cost tiers for ordinary development and
release workflows.

`devtools run ci:dev` runs the ordinary pull-request suites locally. GitHub CI runs
Go, TypeScript runtime, verification tools, and each pinned consumer compiler
version (5.7.3, 5.9.3, 6.0.3, 7.0.2) independently. The `Validate source` job
requires every suite to pass. CI prepares tools and directly invokes the same
scripts; it does not install devtools. Suites avoid repeated preparation and
full cross-product compiler checks. Coverage reports remain opt-in without
fixed percentage gates. Runner jobs use `ubuntu-24.04`.

`devtools run publish:release` requires clean `main`, validates its existing HEAD,
then atomically pushes `main` and the annotated tag. Dry runs perform the checks
without editing files, creating commits, or publishing. Checks cannot change
HEAD or leave the working tree dirty. A failed push removes only the local tag
created by that attempt; `--resume` dispatches the existing tag's workflow.
Releaseway standard generates GitHub release notes. Breaking changes and
migration instructions can be added there, and retries preserve published
release bodies. There is no mandatory changelog file or preparation commit.

`devtools run check:dev` also exercises runnable examples and release cross-builds
for macOS, Linux, and Windows. Release and npm distribution safety simulations
run in the verification-tools suite. Performance and corpus benchmarks remain
separate explicit commands.

Performance has its own acceptance gate. It tracks compile, prepare, emit,
publish, full-process, memory, fresh-publication, and incremental workloads
against checked-in regression thresholds. The baseline records its reference
commit, Go version, platform, and generated workload sizes. When supported
generation changes alter those sizes, measure an unchanged reference commit
independently before updating the baseline; keep the regression tolerances and
historical improvement targets. Validate the candidate with separate runs rather
than deriving its acceptance budgets from those same runs.

## Maintenance rules

When changing the generator, preserve these boundaries:

1. The compiler/IR owns OpenAPI-version and reusable HTTP semantics.
2. Target-specific API design, TypeScript naming, module planning, and source
   formatting belong in the TypeScript target.
3. Expected input problems become structured diagnostics before emission;
   unexpected errors remain internal failures.
4. The output subsystem owns generated-output ownership, path safety, locking,
   rollback, and incremental comparison.
5. Generated code remains application-owned source, and host applications supply
   framework integration.
6. New capability claims update the canonical feature manifest and executable
   evidence together.
7. Security-sensitive input, reference, credential, and publication paths fail
   closed when the safe interpretation is ambiguous.
