# Project scripts

Local commands use `devtools run COMMAND:TARGET ARG...`, with the operation first
and the target group second. Arguments follow the command name directly.
Devtools selects the project directory, environment, and local Node runtime.
CI prepares its toolchain and calls the same scripts directly. Versions come
from `go.mod`, `.node-version`,
and each package's `packageManager` and lockfile.

Scripts are grouped by responsibility: `dev`, `generate`, `compatibility`,
`verification`, `perf`, `security`, `examples`, `release`, `install`, and `npm`.
Documentation commands stay in `docs/scripts`. Go measurement helpers and
TypeScript verification implementations stay with the code they exercise.

`preflight:perf` records bounded local SDK optimization trials with pinned Go,
source/binary/input hashes, redacted logs and explicit failure reasons. Its
[internal recipe](perf/sdk-optimization/README.md) covers full-source TypeScript
configuration and read-only TS/Go output inventories. These diagnostics do not
replace public consumer contracts or establish a full Graph compiler pass.

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
| `install` | `build:install VERSION OUTPUT`, `test:install` |
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
| `install` | Standalone shell and PowerShell installers | Fixture checks on Linux/macOS and Windows PowerShell 5.1/7; no live installation in source checks |
| `docs/scripts` | Documentation install, validation, build and servers | Separate build workflow; dev/preview are managed local servers |

`scripts/lib/commands.sh` owns summaries and diagnostic capture. Successful
commands report the essential result on stderr. JSON and exported data use
stdout. Known secret environment values and credential-shaped diagnostics are
redacted before storage or display. Failed stages report the cause and their protected log directory under
`.tmp/runs`. Project scripts need Bash 4.4+ and Python 3 for diagnostic redaction;
macOS CI selects Homebrew's GNU Bash before running these scripts. The public
`install.sh` uses POSIX `sh`. TypeScript
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
`ci:dev go-core` owns source checks, tests outside the two generator packages,
and the Go generation-check integration. `ci:dev go-consumers SHARD COUNT`
discovers runnable Go-target tests with `go test -list`, then distributes their
sorted names round-robin. CI runs four disjoint Go shards and two TypeScript
generator shards; the final gate verifies each group's combined inventory and
completed test records, including newly added tests.
Each shard preserves timing data under `.tmp/ci-tests`. Test-result caching is
disabled for these measured CI groups; Go compilation caches remain available.
The extra runners trade additional setup and total runner work for shorter waits.

`ci:dev typescript-generator [SHARD COUNT]` owns the other TypeScript generator tests,
using the same discovered-inventory partitioning and excluding the two separately
owned runtime/declaration matrix tests.
`ci:dev typescript-runtime` owns runtime conformance, while `ci:dev typescript-matrix`
owns `TestRuntimeFeatureNativeMatrixRegression`. Each compiler version has separate
`consumers` and `declarations` groups; the latter owns its exact
`TestRuntimeFeatureDeclarationMatrixRegression` subtest. No matrix is repeated by
the other groups. `ci:dev go` composes the three Go-test groups locally, and
`ci:dev all` adds runtime, native matrix, tools and all compiler groups.
`test:dev` still runs every Go test across all compilers.

`test:dev` uses a 20-minute timeout per Go test package, including the cumulative
cost of compiling and running generated SDK consumers. Pass `-timeout=...` to
override it for a local run. Generated Go consumer tests retain their 45-second
test timeout and one-minute subprocess deadline. Each CI suite/shard allows
15 minutes including setup; each compiler group allows 10 minutes. The test
coverage gate requires all groups and inventories to succeed.

## Release documentation review

Documentation publication reconstructs `/vX.Y.Z/install.sh` and `install.ps1`
from public releases containing standalone installers. The workflow only lists
their tags with `gh api`; Releaseway requires both installer files and
verifies the remote assets before placing them under their tag directories; the
current release's installers also remain at the site root. Rebuilding from GitHub
releases preserves previous version URLs without relying on Pages history or
artifact retention. Releases predating standalone installers have no such URLs.
The complete site is passed by immutable same-run artifact ID to Releaseway's
`distribute-pages.yml`, which packages, deploys, and verifies the served bytes.
There is no product-specific Pages verifier or installer hash manifest.

