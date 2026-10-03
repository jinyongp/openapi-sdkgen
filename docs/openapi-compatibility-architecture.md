# OpenAPI Conformance and Compatibility Architecture

> Status: investigation and implementation contract. This document describes
> required future compiler behavior and confirmed current gaps. It is **not** a
> claim that the behavior described here is already implemented.
>
> Investigation revision: `cf34600bac3e73b80d400c4737c46e7d04c23f0c`.

## Purpose

`openapi-sdkgen` needs four separate decisions for every OpenAPI construct:

1. Is the source construct conforming to the declared OAS line?
2. What normative disposition does that OAS line assign to the construct?
3. What compatibility action, if any, may the generator safely apply without
   misrepresenting the normative behavior?
4. Can the selected effective semantics be represented by the target?

The current pipeline often collapses these decisions into one fail-fast version
check or one target-preflight decision. That is too coarse. It rejects some
constructs that the specification requires consumers to ignore, while other
ignored/no-effect constructs reach IR and generated TypeScript and become
observable SDK behavior.

The compatibility architecture therefore separates author conformance,
normative disposition, compatibility action, semantic impact, and target
capability. Provider-specific repair logic is explicitly out of scope.

## Normative baseline

The supported feature sets are OpenAPI 3.0.x, 3.1.x, and 3.2.x. The
authoritative patch texts used by this investigation are:

- OAS 3.0.4: https://spec.openapis.org/oas/v3.0.4.html
- OAS 3.1.2: https://spec.openapis.org/oas/v3.1.2.html
- OAS 3.2.1: https://spec.openapis.org/oas/v3.2.1.html

OAS defines the `major.minor` pair as the feature set. Patch releases correct
or clarify the text and are not a new feature set. The existing
`Version30`/`Version31`/`Version32` model should therefore remain.

The normative text, not the informational OpenAPI JSON Schema, is authoritative
when the two disagree.

## Five independent classifications

Every compatibility rule must carry five independent classifications.

### 1. Author conformance

- `conforming`: the source syntax/structure is permitted by the declared OAS line.
- `nonconforming`: the source syntax/structure is not permitted or violates an
  applicable requirement.

`undefined` and `implementation-defined` are **not** author-conformance states.
They describe the specification's behavior for otherwise representable source
and belong exclusively to the normative-disposition axis.

### 2. Normative disposition

This records what the declared OAS line says, independently of what the
generator chooses for real-world compatibility:

- `defined`
- `ignored`
- `undefined`
- `implementation-defined`
- `not-defined`
- `invalid`

### 3. Compatibility action

- `preserve`: use the construct exactly as defined by the declared OAS line.
- `ignore`: retain it in exact source/provenance metadata but remove it from
  effective generation semantics.
- `normalize`: replace it with a proven equivalent construct before semantic
  compilation.
- `preserve-extension`: deliberately retain behavior beyond the normative
  disposition because the intended API behavior is explicit, the target can
  represent it safely, and dropping it would lose API behavior. This always
  records a compatibility-deviation finding and must never be described as
  portable OAS semantics. It does **not** force author conformance to
  `nonconforming`; OAS 3.0 DELETE `requestBody` is the key conforming-source /
  normative-ignored example.
- `reject`: stop because no safe interpretation has been proved.

Normative `ignored` does not mechanically imply compatibility `ignore`.
For annotation/no-effect fields the two normally match. For behavior-bearing
constructs, silently ignoring can be more dangerous than a documented
compatibility extension or a rejection.

### 4. Semantic impact

At minimum:

- `annotation`
- `validation`
- `wire`
- `routing`
- `security`
- `reference`
- `dialect`
- `metadata-only`

A field that is metadata-only in today's TypeScript target is not automatically
safe to ignore if the OAS gives it routing, security, reference, or dialect
meaning.

### 5. Target state

The existing target states remain useful but answer a different question:

- `generated`
- `metadata`
- `error`

A target state must not substitute for a consumer action. For example, a field
can be present in the exact source metadata while its consumer action is
`ignore`.

## Source document and effective semantic view

The compiler must preserve two views.

```text
exact decoded source
    │
    ├── provenance / generated metadata / diagnostics
    │
    ▼
compatibility analysis
    │
    ├── findings + transformation ledger
    │
    ▼
effective semantic view
    │
    ▼
reference discovery / bundling / normalization
    │
    ▼
OpenAPI model + IR
    │
    ▼
target capability validation
    │
    ▼
emit / transactional publish
```

The source-facing decoded document is immutable. The effective view may omit or
normalize nodes only through registered rules.

The current implementation does **not** actually keep these views separate:
`metadata.ts` renders `ir.Document.Raw`, and `Document.Raw` is the normalized /
possibly bundled tree produced after compiler normalization. Therefore source
fidelity needs an explicit data path rather than assuming `Raw` is the input.

Implementation contract:

- capture the entry document immediately after decode, before remote-reference
  absolutization, schema-extension lowering, bundling, or compatibility action;
- retain that source-facing decoded value as deterministic canonical JSON bytes
  (for example an IR field such as `SourceMetadataJSON`), not as a second
  long-lived `map[string]any` tree;
- keep `ir.Document.Raw` as the effective semantic tree used by IR/targets;
- make the metadata emitter consume the source-facing payload and preserve the
  current prototype-safe generated-object contract **and its useful TypeScript
  literal/readonly shape**; do not replace it with an untyped `JSON.parse`
  surface or a plain object literal that changes `__proto__` semantics merely to
  avoid retaining another decoded tree;
- populate the source-facing payload on every production compiler entry path
  (local path, file URL, HTTP(S), stdin/input-base, and in-memory compile APIs).
  Synthetic/internal tests that manually construct `ir.Document` may fall back
  to `document.Raw`, but a successful production compile must never rely on that
  fallback;
- treat this as decoded-document fidelity, not byte-for-byte preservation of
  YAML comments, anchors, formatting, or key order;
- keep exact fetched bytes separately for integrity/provenance where already
  required by the source cache/reference-lock path.

This representation is required to make source/effective separation real while
bounding large-corpus retained memory.

This is also an explicit public-contract correction. A minimal external-reference
probe on the current implementation shows `metadata.ts` contains the bundled
component tree rather than the entry source, while the public client reference
says `openapi.document` contains the source OpenAPI file/content used to generate
the SDK. After this workstream, `openapi.document` must reflect the decoded entry
document before bundling/compatibility normalization. That can change generated
metadata for users of external references. Treat it as a release-note and semver
review item; do not hide it as an internal refactor. The workstream does not
choose or publish a release.

## Why compatibility must control reference discovery

Current compilation detects and resolves external references before
`openapidoc.ReadParsed()` applies version semantics.

That ordering is observably wrong for ignored subtrees. The following fixtures
all fail in the references phase with `SDKGEN-E120` when a missing local
`$ref` is placed only inside a construct that OAS 3.0 requires consumers to
ignore:

- ignored GET `requestBody`
- ignored reserved `Accept` header Parameter
- ignored response `Content-Type` Header Object

An ignored subtree must not:

- open a local file;
- perform a remote request;
- require an origin allowlist;
- create or update a reference-lock entry;
- create a source-cache entry;
- produce an unresolved-reference diagnostic.

Compatibility visibility must therefore be decided as part of reference
traversal, before generic recursive discovery of nested references. This is a
staged process rather than a blanket rule that *no* reference may ever be
resolved first:

1. rules whose context is known from the occurrence itself may prune a subtree
   immediately;
2. a reference that *is itself the construct being classified* may need one
   controlled resolution under the existing containment/allowlist/lock policy;
3. after that construct is classified as `ignore`, references nested inside the
   ignored value are not traversed;
4. `preserve`, `normalize`, or `preserve-extension` values continue through the
   normal reference policy.

The zero-I/O guarantee applies to nested references that are only reachable
inside a subtree whose selected compatibility action is `ignore`. It does not
pretend that an outer reusable-object reference can always be classified
without resolving that occurrence.

This staged visibility decision must feed:

- `externalReferenceCount`;
- `validatedReferenceFileFilter`;
- remote reference resolution;
- libopenapi bundling;
- reference lock mutation.

It must apply recursively to every referenced document, not only the entry
document.

Specification Extensions and literal data fields use the same path-aware
reference-opacity contract. A `$ref` key below `x-*`, examples/default/value
payload data, or another opaque data field is data rather than an OpenAPI/JSON
Schema reference. Discovery, provenance, model validation, local/remote loader
adapters, and bundling all share that classification.

libopenapi still interprets raw `$ref` keys recursively, so sdkgen presents a
temporary reference-semantic view at the bundler boundary: opaque `$ref` keys
are reversibly renamed to per-compile collision-checked markers, the composed
bundle is produced, then only markers allocated by that compile are restored.
Exact source/effective metadata, remote cache content, and reference-lock hashes
always use the unescaped source. This prevents extension data such as
DigitalOcean `x-codeSamples` from causing file/network I/O without weakening
real reference containment, allowlist, TLS, lock, cache, or cycle policy.

