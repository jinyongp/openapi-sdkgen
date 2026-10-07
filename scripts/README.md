# Project scripts

Local commands use `devtools run COMMAND:TARGET ARG...`, with the operation first
and the target group second. Arguments follow the command name directly.
Devtools selects the project directory, environment, and local Node runtime.
CI prepares its toolchain and calls the same scripts directly. Versions come
from `go.mod`, `.node-version`,
and each package's `packageManager` and lockfile.

Scripts are grouped by responsibility: `dev`, `generate`, `compatibility`,
`verification`, `perf`, `security`, `examples`, `release`, and `npm`.
Documentation commands stay in `docs/scripts`. Go measurement helpers and
TypeScript verification implementations stay with the code they exercise.

| Target | Examples |
| --- | --- |
| `dev` | `build:dev`, `test:dev`, `ci:dev go`, `ts-lint:dev` |
| `generate` | `sdk:generate INPUT OUTPUT`, `check:generate INPUT` |
| `verify` | `typescript:verify [test\|typecheck\|coverage]`, `compilers:verify 5.7.3`, `representation-check:verify` |
| `compatibility` | `fetch:compatibility`, `benchmark:compatibility`, `merge:compatibility` |
| `perf` | `benchmark:perf`, `identifiers:perf`, `profile:perf`, `acceptance:perf`, `runtime-features:perf` |
| `security` | `audit:security`, `audit-test:security` |
| `examples` | `todo:examples`, `advanced:examples`, `capabilities:examples` |
| `release` | `check:release`, `cross-build:release`, `test:release`, `publish:release` |
| `npm` | `source-check:npm` |
| `docs` | `validate:docs`, `build:docs`, `dev:docs` |

Use `devtools command list` for the full command list. TypeScript runtime checks,
typechecking and optional coverage share `typescript:verify` with a mode argument.
`check:generate INPUT [TARGET] -- GENERATOR_OPTIONS...` uses its own separator
to distinguish generator options from the check script's arguments.

| Group | Responsibility | Execution cost / CI placement |
| --- | --- | --- |
| `dev` | Go/TypeScript leaf commands and suite composition | Ordinary CI suites; `all` is a sequential local aggregate |
| `generate` | Input snapshot, SDK generation and compilation checks | Verification-tools suite; disposable SDK outputs |
| `verification` | Runtime conformance and compiler consumers | Runtime suite and independent compiler-version jobs |
| `verification` delivery/representation | SDK delivery measurements, comparisons and browser sessions | Explicit commands; browser servers run until stopped |
| `compatibility` | Pinned corpus fetch/verify, document benchmark and merge | Separate manual workflow; document shards run in parallel |
| `perf` | Workload measurements, profiles and existing acceptance thresholds | Explicit commands; repeated measurements stay outside ordinary CI |
| `security` | Pinned vulnerability scanners and SARIF interpretation | Explicit audit; fixture checks require no database calls |
| `examples` | Runnable example server/client smoke checks | Integrated `check`; temporary servers are stopped on exit |
| `release` / `npm` | Cross-builds, release safety, source packaging and publication | Tools suite uses disposable simulations; publication is explicit |
| `docs/scripts` | Documentation install, validation, build and servers | Separate build workflow; dev/preview are managed local servers |

`scripts/lib/commands.sh` owns summaries and diagnostic capture. Successful
commands report the essential result on stderr. JSON and exported data use
stdout. Known secret environment values and credential-shaped diagnostics are
redacted before storage or display. Failed stages report the cause and their protected log directory under
`.tmp/runs`. Scripts need Bash and Python 3 for diagnostic redaction; TypeScript
commands also need the project's Node version and Corepack.
Reports name their output paths. Local logs remain under `.tmp/runs` until the
owner removes them; CI uploads failure diagnostics with seven-day retention.

`devtools run schema-programs:perf [OUTPUT] [BASELINE_SOURCE]` measures 1,000 APIs
and a 10-route selection using shared schemas and a private Link target. It checks
strict generated source, fresh declarations, actual browser bundle calls and
named-client program sharing. The optional captured source tree builds a before
generator under the same toolchain. Three generation trials and one compiler
trial report time and peak RSS; they do not impose size or timing thresholds.

`devtools run provider-size:perf OUTPUT BASELINE_BINARY CURRENT_BINARY MANIFEST`
compares pinned local OpenAPI documents against a released generator. It measures
full generation and one/two API consumers using static `loadOperations` references,
root selection, and named-client selection. Every deployed minified JS chunk and
its gzip size count toward the comparison; native ESM checks the exact API set
without sending requests. This optional command fails when either JS or gzip
exceeds the baseline by more than 5%; ordinary CI has no byte-size gate.

The JSON manifest has a `providers` array with `name`, `input`, and `sha256`.
Optional `excludedRoutes` entries contain `route` and `reason` for known input
omissions; both generators must expose the same remaining routes. Input fixes
and exclusions must be disclosed separately. An optional `baselineFull` path
reuses an independently verified baseline SDK; omit it for a fresh generation.
Reports include input and binary hashes, the bundler version, all chunk totals,
TypeScript file counts, and total generated-tree bytes including metadata.
Outputs, generated SDKs and logs use owner-only permissions. Set `private: true`
at the manifest root to keep detailed failures and provider names out of command
output. Confidential input, SDKs, logs and reports belong in ignored local storage;
publish only the public-provider results. Use a fresh output directory when
changing inputs or binaries.

Use `--help` to check arguments, side effects, and output paths. `--verbose`
requests full redacted output for one invocation and warns on stderr before
work starts. Prefer the default command for routine work.

CI suite boundaries follow execution cost and dependencies rather than directory
count. Each suite runs its necessary checks once. Mutable outputs belong to one
execution; only prepared artifacts with matching inputs can be shared for reads.
The Go suite runs source checks and Go tests. The TypeScript runtime suite owns
the complete emitted runtime matrix (`TestRuntimeFeatureNativeMatrixRegression`),
so that matrix is excluded from the Go suite and executed once alongside runtime
conformance checks. `test:dev` still runs every Go test.

## Release documentation review

Before publication, compare the public CLI help and generated exports with the
English and Korean guides and references. Confirm that released features are
described as available, examples use working first-run and repeat-run commands,
and migration guidance matches the released behavior. Review time-sensitive
support claims and link to the current release instead of maintaining a fixed
"latest version" in policy pages. This is a semantic review, not a literal-text
test gate.