To refresh this distribution for an already published stable release without
publishing a new package, dispatch `docs.yml` on `main` with `release-tag` set to
the existing tag. The documentation build uses that tag's source; distribution
uses the dispatched workflow and the pinned Releaseway implementation. Dispatching without a tag
only builds and checks documentation. Version artifacts are scoped to the run,
replaced on retry, and retained only for the build/deploy handoff.

Release numbering starts at `v1.2.0` after the historical `v11.0.4` release.
`.github/releaseway.yml` declares the first version and ancestry boundary.
Releaseway selects versions within that series while retaining historical tags.
After the maintainer-selected v1.3.1 patch release, `bump: auto` resumes
Conventional Commit inference for subsequent versions.
Homebrew keeps `version_scheme: 1` in its product spec so existing installations
recognize the new series as an upgrade. Stable npm releases explicitly publish
to `latest`, prereleases publish to `next`, and GitHub uses `current-series` latest
selection. npm channels live in `.github/npm/packages.yml`.

`devtools run publish:release` requests a version plan for remote `main`
without running product checks.
It uses the GitHub CLI's authentication and does not inject devtools profile values.
Add `--yes` to prepare and publish, or `--prerelease rc` to select a prerelease
channel. `--resume TAG` dispatches at the existing tag so npm provenance remains
bound to that commit. Releaseway owns version calculation and source-bound tag
creation. Publication waits for the latest successful push CI for the exact
source, then runs release cross-builds and example clients. The full product
suites are not repeated. A legacy Releaseway marker reuses CI from its verified,
identical-tree parent. Preparation allows up to 30 minutes for source CI and has
a 45-minute job budget including
release build checks. The workflow preserves the verified plan as an artifact.
A separate five-minute job resumes Releaseway's `source-tag` plan to bind the
selected version directly to the verified source commit with an annotated tag.
It restores the unmodified plan by immutable artifact ID and verifies its digest.
It creates no marker and never pushes
`main`. The built-in `GITHUB_TOKEN` has `contents: write` and `actions: write`
only in that job; build checks retain read-only permissions. No release App,
personal token, new secret, or branch protection exception is required.
Only tag runs publish packages or documentation. Since `GITHUB_TOKEN` tag pushes
do not start push workflows, the main run explicitly dispatches the Release
workflow at the new tag and ends without waiting for it. Both runs share the
release concurrency group; waiting inside the main run would block publication.
A plan-only run performs no remote writes. The saved `release-plan.json` is
Releaseway's schema 2 source-tag plan, including source, version and policy identity;
the workflow does not transform it. Rerunning the tag job reuses that exact binding.
Replanning the same source and channel reuses its existing tag; ambiguous bindings fail and require an explicit
`--resume TAG`. Once main has advanced, use
`--resume TAG` to retry publication at the same commit. Local edits and commits are not
pushed by the dispatch command. Bump policy is configured in
`.github/releaseway.yml`; release notes use Releaseway standard.

Releaseway Actions v0.6.3, npm Actions v0.7.1 and Homebrew Actions v0.4.3 are
pinned by immutable commit SHA. npm receives the exact `release-tag`, including
when multiple stable/prerelease tags share a source. Preparation, tag creation,
GitHub publication and npm retain their operation diagnostics as separate
artifacts on controlled success/failure. These diagnostics do not authorize a
retry or prove that an unavailable runner performed no remote writes.

### Repository release settings

Keep the existing `main protection` ruleset unchanged. Tag preparation needs
neither a branch write nor a bypass. If tag protection is added later, ensure
the release workflow may create version tags without replacing existing ones.

In the `github-pages` environment, retain the `main` branch policy and use a tag
policy named `v*`. The pattern also matches prereleases; the release caller and
Pages deployment job both restrict publication to stable releases. Release
identity and published distribution checks must pass before the site is deployed.