The current source cache is a useful boundary: it already owns immutable bytes
and decoded YAML trees for local referenced documents. libopenapi also exposes
`DocumentConfiguration.LocalFS` and `RemoteURLHandler`. A safe design can
retain original snapshots in the source cache while presenting compatibility-
filtered bytes to the bundler through controlled local/remote loaders. This
avoids replacing the existing containment, origin allowlist, lock, cache, and
cycle protections.

## Normative consumer-semantics inventory

The following rules affect SDK generation semantics and must be represented in
the executable compatibility inventory.

### Reference Objects

#### COMP-REF-001 — OAS 3.0 Reference Object additional properties

OAS 3.0 Reference Objects cannot be extended; any additional property is
ignored. This includes ordinary fields and `x-*` fields in Reference Object
context.

Compatibility action for a true OAS 3.0 Reference Object:

- preserve `$ref`;
- ignore **every** sibling in effective semantics, including `x-*` fields;
- preserve ignored siblings only in exact source/provenance metadata;
- do not reinterpret arbitrary siblings as Parameter/Response/Schema fields or
  using 3.1 Reference Object semantics.

A generator-specific extension that needs executable behavior must be declared
on the referenced object type itself, not smuggled through a Reference Object
sibling.

For the observed GitLab `description` siblings this is a lossless ignore.

Current behavior is the opposite for ordinary siblings:
`validateOpenAPI30ReferenceObjects` reports `SDKGEN-E140`. GitLab 19.5
contains two `description + $ref` occurrences and fails at the first.

Important: Path Item Objects and Schema Objects require their own rules. A
`$ref` key does not by itself prove that an object is a Reference Object.

#### COMP-REF-002 — OAS 3.1/3.2 Reference Object fields

OAS 3.1 and 3.2 define `summary` and `description` siblings. Other added
properties are ignored.

`summary` or `description` has no effect when the referenced object type does
not permit the corresponding field.

Current generic reusable-component resolution is nonconforming: both
`internal/compiler/ir/build.go::resolveReusableComponent` and
`internal/target/typescript/references.go::resolveComponentObjectRecursive`
merge every sibling over the referenced object.

Confirmed fixture results:

- a reusable Parameter `$ref` with sibling `name: changed` and
  `required: false` changes the generated API from the referenced required
  `limit` parameter to optional `changed`;
- a reusable Response `$ref` with sibling `content` changes a referenced
  string response into an integer response.

Those siblings must be ignored in Reference Object context.

#### COMP-REF-003 — Path Item `$ref` siblings

Path Item Object `$ref` is not a Reference Object. OAS 3.0/3.1/3.2 says that
when the same Path Item field exists in both referenced and local objects, the
result is undefined.

Current `ResolvePathItem` applies sibling overrides. This is a deliberate
project behavior in an OAS-undefined area, not a portable OpenAPI semantic.

Required policy:

- non-conflicting siblings continue to be supported as a documented
  compatibility behavior;
- a direct field conflict between the referenced Path Item and the local Path
  Item is **fail-closed** because OAS defines the result as undefined;
- the conflict diagnostic points to both definitions and must not describe the
  rejected override as an OAS guarantee.

### Parameters

#### COMP-PARAM-001 — reserved request Header Parameters

In every supported OAS line, a Parameter with `in: header` and name
`Accept`, `Content-Type`, or `Authorization` is ignored.

Matching is case-insensitive because these names map to HTTP header fields.

Current confirmed behavior leaks all three into generated public
`headerParams` and runtime parameter definitions.

The effective view must remove these Parameter occurrences before their schemas
or references are traversed.

#### COMP-PARAM-002 — `allowEmptyValue`

`allowEmptyValue` applies only to query parameters. Where the selected
serialization combination is marked n/a, the field is ignored. Interaction
with the Parameter schema is implementation-defined.

The canonical feature manifest currently claims
`parameter.allowEmptyValue = generated`, but no production implementation
reads `allowEmptyValue`; it appears only in tests and documentation. The
current evidence proves acceptance, not generated semantics.

Implementation must first characterize runtime behavior for
`allowEmptyValue: true`, `false`/absent, and serialization combinations where
OAS says the field is ignored. Only then may the manifest either keep
`generated` with direct wire evidence or downgrade/correct the state. The plan
does not assume the desired result before that probe.

#### COMP-PARAM-003 — `explode` no-effect/undefined cases

For scalar values, `explode` has no effect. OAS 3.0/3.1 leaves some
`deepObject` combinations undefined; 3.2 explicitly makes `explode`
no-effect for `deepObject`.

These are serialization policy rules, not document-wide version errors.

#### COMP-PARAM-004 — Cookie through Header Parameter

Defining `Cookie` using `in: header` has undefined effect. The portable
representation is `in: cookie`.

This belongs in the undefined-policy inventory, not in the normative ignore
set.

#### COMP-PARAM-005 — Path-template / path-Parameter correspondence

Every path template expression must have a corresponding effective `in: path`
Parameter with the exact case-sensitive name, and a path Parameter must
correspond to a template expression. sdkgen validates this after canonical IR
and provenance are available so reusable/remote source locations and the exact
compiled operation owner are known.

A mismatch is nonconforming source input, but the invalid semantics are owned by
one operation. sdkgen therefore:

- emits `SDKGEN-W140` with `rule=COMP-PARAM-005`,
  `scope=operation`, `effect=omit-operation`;
- records a canonical operation owner pointer in the IR semantic restriction;
- does not rename, alias, or otherwise guess the intended path parameter;
- removes the operation from emitted call/resource/helper surfaces while
  retaining independently valid flat/artifact collision reservations;
- keeps ambiguous legacy/source-only ownership fail-closed as target
  `SDKGEN-E507`.

The post-IR analyzer is reported as
`openapi.path-parameter-conformance` in diagnostic coverage.

### Operation request bodies

#### COMP-BODY-001 — OAS 3.0 method applicability

OAS 3.0 normatively says `requestBody` is ignored for methods whose HTTP
semantics do not define use of a request body and explicitly calls out GET,
HEAD, and DELETE. Real-world corpora demonstrate that blindly implementing that
normative ignore would itself lose API behavior.

Pinned-corpus evidence:

- GitHub REST 2022-11-28 has 21 such bodies: 20 DELETE and one GET. All 20
  DELETE bodies carry meaningful payload semantics; 16 are explicitly
  `required: true`.
- Stripe has 297: 265 GET and 32 DELETE. The 265 GET bodies are optional empty
  form-object artifacts; seven DELETE bodies carry meaningful payload
  structure.
- GitLab 19.5 has two HEAD bodies carrying a required `ref` property.

Therefore the compatibility policy is impact-sensitive:

1. If an inline body is structurally empty (not required; no `$ref`; no
   non-empty schema constraints/properties/encoding that could affect the
   request), compatibility action is `ignore`.
2. Any top-level Request Body `$ref`, schema `$ref`, required body, or non-empty
   payload/encoding is treated as **potentially meaningful without following a
   nested reference merely to prove emptiness**.
3. For OAS 3.0 DELETE and OPTIONS, a potentially meaningful body uses
   `preserve-extension`. Generated Fetch runtime proof demonstrates exact
   method, body bytes, and content type for both methods. The action emits a
   compatibility-deviation warning; empty bodies may still be ignored.
4. For GET/HEAD/TRACE, a potentially meaningful body is `reject`; Fetch cannot
   provide a portable GET/HEAD body contract and TRACE request content is
   prohibited/unsupported. The complete owning operation is quarantined and the
   TypeScript target emits an operation-scoped omission warning rather than a
   document-global failure.
5. A referenced unsafe body is conservatively classified before nested reference
   traversal. Once semantics are preserved (for example OPTIONS), reachable
   references are traversed normally and unresolved references remain
   document-blocking.
6. No rule may silently move body fields into query/path parameters or otherwise
   guess the author's intended transport.

POST and PUT have defined payload semantics and are preserved. PATCH has
request-body semantics in RFC 5789 and is preserved. OPTIONS is retained only as
an evidenced compatibility extension: the generated Fetch request is covered by
an exact runtime wire test rather than inferred from the OpenAPI 3.0 wording.

OAS 3.1 changes the OpenAPI policy: request bodies on methods with poorly
defined semantics are permitted but discouraged. OAS 3.2 similarly permits
them with HTTP-semantics warnings. The compatibility extension above is
therefore specifically about older OAS declarations and real API intent.

For OAS 3.1/3.2, keep compatibility and target capability separate:

- DELETE and OPTIONS bodies are preserved by compatibility and may be generated
  because the Fetch request model can transmit them;
