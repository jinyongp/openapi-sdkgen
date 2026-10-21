# Local SDK optimization preflight

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

`public-consumer.mjs freeze BASELINE_SDK REFERENCE` freezes the existing public
route/ID/Link keys. `prepare CANDIDATE_SDK REFERENCE FRESH_OUTPUT` creates a consumer
config containing every SDK source plus exact public key equality and negative
route/ID assertions. The reference comes from the baseline, never the candidate.
An absent Link surface stays absent. Existing compiler/runtime consumers still
verify parameter/response and selection semantics.

`profile-go` under this directory uses the real Go target and product publisher.
It records compile, prepare, release of input state, emit and completed publication
as separate checkpoints, with final package counts and file hashes. Its import root
is `example.test/graph/api` and the server addon is enabled. Optional `-profile`
writes intrusive CPU evidence; do not mix those times into ordinary paired samples.

```sh
devtools run preflight:perf --output .tmp/sdk-preflight/recorder-tests -- python3 scripts/perf/sdk-optimization/record_test.py
devtools run preflight:perf --output .tmp/sdk-preflight/go-audit-tests -- go test ./scripts/perf/sdk-optimization/audit-go
devtools run preflight:perf --output .tmp/sdk-preflight/inventory-tests -- node scripts/perf/sdk-optimization/inventory_test.mjs
devtools run preflight:perf --output .tmp/sdk-preflight/semantics -- node scripts/perf/sdk-optimization/semantics.mjs
```
