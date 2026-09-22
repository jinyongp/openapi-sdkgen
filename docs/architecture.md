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
recorded in the capability contract while the base client contract stays stable.

The manifest tests enforce feature IDs, supported states, version scopes, and
references to executable evidence. The manifest remains the source of truth;
the inventory and matrix provide readable views.

## Validation and performance

Repository validation uses separate cost tiers for ordinary development and
release workflows.

`just agent ci` is the ordinary pull-request gate and remains available for
manual validation. It covers formatting, vetting, Go tests/build/module
integrity, TypeScript formatting/lint/typecheck, conformance generation,
generate-check behavior, and coverage. Release publishing simulations belong to
the release path. `just release` runs the full release checks before atomically
pushing `main` and the release tag.

`just agent check` is the broader integrated gate. In addition to the ordinary
quality checks it exercises release scripts and workflows, npm package/publish
contracts, and runnable examples. Release builds are also checked across the
supported macOS, Linux, and Windows architectures.

Performance has its own acceptance gate. It tracks compile, prepare, emit,
publish, full-process, memory, fresh-publication, and incremental workloads
against checked-in regression thresholds. The baselines serve as regression
guards.

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