- a meaningful GET or HEAD body remains a conforming/allowed source construct,
  but the Fetch-native TypeScript target must reject it as a target-capability
  error rather than reclassifying the OpenAPI document as invalid;
- TRACE is not safely representable by the Fetch-native target at all; target
  preflight must reject the TRACE operation independently of whether it has a
  body, rather than generating code that fails only when invoked.

Current TypeScript planning applies these distinctions before emission:
unsupported operations are removed from `$operations`, `$routes`, resource
call surfaces, Links/callback ownership, and operation modules while collision
reservations for surviving public paths remain stable.

#### COMP-METHOD-001 — Fetch-native method capability

OpenAPI method validity and Fetch target capability are independent. The
TypeScript runtime ultimately constructs Fetch `Request` objects, and current
Node Fetch behavior permits `QUERY` and ordinary extension methods but rejects
`CONNECT`, `TRACE`, and `TRACK` before transport execution.

Target preflight must therefore validate the effective method token for ordinary
Path Item operations and OAS 3.2 `additionalOperations`:

- `CONNECT`, `TRACE`, and `TRACK` are TypeScript target-capability errors,
  regardless of whether the OpenAPI description can represent the operation;
- OAS 3.2 `query` / `QUERY` remains allowed by this Fetch capability gate;
- other additional-operation tokens remain eligible only when they pass the
  existing OpenAPI method-token/operation validation and are not in the
  Fetch-forbidden set;
- method capability diagnostics must happen before emit so generated code never
  defers a deterministic Fetch rejection until invocation.

Focused static corpus audit found no TRACE/QUERY/additionalOperations use in the
pinned GitHub, Stripe, Cloudflare, Microsoft Graph v1.0, Twilio core 2010, or
GitLab inputs. Graph beta static method counting timed out on the large source;
it remains covered only by the required fresh compiler corpus rerun and is not
counted as a clean static result.

### Security Requirement conformance

#### COMP-SEC-001 — undeclared Security Scheme references

For OAS 3.0 and 3.1, each Security Requirement name must identify a Security
Scheme declared under Components. This is source conformance, not TypeScript
target capability.

sdkgen validates the rule after canonical IR/provenance construction:

- an explicitly operation-declared requirement that names no declared scheme
  emits `SDKGEN-W140`, `scope=operation`,
  `effect=omit-operation`; only that operation is quarantined;
- a top-level requirement with the same defect emits `SDKGEN-E140`,
  `scope=document`, `effect=block`;
- operations inheriting an invalid root requirement are not independently
  narrowed;
- a scheme that exists but is malformed or unsupported by the selected target
  remains target-owned (for TypeScript, existing `SDKGEN-E508`);
- OAS 3.2 Security Requirement names may be Security Scheme URIs, so absence of
  a same-named component is not classified as `COMP-SEC-001`.

The post-IR analyzer is reported as
`openapi.security-requirement-conformance` in diagnostic coverage. Compiler
operation restrictions are consumed before target security/cookie analysis, so
the same quarantined route is not duplicated as E508.

#### COMP-LINK-001 — Link target identity

A response Link capability must identify exactly one target through a non-empty
`operationId` or `operationRef`. Missing both targets, declaring both targets,
or providing only an empty target identity is a source-conformance failure owned
by that Link capability, not by the base response operation.

The compiler therefore quarantines only the invalid Link occurrence with
`scope=capability effect=omit-capability`, preserves exact source/provenance,
and records the restriction in IR. Valid sibling Links and the base response
remain eligible. Reusable invalid Links are classified at each reference
occurrence so omission does not degrade into an unrelated missing-reference
failure.

A source-valid Link target form that the TypeScript target cannot represent is a
separate target capability omission (`SDKGEN-W509`); it is not reclassified as
a source-conformance failure and the target is never guessed or redirected.

### Responses


#### COMP-RESP-001 — response `Content-Type` Header Object

In every supported OAS line, a response header named `Content-Type` is
ignored. Matching is case-insensitive.

Current output includes it in generated raw-response header types and runtime
header descriptors.

The ignored Header Object must be removed before its nested references are
traversed.

### Encoding Objects

#### COMP-ENC-001 — Encoding `headers`

`Content-Type` inside Encoding `headers` is ignored because Encoding
`contentType` owns that meaning.

The entire `headers` field is ignored when the body media type is not a
multipart media type.

Current confirmed behavior:

- multipart Encoding `Content-Type` is retained in the generated runtime
  encoding plan;
- `application/x-www-form-urlencoded` Encoding headers are retained even
  though the field must be ignored.

#### COMP-ENC-002 — `style`, `explode`, `allowReserved` applicability

OAS 3.0 applies these RFC6570-style fields only to
`application/x-www-form-urlencoded`; they are ignored elsewhere.

OAS 3.1/3.2 permits the fields for form-urlencoded and multipart contexts with
additional precedence rules. For other media types they are ignored.

When OAS 3.1/3.2 explicitly uses `style`, `explode`, or
`allowReserved`, the Encoding `contentType` value can become no-effect
according to the relevant rule.

For multipart RFC6570-style serialization, `allowReserved` has no effect.

Current target preflight instead rejects a tested JSON request Encoding Object
with `SDKGEN-E502`. Normatively ignored fields must be removed upstream and
must never become target-unsupported diagnostics.

#### COMP-ENC-003 — OAS 3.2 encoding entry applicability

OAS 3.2 requires name-based Encoding entries with no corresponding instance
property to be ignored. Position-based entries with no corresponding instance
item are also ignored, including surplus `prefixEncoding` entries when the
instance is shorter.

These rules are data-dependent. They cannot be implemented only as a compile-
time tree deletion; the emitted runtime encoding plan must implement the
instance-dependent ignore semantics.

### Schema and media semantics

#### COMP-SCHEMA-001 — OAS 3.0 boolean schemas

A standalone boolean Schema is a 3.1+ capability, but OAS 3.0 explicitly allows
the `additionalProperties` Schema Object keyword to contain either a boolean or
a Schema Object. Those native OAS 3.0 booleans are not compatibility findings:
`true` remains an open additional-property space and `false` remains a closed
one, with exact source/effective semantics preserved.

`COMP-SCHEMA-001` applies only where OAS 3.0 requires a Schema Object and the
source instead supplies a boolean Schema. For those nonconforming positions both
boolean forms have proved OAS 3.0-compatible normalizations:

- `true → {}` preserves the public `unknown` type and unconstrained wire
  semantics;
- `false → {not:{}}` preserves the public `never` type after the target
  recognizes an exact negated-empty schema as unsatisfiable, and the generated
  runtime rejects both request and response values.

The original source boolean remains intact in source metadata. The pinned
GitHub and Cloudflare corpora provide real-world coverage for the distinct
native `additionalProperties: true|false` forms and must not produce
`COMP-SCHEMA-001` solely for those keyword values.

#### COMP-SCHEMA-002 — OAS 3.1 `const` in a 3.0 document

`const: V` has the OAS 3.0-compatible representation `enum: [V]`.

Current output shows the same literal TypeScript input type for a native 3.1
`const` and the lowered 3.0 `enum`, but implementation still requires
runtime validation/wire equivalence tests before the normalization is approved.

#### COMP-SCHEMA-003 — nullable type arrays

The narrow 3.1 form `type: [T, "null"]` has a candidate OAS 3.0 lowering to
`type: T, nullable: true` when exactly one non-null OAS 3.0 scalar/container
type is present and sibling semantics remain equivalent.

The tested `string | null` pair produces the same generated public type and
wire descriptor after that lowering.

General type arrays must not be blanket-lowered. Multiple non-null types,
overlapping `integer`/`number`, or sibling constraints require a separate
proof or rejection.

#### COMP-SCHEMA-004 — numeric exclusive bounds

A 3.1 numeric `exclusiveMinimum: N` or `exclusiveMaximum: N` has a
candidate OAS 3.0 lowering using the corresponding bound plus boolean exclusive
flag.

The simple tested exclusive-minimum case produces the same runtime descriptor.
Combined pre-existing bounds require explicit bound algebra and equivalence
tests; otherwise reject.

#### COMP-SCHEMA-005 — JSON Schema 2020-12 keywords in OAS 3.0

The current single `openAPI31SchemaKeywords` set is too coarse.

At minimum split into:

- annotations that do not affect generated validation/wire semantics, such as
  `$comment` and schema `examples`: ignore from effective semantics while
  preserving exact source metadata;
- exact-lowering candidates such as `const`: normalize only with proof;
- reference/dialect keywords (`$id`, `$schema`, `$anchor`,
  `$dynamicAnchor`, `$dynamicRef`, `$vocabulary`, `$defs`): reject
  under OAS 3.0 unless a dedicated equivalent transformation is proved;
