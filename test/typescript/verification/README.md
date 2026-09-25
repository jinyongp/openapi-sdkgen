# Representation verification

This harness is reusable C1/preimplementation infrastructure. It does not modify
the generator or apply an optimization to its output. Each generator produces its
own sources, then Oxc erases TypeScript into separate ES2022 ESM trees without
bundling or minification. Source manifests are checked before comparison.

## Layout

- `../fixtures/catalog.json`: common fixture definitions, profiles and external input pins.
- `catalog.mjs`: catalog/path/input validation and managed-output hash auditing.
- `prepare-fixtures.mjs`: catalog-driven replacement for duplicated conformance setup.
- `contracts.mjs`: descriptor/alias-aware graph fingerprints and paired statistics.
- `scenarios.mjs`: reviewed mock transport and per-fixture actions.
- `worker.mjs`: fresh-process timing, unbundled contracts, leaf dependencies and lifetime probes.
- `preimplementation.mjs`: bounded sequential A/A and A/B orchestration and evidence output.
- `verification.test.mjs`: positive and deliberately corrupted controls for the harness itself.
- `helper-contracts.mjs`: shared exact-key, identity, recursion and opaque-value constructor vectors.
- `artifacts.mjs`: strict checking, unmodified source consumers, declaration consumers and full-client bundles.
- `migration.mjs`: distinct-generator migration, no-op, fresh equivalence and publication conflict probes.

The normal conformance wrapper runs the harness self-tests. Full comparative
benchmarks are opt-in through `just agent representation-check`; they do not run
on every unit test invocation and do not download corpora or install packages.

## Measurement contract

Default runs use five independent baseline/baseline A/A pairs and ten interleaved
baseline/candidate A/B pairs per selected case. Raw samples are retained. Inputs,
binaries, catalog, lockfile and harness sources are hashed in the report.

Timing workers measure three contiguous boundaries: import start, import end,
and completion of the **first** createClient. They record the first construction
and the end-to-end import-through-first-client interval directly. No forced GC or
reflection scan occurs inside that interval. Repeated-client timing is separate;
it must never be relabelled as the first call. Fresh processes do not imply cold
filesystem or kernel caches.

Memory and instrumentation use different workers. Fixtures with the catalog `lifetime` profile (representation, lifecycle, GitHub and Stripe) receive lifetime checks. For each,
three paired runs each execute six batches of forty discarded clients. GC occurs
between batches, and all post-GC heap values are retained. A loader hook counts
only the known generated wireProperties function; it is not used in latency runs.
Ten rounds of fixture actions must add zero helper invocations. Two live clients
use different origins and synthetic credentials to check configuration isolation.
A finite plateau observation is not a proof of the absence of every possible leak.

The contract worker fingerprints own keys/descriptors, prototypes, function names
and lengths, and object/alias sharing without invoking getters. It compares all
existing generated module **runtime value export names**. This is not a complete
TypeScript type-export/declaration compatibility check: normal conformance and
the final declaration-consumer gates still apply. Export comparisons allow only the exact additive internal `wire-properties.wireProperties` module and `callables.createWireProperties` facade, when absent from baseline. They never hide removed existing exports or unrelated additions. The facade must expose the same function object as the dedicated module.

The representation leaf probe uses actual unbundled module loading and requires
that importing Node's wire schemas not load the full codecs implementation. This
checks the dedicated helper boundary that an optimizing bundler can conceal.

## Interpretation

The JSON report has separate correctness and performance-review fields. Exceptions,
wrong hashes, contract differences or missing imports make the command exit
nonzero. A measured cost beyond the predeclared review policy produces
`status: review` and preserves the samples; it is **not** a performance pass even
though older archived runs exited successfully. Current commands exit **2** for review,
**1** for a correctness/tool failure, and **0** for pass. Never turn review into a pass
by checking only that a process completed.

