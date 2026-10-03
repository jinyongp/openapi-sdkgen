# TypeScript fixture catalog

`catalog.json` is the single inventory for reusable TypeScript generation fixtures.
Keep the existing OpenAPI files and generated directory names stable: current
conformance tests already import those paths. Both `prepare-conformance` and
`representation-check` consume this catalog. Do not add a second list of inputs
inside an agent shell script.

## Fixture responsibilities

| Fixture                  | Primary responsibility                                                                                                                             |
| ------------------------ | -------------------------------------------------------------------------------------------------------------------------------------------------- |
| contract                 | Resource/path/query/header/body calls, pagination, envelopes and binary input.                                                                     |
| collisions               | Exact and prototype-sensitive identities, Link leaves, enums, callbacks and webhooks.                                                              |
| bundle-isolation         | Separation of optional runtime modules and streaming.                                                                                              |
| transport-native-headers | Authentication, header boundaries, codecs and inbound contracts.                                                                                   |
| baseline-oas30           | Small, extension-free, ID-less root operation.                                                                                                     |
| baseline-oas31           | ID-less exact-route Links and callbacks.                                                                                                           |
| baseline-oas32           | QUERY method and NDJSON streaming.                                                                                                                 |
| representation           | Recursive/shared schemas, opaque const data, exact Unicode/control/prototype keys, validation and empty shapes.                                    |
| lifecycle                | Inline nested descriptors, helper construction during binding, stream frames and simultaneous-client isolation.                                    |
| aliases                  | Artifact-local aliases, distinct normalization/Unicode identities, input/output reference replay, resource builders and shared Link/stream owners. |
| diagnostics              | Expected generation failure, diagnostic golden and absence of published output.                                                                    |
| github (external)        | Large operation-count real-world input.                                                                                                            |
| stripe (external)        | Schema-heavy real-world input.                                                                                                                     |

A fixture may serve several checks. Its `characteristics` explain its purpose;
`profiles` select consumers; `scenario` selects a reviewed local probe from
`../verification/scenarios.mjs`. Catalog entries cannot execute arbitrary scripts.
The existing runtime/conformance suites remain authoritative for additional
protocol details; a scenario labelled `surface` checks reflection and module
exports, not a simulated request for every feature listed in its characteristics.

## Adding or reusing a case

Prefer extending the appropriate focused fixture rather than copying it. Add a
new OpenAPI file only when the new behavior has a distinct responsibility, and
register its unique ID, input, output directory, addons and profiles. The catalog
self-test checks that every top-level `*.openapi.json` file is registered exactly
once. Output paths are restricted to children of `fixtures/generated`.

Expected failures retain their golden diagnostic file and must publish no output.
Do not turn a failing fixture into a success case by omitting validation.

External corpora are opt-in and never downloaded by the test runner. Register a
content SHA-256 and source provenance, then supply an explicit local path. A
wrong checksum or operation/schema count fails before generation. The checksum is
the input pin even when an upstream branch or filename says `latest`.

## Commands

Run the normal generated-fixture and type/runtime checks:

```sh
devtools run verify:typescript
```

Compare two already-built generator binaries on all local positive cases:

```sh
devtools run verify:representation-check \
  --baseline /absolute/path/to/baseline-sdkgen \
  --candidate /absolute/path/to/candidate-sdkgen
```

Add the registered real-world inputs with `--input github=/path/spec.json` and
`--input stripe=/path/spec.json`. Paths may be absolute or repository-relative.
Use `--fixtures representation,baseline-oas31` for a focused diagnostic run.

Raw inputs, generated source and transpiled comparison outputs are separate.
Generated output and reports belong under `.tmp`, never in this fixture inventory.
See `../verification/README.md` for the measurement contract and interpretation.