- validation/applicator keywords such as `contains`, conditionals,
  dependent/unevaluated/property-name/pattern-property semantics: reject unless
  the target-neutral compiler gains a proven OAS-3.0 equivalent.

#### COMP-SCHEMA-006 — OAS 3.1+ legacy `nullable`

Current behavior is correct in principle: OAS 3.1/3.2 accepts the raw annotation
but does not apply OAS 3.0 nullable semantics. Keep this as an explicit
compatibility rule rather than an accidental target special case.

#### COMP-SCHEMA-007 — `format` and binary content across version lines

OAS 3.1 states that `format` no longer controls content encoding; JSON
Schema `contentEncoding` / `contentMediaType` carry that vocabulary.

Current `isBinaryMedia` treats `format == "binary"` as wire-binary in all
versions. A custom media type with `format: binary` produces `BinaryBody`
and `ReadableStream<Uint8Array>` in 3.0, 3.1, and 3.2 fixtures.

Binary/media classification must become version-aware. Existing media-type
semantics such as `application/octet-stream` remain independent signals.

#### COMP-SCHEMA-008 — contradictory `contentMediaType`

OAS 3.1/3.2 states that Schema `contentMediaType` is ignored when it
contradicts the applicable Media Type Object or Encoding Object.

This is a contextual media/schema rule and cannot be implemented by a
schema-only walker without the enclosing media context.

#### COMP-SCHEMA-009 — XML no-effect rules

OAS 3.0/3.1 Schema `xml` has no effect on a root schema; it applies to
property schemas.

OAS 3.2 adds `nodeType`-specific no-effect/ignore rules, including cases
where XML `name` is ignored.

These belong in version-aware XML normalization/runtime semantics, not generic
metadata handling.

### Component reachability

Objects placed in Components have no API effect unless referenced. The exact
source metadata may still retain and expose unreferenced components, and the
project may deliberately export standalone schema types.

Therefore “unreferenced component” is not a generic tree-deletion rule. It is a
reachability distinction between API semantics and source/type metadata.

## Version-gate classification

Current compilation applies proven compatibility ignores/normalizations before
the remaining declared-version gates. A version-gated construct that is still
invalid for the declared OpenAPI line is reported as `COMP-VERSION-003` with
`scope=document effect=block`; these failures are intentionally not converted
to operation/capability omissions because their semantic ownership is the
declared document version.

| Version gate | Compatibility classification |
| --- | --- |
| OAS 3.0 missing `paths` | reject: required structural contract |
| 3.0 `webhooks` | reject: API/routing semantics |
| 3.0 `jsonSchemaDialect` | reject: dialect semantics |
| 3.0 `info.summary` | ignore for effective SDK semantics + conformance finding; preserve exact metadata |
| 3.0 `license.identifier` | ignore for effective SDK semantics + conformance finding; preserve exact metadata |
| 3.0 `components.pathItems` | reject: reference/routing semantics |
| 3.0 `mutualTLS` | reject: security semantics |
| <3.2 `$self` | reject: base/reference identity |
| <3.2 `components.mediaTypes` | reject: reusable wire semantics |
| <3.2 Security Scheme `deprecated` | ignore annotation + conformance finding |
| <3.2 `oauth2MetadataUrl` | reject: security/discovery semantics |
| <3.2 `deviceAuthorization` flow | reject: security-flow semantics |
| <3.2 Server `name` | ignore annotation + conformance finding |
| <3.2 Tag `summary` | ignore annotation + conformance finding |
| <3.2 Tag `parent` / `kind` | ignore SDK semantics + conformance finding; preserve raw metadata |
| <3.2 Example `dataValue` / `serializedValue` | ignore generated semantics + preserve raw example metadata |
| <3.2 Path Item `query` / `additionalOperations` | reject: operation/routing semantics |
| <3.2 `querystring` Parameter / cookie style | reject: request serialization semantics |
| <3.2 Response `summary` | ignore annotation + conformance finding |
| <3.2 Media `itemSchema` / `prefixEncoding` / `itemEncoding` | reject: streaming/wire semantics |
| <3.2 discriminator `defaultMapping` | reject: schema branch-selection semantics |
| <3.2 XML `nodeType` | reject: XML wire semantics |
| 3.2 `nodeType` + deprecated XML conflicts | reject: explicitly incompatible representation |
| 3.0 numeric exclusive bound | conditional normalize with equivalence proof; otherwise reject |
| 3.0 type array | narrow conditional normalize only; otherwise reject |
| 3.0 JSON Schema 2020-12 keywords | split per COMP-SCHEMA-005 |
| 3.0 Reference Object sibling | ignore, not fatal |

This table is a semantic policy. It does not require every safe-ignore finding
to be a user-visible warning in the first implementation. Diagnostic rendering
is a separate presentation decision.

## Undefined and implementation-defined inventory

These cases must never be described as universally portable behavior.

| Case | OAS classification | Project requirement |
| --- | --- | --- |
| Path Item `$ref` conflicting siblings | undefined | document chosen policy; never call it OAS-defined |
| Header Parameter named `Cookie` | undefined | fail closed or explicitly document project behavior |
| unsupported/deepObject parameter combinations | undefined | do not invent serialization |
| `allowEmptyValue` + schema interaction | implementation-defined | choose and test one policy before claiming support |
| empty Request Body `content` in 3.1+ | implementation-defined | explicit target policy required |
| serialization ordering where media rules do not specify order | implementation-defined | deterministic order is acceptable but must be documented |
| ambiguous Link parameter names | implementation-defined | prefer qualified interpretation where required by OAS guidance |
| discriminator configurations outside defined patterns | undefined | reject instead of guessing |
| discriminator mapping that is both schema name and URI | implementation-defined | document resolver precedence |
| non-JSON scalar conversion | implementation-defined | deterministic codec policy with runtime tests |
| cross-document implicit Security Scheme name resolution | implementation-defined | document entry-vs-referenced-document policy and test it |

## Pre-compatibility baseline mismatches

The table below is retained as investigation history: every row was reproduced
before the compatibility/failure-isolation implementation. It is **not** a list
of current HEAD behavior. Current contracts are the executable rules, feature
manifest, and post-change corpus evidence in the following sections.

| Finding | Baseline result |
| --- | --- |
| OAS 3.0 Reference Object `description + $ref` | fatal `SDKGEN-E140`; should ignore sibling |
| OAS 3.1 Parameter Reference Object with `name/required` siblings | sibling overrides referenced Parameter and changes public API |
| OAS 3.1 Response Reference Object with `content` sibling | sibling overrides response schema and changes output type |
| reserved request headers | generated into public `headerParams` and runtime plan |
| OAS 3.0 GET request body | generated into BodyInput/runtime plan |
| OAS 3.0 OPTIONS request body | generated into BodyInput/runtime plan despite no defined HTTP use |
| OAS 3.0 TRACE request body | generated into BodyInput/runtime plan despite HTTP prohibition |
| response `Content-Type` Header Object | generated into RawResponse header contract/runtime plan |
| multipart Encoding `Content-Type` header | generated into encoding runtime plan |
| urlencoded Encoding `headers` | generated although field is multipart-only |
| JSON request Encoding Object | fatal target `SDKGEN-E502` although relevant fields are ignored by OAS |
| ignored subtree containing missing local `$ref` | fatal `SDKGEN-E120` before semantics |
| OAS 3.0 boolean schema | accepted; 3.1 JSON Schema semantics leak into 3.0 |
| OAS 3.1/3.2 custom media `format: binary` | still treated as wire-binary |
| `allowEmptyValue` capability evidence | manifest says generated; production code never consumes the field |

## GitLab 19.5 as a real-corpus probe

The exact local GitLab REST API 19.5 document declares `openapi: 3.0.0`.

Observed compatibility footprint:

- two OAS 3.0 Reference Object `description + $ref` siblings;
- two OAS 3.0 HEAD operations with `requestBody`;
- no tested reserved request-header Parameters;
- no tested response `Content-Type` Header Objects;
- no 3.1 Schema keywords/type arrays/numeric exclusive bounds/boolean component
  schemas in the component-schema audit;
- two independent path-template/Parameter name mismatches.

On current HEAD, OAS 3.0 Reference Object siblings—including
Schema-or-Reference positions—are removed from effective semantics while
remaining in source metadata. The two meaningful HEAD bodies are quarantined as
operation omissions.

A full path-template audit finds exactly two independent mismatches:

1. GET `/api/v4/jobs/{id}/sbom_scans/{sbom_digest}`:
   expected `sbom_digest`, declared `sbom_scan_id`.
2. POST `/api/v4/groups/{id}/(-/)epics/{epic_iid}/issues/{epic_issue_id}`:
   expected `epic_issue_id`, declared `issue_id`.

These are not safe normalization candidates. sdkgen does not rename either
parameter. Instead the post-IR `COMP-PARAM-005` analyzer assigns exact
operation ownership, emits two `SDKGEN-W140 scope=operation
effect=omit-operation` findings, and removes only those malformed operations
from generated call/resource/helper surfaces.