Review policy is recorded before sampling: a positive paired regression greater
than 5% and both the absolute floor and twice the median absolute A/A variation
is flagged. Absolute floors are 2 ms for import/end-to-end and 0.25 ms for client
construction. These are diagnostic review boundaries, not automatically accepted
release budgets. Post-GC late heap growth above baseline by more than 1 MiB is
flagged. Do not change policy after seeing results to obtain a pass.

A `surface` scenario intentionally performs no HTTP request; the report records
zero requests. Existing conformance covers its transport details. The collision
fixture also records the baseline's ordinary-empty-map Webhook behavior separately:
prototype-sensitive absent keys can cause a startsWith error in the existing
router. This observation is compared, not silently fixed or counted as successful
routing. Null-prototype empty host maps are used for the valid unmatched-route test.

## Running and preserving results

```sh
just agent representation-check \
  --baseline /absolute/path/to/baseline-sdkgen \
  --candidate /absolute/path/to/candidate-sdkgen \
  --input github=/absolute/path/to/pinned-github.json \
  --input stripe=/absolute/path/to/pinned-stripe.json \
  --candidate-label 'identify the exact prototype or production candidate'
```

`--fixtures`, `--pairs`, `--aa-pairs` and `--memory-pairs` support bounded diagnostic
runs. A one-pair pilot is for correctness/debugging, not a performance conclusion.
Paths are repository-relative or absolute. External input IDs must be registered.

Each invocation owns a fresh `.tmp/preimplementation/run-*` directory. It writes
`report.json` from the start, updates it after each case and preserves errors and
raw samples. Generator logs, source and ESM outputs remain beside it. A free-space
and inode check runs before generation. Outputs are intentionally retained, not
removed by a broad cleanup command; archive useful reports and manage old runs
through the workspace's normal explicit cleanup procedure.

This runner never sets final production approval. It covers a scoped subset of
the workstream matrix and must be combined with strict/declaration/IDE checks,
full CI, bundle/performance acceptance and publication/migration checks for the
final integrated implementation.

## Fingerprint and lifecycle guardrails

Fingerprint version 2 hashes the root primitive value as well as its reachable descriptor graph. Version 1 missed primitive-only response changes; archived reports remain historical and the final candidate was rerun using version 2. Negative controls cover string/number/null/undefined differences. No getters are invoked.

The lifecycle fixture must make a positive number of constructor calls when a candidate helper exists, and zero additional calls during requests and stream frames. A zero count because the helper was never loaded is not a passing lifecycle probe. Other reference-only fixtures may correctly leave a helper module unloaded.

## Artifact and migration commands

```sh
just agent representation-artifacts --report .tmp/preimplementation/run-EXAMPLE/report.json --pairs 3
just agent representation-migration --baseline /path/to/base --candidate /path/to/candidate
```

The artifact command verifies the source and ESM hashes from its input report before using them. Strict source checking removes nocheck equally from both check copies; normal source and declaration consumers use the generated source as shipped. A baseline strict failure is retained with raw diagnostics. Matching candidate diagnostics are classified `baseline-blocked`, not PASS, and the command exits 2. A new or different candidate diagnostic fails with exit 1. Existing diagnostic comparison normalizes only check-directory paths, source positions and private lexical spellings; raw messages remain available for review. No compiler strictness flag is relaxed.

Migration compares existing managed files byte-for-byte, protects their mtimes on no-op, and verifies refusal for edited files and unmanaged file/directory/symlink collisions at the new helper path. The command does not synthesize successful publication by deleting user content.

## Independent process cost measurement

```sh
just agent representation-measure --report .tmp/preimplementation/run-EXAMPLE/report.json --artifacts .tmp/preimplementation/run-EXAMPLE/artifacts-EXAMPLE/report.json --pairs 10
```

This consumes the same pinned input/binary and compiler configurations as the correctness checks. It interleaves fresh generation, strict and consumer checks with per-child GNU time CPU/peak-RSS measurements. A generated directory belongs to the measuring invocation and is removed only after that individual run; raw measurements and command logs remain. A passing status means measurements completed, not that every performance delta meets the separate acceptance policy. This report does not include network/download/binary-build time or claim a cold filesystem.
