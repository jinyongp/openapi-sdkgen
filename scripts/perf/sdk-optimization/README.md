# Local SDK optimization preflight

`compare-apis.py --output FRESH_DIRECTORY --pairs 5` compares the pinned original
and final generator revisions on full TVMaze, ListenNotes and GitHub documents.
Run it through `preflight:perf` with a 7,200-second outer deadline. It builds both
generators from Git archives, then alternates generation and cold/warm compilation.
Input hashes are checked before work; all inputs and compiler dependencies must
already be available. See the fixed protocol and documentation boundary in
`plans/sdk-optimization/representative-comparison.md`. Results are local evidence,
not published GitHub Actions benchmarks. No full Graph or Cloudflare run is included.

If the host stops the driver between completed Go cases, `resume-apis.py --output
EXISTING_DIRECTORY` preserves completed samples and executes only missing cases.
Use a fresh outer `preflight:perf` recorder directory and an appropriate deadline.
Recovery checks the original worker, recorder, resource policy, input and generator hashes and keeps
using that unchanged timing worker. It refuses partial cases and missing TS cases;
it does not overwrite a failed trial or retry a warm cache as a cold cache.
Recovery remains bound to the exact worker and recorder saved with that study.
For an older interrupted study, restore its archived versions; do not bypass
their hash checks when current lifecycle handling has changed.

New runs use a temporary run cache unless the caller explicitly supplies a
nonempty `GOCACHE`; caller caches are not removed. Go cold/warm pairs always use
their own fresh temporary cache, reclaimed after both phases or an error. Warm
uses the exact cold cache; the next cold starts empty. Recovery applies the same
policy to missing cases without rerunning completed evidence. The resource helper
hash is pinned alongside the worker and recorder. Historical recovery needs all
versions frozen with that study. Raw records and SDK artifacts remain available;
the caches are disposable. SIGKILL/host shutdown can leave the current directory.

Run through `devtools run preflight:perf`. This is an optional Linux diagnostic,
not an optimization or an ordinary CI performance gate. It uses Python's standard
library and the installed pinned Go/Node/TypeScript tools. It does not install
tools or unlock the devtools vault.

```sh
devtools run preflight:perf --output .tmp/sdk-preflight/build --timeout 180 -- go build -o .tmp/sdk-preflight/sdkgen ./cmd/openapi-sdkgen
devtools run preflight:perf --output .tmp/sdk-preflight/generation --input INPUT --timeout 900 -- .tmp/sdk-preflight/sdkgen generate --target typescript --typecheck --input INPUT --output .tmp/sdk-preflight/sdk
devtools run preflight:perf --output .tmp/sdk-preflight/config -- node scripts/perf/sdk-optimization/prepare-ts.mjs .tmp/sdk-preflight/sdk
devtools run preflight:perf --output .tmp/sdk-preflight/check --timeout 600 -- node test/typescript/node_modules/typescript-5-9/lib/tsc.js -p .tmp/sdk-preflight/sdk/tsconfig.optimization.json --extendedDiagnostics --pretty false
devtools run preflight:perf --output .tmp/sdk-preflight/inventory-run -- node scripts/perf/sdk-optimization/inventory.mjs .tmp/sdk-preflight/sdk .tmp/sdk-preflight/inventory
devtools run preflight:perf --output .tmp/sdk-preflight/go-inventory-run -- go run ./scripts/perf/sdk-optimization/audit-go -root SDK_ROOT -output .tmp/sdk-preflight/go-files.jsonl
```

Every trial directory must be fresh. Blocked, failed, signal, timeout and RSS
protection outcomes return nonzero; no old trial is silently reused. Go is
resolved under `GOTOOLCHAIN=go1.27.2`, version checked, hashed, then the same binary
executes with `GOTOOLCHAIN=local`. The recorder hashes source files including
staged/unstaged/untracked files, command files, inputs, and observed executables.
For TS7 supply `--singleThreaded`; its native process is observed separately from
the Node wrapper. A very short-lived binary can escape 200ms sampling. This is
not a complete process execution trace.

Source hashes exclude ignored files; explicitly pass ignored inputs/configs with
`--input`. Explicit `files` in the prepared TS config covers every generated TS
source, including declarations. `@ts-nocheck` is rejected. After a source check,
the full public consumer and negative contracts are still required separately.

The memory guard sums RSS for the owned session every 200ms. Shared pages can
count twice, short peaks can be missed, and deliberately detached sessions are
unsupported. The default limit is 8 GiB with a ten-minute deadline. Set both
before a matched experiment. These bounds are local smoke policy, not a product
SLA or the final CI budget. Do not run competing memory-heavy trials together.
Generation and checking use separate processes. Preserve interrupted trials.

SIGINT/SIGTERM requests to the recorder produce an `interrupted` result with
`requestedSignal` and exit status `128 + signal`. Owned command-session processes
are stopped before redactors drain and the recorder returns. Unrelated sessions
are not stopped. SIGKILL and host shutdown cannot provide this cleanup guarantee.
Termination follows observed descendants across nested trial sessions, including
workers whose controller has exited. The RSS metric still sums only the original
command session; detached daemons orphaned before observation are unsupported.

Raw stdout/stderr pass through the existing project redactor before storage.
Output directories are private. Use public fixture commands; argv and source
manifests are provenance and must not contain credentials. Do not pass task
context to measured commands. Reports and SDK trees stay in ignored local storage.