The pinned GitLab 19.5 corpus therefore completes generation and strict
TypeScript validation with `0 errors / 4 warnings`: two
`COMP-PARAM-005` omissions and two `COMP-BODY-001` HEAD-body omissions.
All compiler and target analyzers report complete coverage.

## Real-corpus impact and existing boundaries

Real corpora prove that compatibility action cannot be derived mechanically from
the OAS normative disposition.

| Corpus | Frozen identity / baseline | Compatibility footprint found by this investigation | Required post-change interpretation |
| --- | --- | --- | --- |
| GitHub REST 2022-11-28 | SHA-256 `d39842ee4d43d701e8a8c5483b4afa7c23218518f943dea2e42d36a3798cbdcc`, 12,891,411 B, OAS 3.0.3 | 20 meaningful DELETE bodies, one non-meaningful GET body, one response `Content-Type` Header, and native boolean `additionalProperties` values; no audited Reference siblings, reserved request headers, path mismatch, general type array, or numeric exclusive-bound mismatch | Empty GET body and response `Content-Type` are lossless ignores. Meaningful DELETE bodies require an evidenced `preserve-extension`; native boolean `additionalProperties` values are preserved without COMP-SCHEMA-001. Corpus must remain full-generation/strict-TypeScript successful. |
| Stripe SDK spec | SHA-256 `2c31317cdff103e4495b5b3501004d9ddc0af61f43b0ab819e2db392eef008f6`, 4,518,735 B, OAS 3.0.0 | 265 optional empty GET form bodies, 32 DELETE bodies of which seven are meaningful; no other audited mismatch | Empty GET/DELETE artifacts are lossless ignores; seven meaningful DELETE bodies require `preserve-extension`. Corpus must remain successful. |
| GitLab REST 19.5 | SHA-256 `06db53616968cb3b30d5064cfcdfcfa3bbd3923118c837f73ecc7370feaef236`, 3,794,471 B, OAS 3.0.0 | two Reference Object `description` siblings; two meaningful HEAD bodies; exactly two path-template/Parameter-name mismatches | Final rerun is `0E/4W`: two HEAD-body `COMP-BODY-001` omissions plus two `COMP-PARAM-005` operation omissions. Coverage is complete and generation/strict TypeScript pass. |
| Cloudflare | SHA-256 `179f1cd2bb3921aad64f9dcf05d45a0f9b9905fb2c1ea3ca2aabef53383ed2b3`, 26,098,205 B, OAS 3.0.3 | 32 meaningful DELETE bodies, one meaningful required GET body, four explicit operation Security Requirements naming undeclared schemes, recursive JSON-value schemas | Final rerun is `0E/37W`: 33 `COMP-BODY-001` findings plus four `COMP-SEC-001` operation omissions. Recursive JSON projections and bounded generated-operation binding remove the previously masked TS2456/TS2589 failures; coverage is complete and generation/strict TypeScript pass. |
| Microsoft Graph v1.0 | OAS 3.0.4 local corpus | no Reference-sibling, reserved-header, OAS-3.0 body, or response-`Content-Type` occurrence in the focused audit | Existing internal TypeScript preparation failure remains independent unless direct rerun evidence changes it. |
| Microsoft Graph beta | current local SHA-256 `46bead9a6459cbe5f66d31094b23a614eb8da8f46690ee61404d97523e23daec`, 69,965,060 B, OAS 3.0.4 | current snapshot contains no remaining diagnostic findings after named-map-aware context correction | Final rerun is `0E/0W`, coverage complete, generation and strict TypeScript pass. |
| DigitalOcean source tree | entry SHA-256 `9601e8c39dde0bdbe9a7ed97b948f71ffd01da3025721fdb31e82aad7490923e`, OAS 3.0.0; heavily fragment/reference based | extension/example `$ref` values are opaque data; 459 independently owned path-template/Parameter mismatches remain in reachable operations | Opaque `x-codeSamples` references no longer trigger E120/file I/O. Final rerun is `0E/459W`, all `COMP-PARAM-005` operation omissions; coverage complete and generation/strict TypeScript pass. |
| Twilio core 2010 | SHA-256 `170b3ccd0f891416840083d72f1795b1499b14a18d4873fd2b39f47ef84642d6`, 1,877,664 B, OAS 3.0.1 | zero focused compatibility occurrences | Generation and strict TypeScript now pass after the generic numeric-leading identifier normalization fix; no Twilio-specific compatibility branch is required. |

## Independent 20-provider holdout benchmark

The seven corpora above are regression evidence. They are intentionally **not**
treated as independent generalization evidence because they participated in
compatibility development and review.

A separate holdout benchmark is frozen in
`test/compatibility/holdout.json` from APIs.guru
`openapi-directory` commit
`f04b8d0bcd39c52e1cf3ad7a5fe744709832ae49`. The selector is deterministic and
does not inspect sdkgen results:

1. enumerate every `openapi.yaml` / `openapi.json` blob under the pinned
   `APIs` tree;
2. exclude the seven regression-provider families above;
3. sort paths lexicographically and retain the first OpenAPI document per
   provider;
4. partition the resulting 482 providers by document size;
5. select five evenly spaced lexicographic positions from each of four size
   strata.

The resulting 20 providers are therefore a fixed size-stratified holdout rather
than a pass-selected corpus. Fetching verifies the pinned Git blob identity and
byte size for every document. Benchmark execution re-verifies the materialized
corpus before measuring it, runs under the repository-pinned Node environment,
and records a deterministic JSON report in
`test/compatibility/holdout-results.json`.

Current corrected holdout evidence:

| Metric | Result |
| --- | ---: |
| Documents | 20 |
| Default client-only end-to-end success | 18 / 20 (90%) |
| Capability-adjusted support | 20 / 20 (100%) |
| Strict TypeScript among default generated documents | 18 / 18 (100%) |
| Default emitted operations | 2,647 |
| Diagnosed operation omissions / helper omissions | 1 / 48 |
| Historical IR retention metric | 2,829 / 2,830 (99.96%) |
| Explicit compatibility findings preserved | 37 / 38 (97.37%) |
| Benchmark feature detectors observed | 20 / 44 (45.45%) |

`capability-adjusted support` does not hide the default result. Documents with
top-level Webhooks or operation Callbacks are additionally verified with the
existing TypeScript `server` add-on. In the current holdout, Listen Notes and
UniCourt fail the default client-only profile because inbound contracts require
that add-on, then both pass generation and strict TypeScript with `server`
enabled.

`eos.local` uses local Schema Object references to nested Schema Objects under
response schemas. The v9 implementation lowers these arbitrary Schema locations.
The unchanged document now completes discovery, generation and strict checking
with four emitted operations and no diagnostics. No document remains unsupported
after capability adjustment.

The current version 2 report measures emitted operations from the actual output
manifest. The two default blockers contribute zero emitted operations; their
182 IR operations explain why retention is higher than default emission.
The separate server profiles emit 24 and 158 operations. Local helper omissions
remain visible even when a document succeeds. The catalog expanded from 31 to
44 feature detectors; its lower observed proportion is a broader measurement,
not a compatibility regression. The pre-v9 report is retained separately as
`test/compatibility/holdout-results-pre-v9.json`.

The holdout also exposed and now regression-tests a generic emitter bug: schema
post-processing previously replaced every `Contract.` substring in an
operation module, so an ordinary generated call such as
`api.taxContract.delete()` could be corrupted into a schema namespace
reference. Qualification is now restricted to generated
`Contract.ComponentInput<...>` and `Contract.ComponentOutput<...>` tokens.
The unchanged `gerermesaffaires.com` holdout document now generates and
strict-typechecks successfully without provider-specific behavior.

The holdout currently contains 19 OAS 3.0 documents and one OAS 3.1 document.
It contains no OAS 3.2 document and no selected external-`$ref` occurrence.
Accordingly, the 100% capability-adjusted result must **not** be interpreted as
"100% of all OpenAPI documents" or "100% of popular APIs." It is empirical
evidence over this fixed independent cross-section.

A separate `test/compatibility/production32.json` cohort pins two unchanged
OpenAPI 3.2.0 documents from Zenith Payments' official API reference. Both pass
generation and strict TypeScript with 45 emitted operations and no omissions.
They are two documents from one provider, with API-key/HTTP security and local
references, and have no external reference or streaming observation. This
adds production evidence without claiming large-document, multi-file, or
security/streaming coverage. Source provenance, reproduction commands, and
remaining gaps are recorded in `test/compatibility/README.md`.

The feature manifest remains the canonical feature-by-feature contract. The
holdout benchmark answers a different question: whether complete real external
documents survive diagnostic discovery, generation, and strict target
verification without provider-specific behavior.