Use `devtools run publish:release --dry-run` to verify
planning. The next `--yes` prepares the tag; follow the tag's Release run for
publication results. A successful main preparation run alone does not establish
that all packages and the site have been published.

`scripts/install/build.sh VERSION OUTPUT` stamps the shell and PowerShell
installers with an exact release version. After GoReleaser builds the archives,
the release workflow prepares both scripts, appends their hashes to
`checksums.txt`, and publishes them with the six native archives. Published
asset retries reuse the original scripts and verify the complete checksum file.
The installers default to that embedded version; explicit version arguments
select another release. Downloads always use that release's GitHub asset URLs.
They verify the archive checksum before replacing an executable, stage beside
the destination, and leave PATH configuration to the user. Progress identifies
the version, platform, destination and current step. Completion prints an
executable verification command and, when needed, a safely quoted command to
prepend the directory to PATH. Existing commands taking precedence are identified.
PowerShell preference changes are local to the installer, including `irm | iex`.

Ordinary CI requires installer fixtures on Linux, macOS and Windows. The release
workflow additionally executes the published installers and native commands on
Linux, macOS and Windows before documentation deployment. Linux also generates
and runs the existing Go consumer. `releaseway/actions/distribute` merges the
exact release's digest-verified scripts with the documentation build; Releaseway's
Pages workflow deploys and verifies the complete site. The source templates are not copied
directly to the public site. Prereleases retain their GitHub script assets and do
not replace the stable site's installers. Releaseway owns Pages concurrency,
archive membership checks, deployment identity, bounded propagation retries, and
served-byte verification. Its workflow summary and delivery report record the
actual delivery state; verification failure does not roll back the live site.

Before publication, compare the public CLI help and generated exports with the
English and Korean guides and references. Confirm that released features are
described as available, examples use working first-run and repeat-run commands,
and migration guidance matches the released behavior. Review time-sensitive
support claims and link to the current release instead of maintaining a fixed
"latest version" in policy pages. This is a semantic review, not a literal-text
test gate.


`devtools run golang:verify [test|race|coverage]` runs the Go target's focused suite.
Generated consumers already run their own race/lifetime checks; the `race` mode also
checks the generator. Coverage is optional and writes `.tmp/golang/coverage.out`.
`devtools run golang:perf micro [GO_BENCHMARK_ARGUMENTS...]` records repeated
prepare/emit measurements for a fixed100-operation workload with allocation counts.
The `runtime`, `generation`, `providers`, `builds`, and `platform` modes exercise
fixed generated consumers and original pinned provider inputs. See
[the Go measurement guide](perf/golang/README.md) for sample counts, cache states,
raw evidence and limits. Run these optional checks sequentially; they impose no
ordinary CI timing, percentage or source-size gate.

`check:generate INPUT golang -- --go-import-path example.com/consumer/internal/api`
retains the input snapshot and invokes a Go verifier. The verifier copies source into
an owned module, pins the imported runtime dependencies, compiles and runs a consumer,
and removes the directory on success or failure. It does not use Node or TypeScript.
The Go CI suite owns its real no-Node/config/failure/cleanup integration test. The
TypeScript tool suite retains the existing TypeScript verifier and module/checking profiles.
`sdk:generate INPUT OUTPUT [TARGET] [-- GENERATOR_OPTIONS...]` accepts Go package/import
options and addons while retaining ownership of input/output/target arguments.

`devtools run go:example` regenerates and runs the Go Todo example in a disposable
module with loopback servers. `devtools run playground:verify` builds and executes
both WASM targets and checks every exported ZIP path/byte. Playground verification
requires installed docs dependencies and does not execute a generated consumer.


`devtools run format:docs` formats code blocks in the root README and Markdown files
under `docs/`, including both display languages. TypeScript/JavaScript, JSON and YAML
use the pinned documentation Prettier version; Go uses `gofmt`, including statement
fragments. Markdown prose, shell commands, TOML and literal output are left unchanged.
`devtools run format-check:docs` checks without writing. Documentation validation and
builds run the same check, so the documentation CI rejects unformatted code examples.