`inventory.mjs` reports syntactic import SCCs and projected closure incidence;
these are not type-instantiation SCCs, checker memory predictions or net byte
savings. `audit-go` never packs files. It inventories current final package
ownership, public AST signatures (including tags/receiver types), and conservative
file eligibility. Runtime reflection/error/initialization contracts and actual
bounded packing remain W06/W09 work. Files beyond its 8 MiB parse cap leave the
symbol inventory incomplete and must be inspected separately. Use
`-only-large -max-file-bytes 0` in a separate bounded trial to inventory precisely those files
without the parse cap; combine matching path/hash records with the original audit.

`compare-go.py --baseline ORIGINAL_AUDIT --baseline-supplement LARGE_AUDIT
--candidate CANDIDATE_UNLIMITED_AUDIT --baseline-manifest ORIGINAL_MANIFEST
--candidate-manifest CANDIDATE_MANIFEST --output FRESH_REPORT` compares defined
type ownership and exported AST signatures, including their exact import binding,
while ignoring filenames. Each audit must cover all Go artifacts in its publisher
manifest. A supplemental unlimited audit must match the capped source hash and
size. Missing, stale or duplicate source records fail. A disk-backed multiset
keeps the full Graph comparison bounded in memory. Run its negative ownership,
signature, binding and coverage tests with `python3 -B
scripts/perf/sdk-optimization/compare-go_test.py`. This is a contract inventory;
compiler, reflection and runtime validation remain separate checks.

When a candidate deliberately adds internal helpers, `--allow-additions FILE`
accepts a reviewed JSON document with `version: 1` and an `additions` array. Each
entry specifies `package`, `name`, `signature`, and `importKey` exactly as audited;
defined-type entries use `type:NAME` with empty signature/import key, and exported
declarations use `public:NAME`. Only one new occurrence can be accepted. Existing
baseline names, changed signatures/import bindings, removals, unknown additions,
and missing or duplicate approved entries fail. Keep this explicit document with
the reviewed helper contracts; never derive exemptions from comparison failures.

`public-consumer.mjs freeze BASELINE_SDK REFERENCE` freezes the existing public
route/ID/Link keys. `prepare CANDIDATE_SDK REFERENCE FRESH_OUTPUT` creates a consumer
config containing every SDK source plus exact public key equality and negative
route/ID assertions. The reference comes from the baseline, never the candidate.
New references also freeze each operation ID's exact route literal and check the
public `OperationRoutes` contract. An absent Link surface stays absent. Existing compiler/runtime consumers still
verify parameter/response and selection semantics.
The consumer also reconstructs the original literal `OperationMethod` field maps
from that frozen metadata and checks both directions against the candidate Client.
It uses the same canonical operation identities, so nominal symbols from two
different SDK copies cannot create a false compatibility failure.

`profile-go` under this directory uses the real Go target and product publisher.
It records compile, prepare, release of input state, emit and completed publication
as separate checkpoints, with final package counts and file hashes. Its import root
is `example.test/graph/api` and the server addon is enabled. Optional `-profile`
writes intrusive CPU evidence; do not mix those times into ordinary paired samples.
Checkpoints also report cumulative `totalAllocBytes`, distinct from live heap and
process RSS. Optional `-heap FRESH_FILE` writes a heap/allocation profile after
publication and a GC. Use `alloc_space` for sampled cumulative allocation attribution;
this post-publication snapshot does not measure peak live memory. Heap collection
and CPU profiling are diagnostic overhead, not ordinary CLI timing samples.

`compare-ts.py --output FRESH_OUTPUT --baseline BASELINE_BINARY --candidate
CANDIDATE_BINARY --input PINNED_DOCUMENT [--input SECOND_DOCUMENT] --pairs 5`
alternates fresh generation and whole-source/public-key strict checker processes.
Use it through `preflight:perf` with pinned Node/Go. Each child trial has its own
8 GiB RSS/900-second bound and raw recorder result. Set Node/Go heap policy before
the run. Keep binary build provenance; a binary hash alone does not identify its
source. Run competing variants without other measurement load. Failures remain
in the requested denominator; completed medians/ranges and paired ratios are
separate, with no arbitrary percentage pass gate. Supply `--reference FILE` once
per input when comparing candidates with derived Client maps: freeze that
reference from the original SDK, never from either candidate. The legacy freeze
parser deliberately rejects unfamiliar Client layouts. Filesystem cache is warm or
unspecified, and this command does not claim empty-cache measurements.

`trace-ts-phases.mjs COMPILER CONFIG FRESH_JSONL` uses a pinned JavaScript compiler
API to separate program construction, syntactic diagnostics, binding and semantic
diagnostics without relaxing the configuration. It records each checkpoint before
continuing, so a heap failure still identifies the last completed stage. This is
intrusive classification, not representative checker timing. Native TS7 uses its
own CLI instead of this JavaScript API path.

```sh
devtools run preflight:perf --output .tmp/sdk-preflight/recorder-tests -- python3 scripts/perf/sdk-optimization/record_test.py
devtools run preflight:perf --output .tmp/sdk-preflight/go-audit-tests -- go test ./scripts/perf/sdk-optimization/audit-go
devtools run preflight:perf --output .tmp/sdk-preflight/inventory-tests -- node scripts/perf/sdk-optimization/inventory_test.mjs
devtools run preflight:perf --output .tmp/sdk-preflight/semantics -- node scripts/perf/sdk-optimization/semantics.mjs
```