The previous integrated workstream's historical failures are evidence, not
expected-output strings that the new pipeline must artificially preserve. A new
compatibility rule may expose an earlier, more precise failure (Cloudflare is a
known example). When ordering changes, validation must prove both that the new
diagnostic is justified and that the older independent defect still exists when
the new blocker is isolated.

No provider corpus pass substitutes for rule-local semantic evidence, and no
provider-specific condition is permitted in the compatibility registry.

## Compatibility subsystem design

### Rule representation

A rule must be data-addressable and executable. A representative internal shape
is:

```go
type CompatibilityAction string

const (
    Preserve          CompatibilityAction = "preserve"
    Ignore            CompatibilityAction = "ignore"
    Normalize         CompatibilityAction = "normalize"
    PreserveExtension CompatibilityAction = "preserve-extension"
    Reject            CompatibilityAction = "reject"
)

type CompatibilityRule struct {
    ID                   string
    Versions             VersionSet
    Context              ObjectContext
    NormativeDisposition NormativeDisposition
    Action               CompatibilityAction
    Impact               SemanticImpact
    Conformance          ConformanceClass
}
```

A rule implementation also needs:

- source pointer matcher;
- reason/spec citation;
- transform or ignore function;
- finding construction;
- proof/evidence ID.

Do not infer object context from the presence of `$ref` alone.

### Transformation ledger

Each applied `ignore`, `normalize`, or `preserve-extension` action must create a
stable record such as; `reject` uses the same rule identity in the blocking
finding:

```json
{
  "rule": "oas30.reference.additional-property",
  "source": "api.yaml",
  "pointer": "#/components/schemas/Alias/description",
  "declaredVersion": "3.0",
  "action": "ignore",
  "impact": "annotation"
}
```

The ledger is compiler-internal evidence first. A future public reporting surface
can render it without changing semantic behavior.

### Traversal ownership

Do not create another set of ad-hoc recursive walkers.

The repository already centralizes important traversal opacity rules in
`internal/openapiwalk`, used by reference scans, provenance, and schema
reachability. Compatibility should extend that shared traversal model with
semantic visibility/context helpers.

Consumers that must share the same rule result include:

- external-reference counting;
- local reference file filtering;
- remote reference handlers;
- bundler input filesystem;
- provenance mapping;
- OpenAPI version/conformance analysis;
- IR construction;
- target preflight scans.

Package ownership is fixed for implementation:

- `internal/openapiwalk` owns syntax/context classification and traversal helpers
  only; it does not choose compatibility policy;
- a new `internal/compiler/compatibility` package owns rule IDs, normative
  disposition, compatibility action, findings, and pure source/effective-value
  transforms; it may depend on `internal/compiler/openapi` version types and
  `internal/openapiwalk`, but never on the parent compiler package;
- `internal/compiler` owns source snapshots, the canonical source-metadata JSON
  payload, source-cache integration, compatibility-aware reference traversal,
  controlled local/remote loaders, locks, and provenance wiring;
- `internal/compiler/openapi` retains supported-version detection and structural
  OpenAPI reading after the effective view is selected; broad policy decisions
  leave `version.go`;
- IR and TypeScript target code consume the effective semantics and retain only
  genuine target-capability checks. They do not reimplement consumer policy.

### External document handling

For each local or remote referenced source:

1. snapshot exact bytes under the existing trust/integrity policy;
2. decode the complete containing document once into the source cache;
3. apply the **entry document's OAS `major.minor` line** to OpenAPI Object
   semantics throughout that OpenAPI Description; a referenced document does
   not switch OAS lines merely because its root happens to contain another
   `openapi` field;
4. derive the expected OpenAPI Object type from the referencing occurrence.
   If the same source value is reached under incompatible Object-type contexts,
   use the documented OAS implementation-defined policy and fail closed when
   the ambiguity affects generation;
5. for Schema Objects, preserve the separate JSON Schema resource/dialect model:
   fully parse the containing document, honor `$schema`/resource identifiers
   where OAS 3.1+/3.2 permits them, and use the entry OAS dialect only as the
   applicable default when the Schema resource does not declare another one;
6. run compatibility-aware traversal, allowing one controlled
   resolve-to-classify step only when the occurrence itself cannot be
   classified from local context;
7. compute or lazily expose the effective compatibility view and its ledger;
8. scan nested references only through values whose selected action permits it;
9. provide effective bytes to libopenapi's local/remote loader;
10. retain exact bytes for provenance and lock integrity.

Reference lock hashes must continue to describe the fetched source bytes, not a
post-normalization rewrite. Compatibility code changes are already represented
by the existing reusable generator identity; only a future runtime-selectable
policy would need an additional generation-identity component. Neither belongs
in the network integrity hash.

### Incremental generation identity

Any rule change that can alter effective semantics can change generated output
without changing the input OpenAPI hash.

The existing generation fingerprint already includes the reusable generator
identity (`release:<version>`, module version, or clean VCS revision). Because
this workstream deliberately adds no runtime-selectable compatibility mode, a
policy code change necessarily changes that identity and already invalidates
managed reuse. Do **not** add a redundant compatibility-policy field or manifest
migration. Add a separate policy identity only if a future runtime-selectable
policy can vary independently of the generator identity.

## Capability manifest v3

Do not replace the current canonical manifest with a second unrelated registry.
Extend it.

A compatible next schema should retain target state and add orthogonal fields,
for example:

```json
{
  "schemaVersion": 3,
  "features": [{
    "id": "parameter.header.reserved",
    "versions": ["3.0", "3.1", "3.2"],
    "state": "metadata",
    "normativeDisposition": "ignored",
    "compatibilityAction": "ignore",
    "conformance": "conforming",
    "semanticImpact": "wire",
    "proof": "runtime",
    "evidence": "internal/...::Test..."
  }]
}
```

Required proof kinds should distinguish at least:

- `compile`: parser/compiler accepts or rejects at the expected pointer;
- `type`: generated public type contract;
- `runtime`: observable request/response behavior;
- `reference-io`: proves ignored content causes no local/remote resolution;
- `metadata`: exact source metadata preservation;
- `equivalence`: A/B normalization proof;
- `target-error`: precise unsupported target diagnostic.

A `generated` state cannot be justified solely by “SourceArtifacts returned no
error”. A `metadata` state must likewise identify the concrete generated
metadata surface and source-location contract that proves it. After the
`openapi.document` entry-source correction, an annotation that exists only in a
referenced external document is **not** automatically proven to be exported
through `openapi.document`; the manifest evidence must place the feature in the
entry document or name another explicit generated metadata surface.

The manifest test should reject:

- unknown consumer actions/impact/proof values;
- invalid failure-scope/generation-effect pairs;
- an `omitted` state that is not operation- or capability-scoped with the matching omit effect;
- an `error` state without a non-empty scope and `block` effect;
- version scopes inconsistent with the rule;
- missing evidence;
- an `ignore` rule whose evidence does not assert absence from generated
  semantics;
- a `preserve-extension` rule without target/runtime evidence and a
  compatibility-deviation finding; author conformance is validated separately;
- a `normalize` rule without equivalence evidence;
- a reference-affecting ignore rule without reference-I/O evidence;
- a `metadata` claim whose evidence depends on bundling a referenced document
  into `openapi.document` after the entry-source metadata contract correction.

## Diagnostics policy

Generation diagnostics need to distinguish these layers:

- conformance finding;
- compatibility action;
- reference failure;
- target capability failure;
- failure ownership scope;
- generation effect.

A safe ignore, normalization, or explicitly proved compatibility extension can
continue generation without pretending that its behavior is portable OAS
semantics.

The implementation policy is fixed for this workstream and requires no new CLI
mode:

- a source construct that is conforming and whose normative action is simply
  `ignore` is applied silently to effective semantics, while remaining in exact
  metadata/provenance and the internal ledger;
- a nonconforming construct that is safely ignored or normalized emits an
  existing warning diagnostic carrying the rule/action;
- `preserve-extension` emits a compatibility-deviation warning even when the
  source itself is conforming, because generated behavior intentionally exceeds
  the normative disposition;
- `reject` stops the declared owning scope. A document-scoped reject remains a blocking error; once scoped restriction plumbing is enabled, an operation- or capability-scoped reject may be represented as a recoverable omission warning only when the rejected semantics are quarantined from the effective view and the owning scope is not emitted;
- a future strict-conformance flag may promote findings but is outside this
  workstream.

In all cases:

- semantic loss is never hidden: a recoverable warning is permitted only when the complete unsafe owning scope is omitted;
- semantic loss inside an emitted owning scope is never downgraded to warning;
- source pointer and original source identity are preserved;
- target context is only attached to target failures;
- a rule-applied finding includes the stable compatibility rule ID/action.

