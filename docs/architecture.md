# openapi-sdkgen Architecture

This document describes the current implementation boundaries of
`openapi-sdkgen`. It is a maintainer reference, not a feature roadmap. The
canonical user-visible capability contract remains the
[OpenAPI feature manifest](openapi-feature-manifest.json), with the
[feature matrix](openapi-feature-matrix.md) and
[feature inventory](openapi-feature-inventory.md) as readable views.

## Product boundary

`openapi-sdkgen` accepts OpenAPI 3.0.x, 3.1.x, and 3.2.x documents and generates
application-owned TypeScript source. The generated source is compiled by the
consumer's existing TypeScript toolchain; it does not require a separately
published runtime package.

TypeScript is the active output target. The optional `server` add-on generates
Fetch-native Webhook and Callback contracts alongside the client output. The
server add-on does not introduce framework-specific adapters.

The central correctness rule is that valid OpenAPI semantics are never silently
dropped. A supported construct is generated or preserved as documented
metadata. A construct that cannot be represented safely is rejected with a
source-aware diagnostic.

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
and target preflight findings are accumulated as structured diagnostics. Source
emission begins only after a safe compiler document and target plan exist, and
publication begins only after target preparation succeeds.

## Input and reference layer

The compiler owns input acquisition and reference policy. Inputs may come from a
local file, `file:` URL, HTTP(S) URL, or standard input. Relative local
references are resolved within the permitted input root. Network access for
remote references is explicit rather than implicit.

Remote references use an exact HTTPS-origin allowlist, integrity lock, bounded
fetching, a content-addressed cache, and offline mode. Credential-bearing root
requests never forward credentials to another origin or through a redirect.
Protected cache entries require owner-only protection on platforms where that
policy can be enforced.

Custom required JSON Schema vocabularies are compile-time extensions. An
extension is explicitly registered from a trusted local manifest, is integrity
tracked, returns schema data rather than executable TypeScript, and is never
required by generated application code.

## Compiler and normalized IR

The compiler owns OpenAPI-version semantics and the reusable meaning that should
not be rediscovered independently by an output target. Its IR retains the
lossless source document for metadata and exceptional scans, but common HTTP
semantics are normalized before target preparation.

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

The raw OpenAPI tree remains an intentional escape hatch for lossless metadata,
feature-preflight scans, custom extensions, and Webhook/Callback operation
surfaces that are not ordinary path operations. New common HTTP behavior should
prefer a typed IR field instead of adding another target-side traversal of
`map[string]any`.

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
JSON report for CI and tool integration. Source identities are sanitized before
they are rendered so URL credentials, queries, fragments, and arbitrary
transport details cannot leak through diagnostics.

Skipped pipeline phases are reported explicitly when an earlier phase prevents
safe continuation.

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

The generated runtime is source, not a dependency. Client runtime modules keep
transport, configuration, security, codecs, pagination, Links, request
construction, and HTTP execution behind internal generated entry points. The
server add-on has a separate Fetch-native runtime surface while reusing the
shared schema/wire semantics where applicable.

## Generated-output publication

Output publication is owned by the dedicated output subsystem rather than the
CLI command handler. Target emission can stream artifacts into a rollback-safe
staging area, so large generated trees do not need to be retained only for
publication.

A fresh generation publishes atomically into a previously absent output
directory. Managed incremental generation uses
`.openapi-sdkgen-manifest.json` to record generated-file hashes and, when
available, a generation fingerprint. Before replacing a managed file, the
publisher verifies that its current contents still match the previous manifest;
user edits to generated files therefore fail closed instead of being silently
overwritten.

Incremental publication preserves unchanged file identities and timestamps,
removes only stale manifest-owned files, and leaves unmanaged files untouched.
Changes are staged and backed up so a failed commit can restore the previous
managed output.

Concurrent incremental writers use a non-blocking operating-system advisory
lock. The lock file may persist, but ownership follows the live file descriptor,
so an abnormal process exit does not leave a permanently blocking stale lock.

`generate --check` reuses the same compiler, target, artifact validation, and
managed-output comparison semantics without publishing changes. Without an
output directory it is a no-write compiler/target preflight. With a managed
output directory it also fails when generated content, paths, or generation
fingerprints have drifted.

## Capability contract

`docs/openapi-feature-manifest.json` is the canonical field-level capability
register. Every entry has a version scope, state, and executable evidence.
Conditional support, such as Webhooks and Callbacks under `--with server`, is
recorded explicitly rather than changing the base client contract.

The manifest tests enforce feature IDs, supported states, version scopes, and
references to executable evidence. The inventory and matrix provide readable
views but do not replace the manifest as the source of truth.

## Validation and performance

Repository validation is layered so ordinary development and releases do not
need identical cost profiles.

`just agent ci` is the ordinary pull-request/main gate. It covers formatting,
vetting, Go tests/build/module integrity, TypeScript formatting/lint/typecheck,
conformance generation, generate-check behavior, and coverage without running
release publishing simulations.

`just agent check` is the broader integrated gate. In addition to the ordinary
quality checks it exercises release scripts and workflows, npm package/publish
contracts, and runnable examples. Release builds are also checked across the
supported macOS, Linux, and Windows architectures.

Performance has a separate acceptance gate. It tracks compile, prepare, emit,
publish, full-process, memory, fresh-publication, and incremental workloads
against checked-in regression thresholds. Performance baselines are treated as
guards against regressions rather than implementation targets.

## Maintenance rules

When changing the generator, preserve these boundaries:

1. OpenAPI-version and reusable HTTP semantics belong in the compiler/IR rather
   than being independently reinterpreted by a target.
2. Target-specific API design, TypeScript naming, module planning, and source
   formatting belong in the TypeScript target.
3. Expected input problems become structured diagnostics before emission;
   unexpected errors remain internal failures.
4. Generated-output ownership, path safety, locking, rollback, and incremental
   comparison stay in the output subsystem rather than the CLI or a target.
5. Generated code remains application-owned source with no mandatory runtime
   package or framework adapter.
6. New capability claims update the canonical feature manifest and executable
   evidence together.
7. Security-sensitive input, reference, credential, and publication paths fail
   closed when the safe interpretation is ambiguous.