`devtools run verify:docs` follows `build:docs` and checks TypeScript/Vue setup source, rendered SDK scopes,
historical anchors, search indexes and documentation tool contracts. It unpacks all
eight example ZIPs from the built site, generates and compiles their SDKs, and runs
the consumers with local servers or local Fetch responses. Go consumers use empty
module caches with the network disabled. Install the existing verification
tooling first with `devtools run ts-install:dev`; this requirement is for docs development,
not SDK generation. Vue templates are compiled by the static build and exercised through browser flows.

Release checks include the Go example. Published native, npm (three operating systems)
and stable Homebrew distributions also generate and run a Go consumer through
`scripts/release/smoke-go.sh`. Production Pages is called by the stable release only
after those distributions pass. Main/PR docs workflows and manual runs without a
release tag build and verify without production deployment. A manual run with
`release-tag` refreshes the existing release's site and versioned installers.
The reusable docs workflow checks the tag/commit/version and
builds its WASM and pages at that commit, recording `release.json` in the artifact.
On a docs-only failure, rerun the failed release jobs at the same tag; successful
package publication jobs and immutable assets are reused by the existing release flow.

Downloadable examples are assembled from the explicit source inventory in
`docs/scripts/build-example-downloads.mjs`. Documentation validation, development
and builds create the ignored `docs/public/downloads/` archives. Each ZIP includes
its own README, contract, toolchain configuration, source and license; generated
SDKs, installed dependencies and local output are excluded. Keep runnable setup
and verification in example files, and keep public guides focused on key calls.

### Provider authoring preparation and Go verification

`preparation` in a compatibility corpus explicitly opts into a pinned input transform.
`operation-references-v1` expands local Operation authoring references and reusable
non-schema objects, rebases moved schema references/discriminator mappings, and leaves
example/extension data opaque. Both source and prepared SHA256 values are recorded.
The runner checks operation IDs, effective required parameters, request-body presence,
response media/schema presence and direct response types against the compiled IR.
This is a loss guard, not full schema-equivalence verification. It never rewrites invalid
patterns or silently treats Operation Object references as supported generator input.

For an independent Go measurement, run `scripts/golang-compat` with
`--mode golang-generation --verify --server`, the corpus `--manifest`, `--corpus-root`,
`--document`, and `--output`. `--verify` compiles the emitted module with `go test ./...`;
without it, the result proves generation only. Blocked generation still writes a failing
report; tool/input integrity failures return without claiming a completed measurement.
The Actions combiner requires exact clean-source provenance and complete per-document
outcomes. A valid measurement can contain failed SDKs; it is not an all-provider success
claim. Graph is excluded from this profile.

Historical public reports retain their matching `*-manifest.json` when the active corpus
changes. Documentation verifies the archived manifest hash against the report; historical
results do not become evidence for the current source. Fresh public results must come
from the manual compatibility Actions workflow, not local runs.

Go compatibility compilation uses serial package builds, `GOMAXPROCS=2`,
a `3GiB` soft Go heap limit per process and a 20-minute compiler deadline. The
requested policy is recorded in each report. It isolates `GOWORK`, uses the installed
toolchain and disables module downloads. Compiler deadline failures are explicit;
a runner shutdown without a report remains incomplete evidence.

The workflow uploads TypeScript evidence and a Go `--verification-checkpoint` before
the separate Go compiler attempt. A checkpoint records generated SDK hashes and
surfaces but can never pass compiler verification. The final report must match that
checkpoint. If the compiler report is lost, the combiner checks the exact source,
run, job identity and started compiler step through Actions before recording
`unavailable`. It never converts a checkpoint into a compiler pass.

The optional `measurement_run` input of `compatibility.yml` calls
`compatibility-report.yml` to revalidate an existing completed measurement run after
reporting-only corrections. It rejects changes to SDK and measurement producer
sources, checks the original input pins (including legacy Git blob identities),
and records separate measurement and validation run/commit provenance. It does not
relabel earlier SDK measurements as results for a newer SDK source.