The machine-readable contract represents that data structurally. Current
diagnostic schema version **4** retains the v3 meaning of optional `rule`,
`action`, `capability`, `scope`, and `effect` fields and adds stable issue
identity plus analyzer coverage / skipped-prerequisite accounting. `scope` uses
`document`, `operation`, or `capability`; `effect` uses `block`,
`omit-operation`, or `omit-capability`.

Compatibility/OpenAPI findings use the existing OpenAPI code family:
`SDKGEN-W140` for non-blocking compatibility/conformance findings and
`SDKGEN-E140` for blocking OpenAPI-level compatibility rejects, with the
stable rule ID providing the specific reason. Fetch-method target rejection
uses the TypeScript capability code `SDKGEN-E511` rather than masquerading as
an OpenAPI error.

Consumers branch on `schemaVersion`. Existing unrelated diagnostics omit
optional compatibility fields. Human rendering shows capability/scope/effect
and rule/action when present. Severity remains the blocking contract in v4: the
CLI exits 0 when only warnings are present and nonzero when an error is present.
Scoped omission behavior therefore emits warnings only after the unsafe
operation/capability has been removed from the target plan, and a zero-finding
report is exhaustive only when the relevant analyzer coverage is complete.

## Implementation ordering

The implementation should be split into independently verifiable units.

1. Introduce `internal/compiler/compatibility` rule types, context-sensitive
   traversal contracts, findings, a no-op effective-view policy, and the
   source-facing canonical metadata payload. Prove semantic identity and prove
   that metadata reflects the decoded entry source rather than the effective
   tree before enabling any compatibility rule.
2. Integrate the no-op policy with the existing source cache and
   compatibility-aware reference traversal, then feed exact-equivalent local
   and remote bytes to libopenapi through controlled loaders. Prove containment,
   allowlist, lock, offline, cache, provenance, and bundling behavior unchanged.
3. Enable lossless ignore rules whose occurrence context is sufficient to prune
   safely: Reference Object extra siblings, reserved inline request headers,
   response Content-Type headers, and Encoding applicability/no-effect cases.
   Add resolve-to-classify coverage for reusable-object occurrences.
4. Implement OAS 3.0 method-specific request-body classification, including
   shallow emptiness, DELETE `preserve-extension`, unsafe-method reject, and
   reference-visibility rules.
5. Remove or simplify duplicate IR/TypeScript Reference Object sibling-merge
   semantics only after the effective-view tests prove those paths unreachable.
6. Split the current version-feature gate into the reviewed rule
   classifications and apply the fixed diagnostic policy.
7. Add only normalization rules whose equivalence matrix passes. Start with
   narrow candidates (`const`, nullable two-type union, simple numeric
   exclusive bounds, OAS 3.0 boolean `true`); keep all unproved cases reject.
8. Make media/binary, `allowEmptyValue`, Encoding runtime behavior, and XML
   behavior explicitly version/context aware.
9. Upgrade the canonical feature manifest/evidence contract and correct claims
   that are supported only by acceptance tests.
10. Rerun local fixtures, pinned/known-boundary corpora, process/memory checks,
    incremental checks, CI, performance, package, docs, and release simulations.

## Validation matrix required before implementation close

The implementation is not complete until every applicable row below has a
recorded result. Earlier failures are appended as history rather than rewritten.

| ID | Scope | Exact method / command | Pass condition |
| --- | --- | --- | --- |
| VAL-COMP-001 | Version model and rule registry | focused Go tests through `devtools run dev:test` | 3.0.x/3.1.x/3.2.x patch variants map to one feature set per minor line, unsupported major/minor lines reject, and every rule ID has version/context, normative disposition, compatibility action, impact, and deterministic ordering |
| VAL-COMP-002 | Reserved request headers | compiler/IR/TypeScript fixtures for case variants of Accept, Content-Type, Authorization plus ordinary control header | reserved headers absent from effective IR/public/runtime contract; ordinary control header preserved; exact source metadata preserved |
| VAL-COMP-003 | Response Content-Type | response/Header fixtures including reusable Header refs and case variants | ignored header absent from typed/raw response header contract and runtime descriptor; source metadata preserved |
| VAL-COMP-004 | Reference Object rules | OAS 3.0/3.1/3.2 fixtures for allowed and extra siblings across Parameter, Header, RequestBody, Response, Link, Callback, Example, SecurityScheme, MediaType where applicable | only version/object-type-defined Reference fields affect effective semantics; arbitrary siblings never override referenced objects |
| VAL-COMP-005 | Path Item refs | non-conflicting and conflicting local Path Item sibling fixtures | non-conflicting siblings retain documented behavior; direct conflicts fail closed with both source locations; behavior is never described as normative |
| VAL-COMP-006 | OAS 3.0 empty bodies | GET/HEAD/DELETE/OPTIONS empty/non-meaningful body fixtures | compatibility action `ignore`; no public body input/runtime body plan |
| VAL-COMP-007 | OAS 3.0 meaningful DELETE/OPTIONS | representative DELETE fixtures plus direct OPTIONS runtime request capture | action `preserve-extension`; request body is typed, encoded, and sent exactly; compatibility-deviation warning recorded while author conformance is classified independently |
| VAL-COMP-008 | unsafe method bodies / Fetch capability | OAS 3.0 and 3.1/3.2 meaningful GET/HEAD bodies; ordinary TRACE; OAS 3.2 `additionalOperations` controls for `QUERY`, `CONNECT`, `TRACE`, `TRACK`, and one allowed custom method | Unsafe GET/HEAD/TRACE or Fetch-forbidden operations are omitted as complete operations with scope/effect diagnostics; QUERY/control custom methods remain eligible; no automatic method/parameter rewrite; managed output remains unchanged when no meaningful entry surface survives |
| VAL-COMP-009 | Encoding applicability | multipart, urlencoded, JSON, response, Content-Type-header, style/explode/allowReserved precedence fixtures | ignored/no-effect fields do not reach target unsupported validation or runtime plans; applicable fields retain wire behavior |
| VAL-COMP-010 | Reference I/O visibility | inline ignored nodes plus local/remote reusable-object occurrences; missing local ref, disallowed remote ref, allowed remote ref with counting handler, lock/cache probes | nested refs reachable only after an `ignore` action cause zero file/network fetches and zero lock/cache entries; an outer reference needed to classify its own occurrence may resolve once under existing trust policy; preserved/extended content retains existing allowlist/lock/offline behavior |
| VAL-COMP-011 | External documents, version context, and source fidelity | local/file-URL/HTTP(S)/stdin/in-memory root inputs; local and remote multi-document fixtures with non-entry documents, complete-document parsing, nested refs, entry OAS 3.0/3.1/3.2 lines, standalone/embedded Schema `$schema` cases, entry-source metadata assertions across normalization/bundling, and prototype-sensitive metadata keys (`__proto__`, `constructor`) | every production compiler entry path populates source-facing metadata; OpenAPI Object semantics inherit the entry OAS line; referenced roots do not silently switch OAS versions; Schema dialect/resource rules remain independent where specified; exact fetched bytes remain integrity/provenance input; generated metadata reflects the decoded entry source, preserves prototype-safe own data properties and the existing useful TypeScript readonly/literal contract, and does not retain a second decoded tree for the plan lifetime; only synthetic manually-built IR may use the documented Raw fallback |
| VAL-COMP-012 | Narrow schema normalization | A/B fixtures for approved `const`, nullable two-type union, numeric exclusive bound, and boolean `true`/`false` candidates | public types, runtime validation, request serialization, response decoding, and wire descriptors are equivalent; source-facing metadata remains the original decoded construct rather than the normalized replacement |
| VAL-COMP-013 | Schema reject boundaries | general type arrays, dialect/reference keywords, and unproved JSON Schema assertions under OAS 3.0 | precise reject; no silent partial lowering |
| VAL-COMP-014 | Version-aware media/schema | 3.0 vs 3.1/3.2 `format: binary`, contentEncoding/contentMediaType contradictions, XML context fixtures | generated wire semantics follow the declared OAS line/context; legacy annotations do not accidentally control 3.1+ wire behavior |
| VAL-COMP-015 | Manifest v3 | feature-matrix/manifest tests through `devtools run dev:test` and `devtools run verify:typescript`; direct `allowEmptyValue` true/false/absent/n-a runtime fixtures; audit of every `metadata` evidence path that currently relies on external-ref bundling | every compatibility rule has executable proof; `generated` cannot be supported by acceptance-only evidence; `metadata` evidence names an actual generated surface and does not assume external documents are folded into entry-source `openapi.document`; `allowEmptyValue` state is retained only if wire behavior is directly proved, otherwise corrected |
| VAL-COMP-016 | Diagnostics | human + JSON-v3 diagnostic fixtures for compatibility findings and target failures; baseline fixtures for pre-existing fields; CLI exit-code assertions | report `schemaVersion` is 3; diagnostics may expose structural `rule`/`action` plus `capability`/`scope`/`effect`; warnings remain exit 0 and errors remain nonzero; unrelated diagnostics omit optional fields; target appears only for target failure; human/JSON ordering and sanitization remain deterministic; docs/release notes call out the versioned JSON schema change |
| VAL-COMP-017 | Incremental identity | managed fresh/check/incremental tests across unchanged build identity and a changed clean VCS/release generator identity carrying a compatibility change | existing generator identity invalidates reuse across policy code changes; unchanged identity retains normal incremental stability; no new manifest field is added unless policy later becomes runtime-selectable |
| VAL-COMP-018 | Publication recovery | existing staging-write/final-rename failure injection plus compatibility reject after existing output | rollback/preservation remains exact; no partial effective output is published |
| VAL-COMP-019 | GitHub pinned corpus | exact SHA above through generation, strict TS, source consumer, declaration consumer, bundle/runtime checks | remains successful; empty ignored fields disappear where expected; meaningful DELETE behavior remains callable |
| VAL-COMP-020 | Stripe pinned corpus | exact SHA above through same artifact matrix | remains successful; empty artifact bodies are removed without breaking meaningful DELETE operations |
| VAL-COMP-021 | GitLab 19.5 exact corpus | exact local SHA above | `0E/4W`, complete coverage, generation/strict TypeScript pass; two HEAD-body and two path-binding omissions are generic rule outcomes; no GitLab-specific code |
| VAL-COMP-022 | Cloudflare | exact local corpus through full generation + strict TypeScript | `0E/37W`, complete coverage; body/security omissions are generic and recursive schema/callable type generation remains typecheck-safe |
| VAL-COMP-023 | Graph v1/beta | existing local corpora | no new compatibility regression; current beta snapshot `0E/0W`, complete coverage, generation/strict TypeScript pass |
| VAL-COMP-024 | DigitalOcean | real source tree through existing compiler reference path | opaque extension/example refs cause no false I/O/E120; `0E/459W` path-binding omissions, complete coverage, generation/strict TypeScript pass |
| VAL-COMP-025 | Twilio | existing core 2010 corpus + strict TypeScript | generation and strict TypeScript pass; numeric-leading identifiers are normalized generically without provider-specific code |
| VAL-COMP-026 | Compile/generate cost | paired pinned GitHub/Stripe process runs using existing representation/perf harnesses | wall/CPU/allocation/peak RSS recorded; no unreviewed material regression or duplicate full-tree phase |
| VAL-COMP-027 | Target artifacts | `devtools run generate:check-test` plus strict/source/declaration/bundle/callable checks | generated API/runtime/public declarations remain coherent |
| VAL-COMP-028 | Repository conformance | `devtools run verify:typescript` | pass |
| VAL-COMP-029 | CI | `devtools run dev:ci` | pass with coverage not weakened |
| VAL-COMP-030 | Performance gate | `devtools run perf:acceptance` | pass; nonzero cost still disclosed |
| VAL-COMP-031 | Full release-safe repository gate | `devtools run dev:check` | pass including packaging, release simulations, examples, cross-build paths |
| VAL-COMP-032 | Documentation and migration note | `devtools run docs:validate` and `devtools run docs:build`, plus review of generated-metadata/reference docs, diagnostics JSON docs, and release notes | pass; public docs describe only implemented behavior; external-reference `openapi.document` semantics and the current diagnostic JSON schema version are explicitly documented as public generated/tooling behavior changes; architecture docs retain investigation/current-state distinction |

For `ignore` rules, rule-local proof additionally requires exact source
metadata/provenance preservation. For `normalize` rules it requires A/B
semantic equivalence. For `preserve-extension` it requires runtime behavior
and a visible compatibility-deviation warning while author conformance remains
independent. For `reject` it requires no emit/publish and managed-output
preservation.

No corpus pass can substitute for the corresponding rule-local row.

## Requirement and acceptance traceability

This table is the investigation-close proof that every settled requirement and
acceptance criterion has an implementation work item and an executable
implementation-close validation path.

| Contract | Work item(s) | Validation path |
| --- | --- | --- |
| REQ-001 / AC-001 — minor-line version model | WI-001, WI-003 | VAL-COMP-001 |
| REQ-002 / AC-002 — normative disposition vs compatibility action | WI-001, WI-002, WI-003, WI-004 | VAL-COMP-002–009 |
| REQ-003 — undefined / implementation-defined taxonomy | WI-001, WI-003 | VAL-COMP-005, VAL-COMP-016 |
| REQ-004 — exact source vs effective semantic view | WI-002, WI-004 | VAL-COMP-002–011 |
| REQ-005 / AC-003 — transformation evidence | WI-004, WI-005 | VAL-COMP-001, VAL-COMP-015, VAL-COMP-016 |
| REQ-006 / AC-004 — compatibility-aware staged reference traversal | WI-002, WI-004 | VAL-COMP-010, VAL-COMP-011 |
| REQ-007 / AC-005 — proven safe normalization only | WI-003 | VAL-COMP-012, VAL-COMP-013 |
| REQ-008 — no document-wide version promotion | WI-003 | VAL-COMP-012–014 |
| REQ-009 / AC-006 — capability separation | WI-002, WI-004 | VAL-COMP-009, VAL-COMP-016, VAL-COMP-027 |
| REQ-010 — reference/dialect/security safety | WI-004 | VAL-COMP-010, VAL-COMP-011, VAL-COMP-013 |
| REQ-011 / AC-007 — canonical feature inventory | WI-005 | VAL-COMP-015, VAL-COMP-028 |
| REQ-012 — required consumer-semantics baseline | WI-001, WI-002, WI-003 | VAL-COMP-002–009 |
| REQ-013 — Schema-version granularity | WI-003 | VAL-COMP-012–014 |
| REQ-014 / AC-008 — Path Item reference policy | WI-003 | VAL-COMP-005 |
| REQ-015 / AC-009 — structured diagnostics | WI-004 | VAL-COMP-016 |
| REQ-016 / AC-010 — incremental/reuse integrity | WI-004 | VAL-COMP-017 |
| REQ-017 — provider neutrality | WI-006, WI-008 | VAL-COMP-019–025 |
| REQ-018 / AC-011 — real-corpus validation | WI-006 | VAL-COMP-019–025 |
| REQ-019 — deterministic bounded cost | WI-007 | VAL-COMP-026, VAL-COMP-030 |
| REQ-020 / AC-012 — publication/release safety | WI-007 | VAL-COMP-018, VAL-COMP-027, VAL-COMP-029–032 |
| AC-002 — ignored vs preserved-extension observability | WI-002, WI-003, WI-004 | VAL-COMP-002–009, VAL-COMP-016 |
| AC-006 — ignored constructs never become target failures | WI-002, WI-004 | VAL-COMP-009, VAL-COMP-016 |
| AC-011 — generic rules across corpora | WI-006 | VAL-COMP-019–025 |

The repeated AC rows make the cross-cutting observable behavior explicit rather
than relying only on their parent REQ rows. No REQ or AC is left without a
work-item owner and a concrete validation path.

## Performance constraints

Do not clone the complete OpenAPI tree once per pipeline phase.

Preferred shape:

- exact decoded source is used once to produce the source-facing canonical JSON
  metadata payload; do not retain a second decoded source tree;
- effective view is produced once per source/policy identity, or lazily through
  a deterministic copy-on-write transform;
- compatibility ledger stores only findings/actions, not a second provenance
  tree;
- bundler reads compatibility-filtered bytes through controlled loaders;
- large-corpus phase allocation and process RSS are measured against the
  current baseline.

A meaningful RSS/allocation regression must be recorded even when existing
thresholds still pass.

## Security constraints

Compatibility must never:

- enable remote references that the existing policy forbids;
- widen an origin allowlist;
- skip lock integrity;
- follow a reference that exists only in ignored content;
- reinterpret a later-version security scheme/flow under an older version;
- change base URI or schema dialect by guessing;
- expose credential-bearing URLs or transport details in diagnostics.

Reference/dialect/security uncertainty fails closed.

## Durable conclusion

The structural problem is not “GitLab uses one odd `$ref` sibling”. The
current compiler lacks one explicit layer that models the difference between:

- exact author source;
- OAS conformance;
- normative consumer semantics;
- compatibility normalization;
- target capability.

That missing separation already causes both false rejection and false
generation semantics.

The implementation direction is therefore:

> preserve source exactly, record the specification's normative disposition,
> compute compatibility visibility as part of staged reference traversal,
> preserve behavior beyond the normative disposition only when target/runtime
> proof shows that dropping it would lose clear API intent, normalize only with
> executable equivalence proof, reject ambiguity or unsafe loss, and keep target
> support as an independent layer.

GitLab is an acceptance corpus for these generic rules, never a source of
provider-specific exceptions.
