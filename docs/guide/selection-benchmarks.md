# Compare selection costs

Choose APIs during generation to reduce generated source and checking work.
Choose an entry point to control what the application imports and deploys.
These are separate decisions: a small application bundle can come from a large
generated SDK, and a small initial import can still require many deployment files.

## Released-provider comparison {#released-providers}

The October 5, 2026 recovery snapshot is compared with the released **v10.1.1**
generator on pinned official documents. Every provider has seven comparisons:
full generation, static `loadOperations` for one and two APIs, root `[selection]`
for one and two APIs, and named client selection for one and two APIs.
**All 35 comparisons reduce both deployed JS and gzip.** The largest remaining
JS ratio is 87.5% of v10; the largest gzip ratio is 98.2%.

Full-client results below sum every deployed chunk. Values are bytes, before → after.

| Input | Deployed JS | Deployed gzip | Generated tree bytes | TypeScript files |
| --- | ---: | ---: | ---: | ---: |
| Stripe GA | 3,182,786 → 1,879,811 | 299,802 → 233,518 | 40,578,789 → 45,777,436 | 5,558 → 8,715 |
| Stripe v1 | 2,862,026 → 1,714,840 | 274,878 → 214,997 | 38,431,729 → 43,130,001 | 5,205 → 8,021 |
| GitHub | 2,516,853 → 2,202,717 | 217,123 → 197,640 | 32,835,540 → 38,430,561 | 8,005 → 9,749 |
| Twilio | 702,794 → 572,622 | 55,383 → 45,766 | 6,187,188 → 7,114,737 | 1,309 → 1,698 |
| Cloudflare corrected control | 10,981,908 → 8,284,847 | 719,760 → 706,521 | 104,350,156 → 126,388,298 | 26,587 → 36,412 |

Generated trees still grow by roughly 12–22% in these full cases. They include
type-only files and generator metadata, so source cost and bundle cost remain
separate measurements. Shared algorithms and descriptor data remove duplicated
executable validators; the additional module boundaries do not imply a smaller
generated tree or faster type checking.

Cloudflare's original pinned document has four unresolved discriminator mappings.
It must produce `SDKGEN-E120`, rather than an internal failure. Its size comparison
uses a separate control that corrects only those four mapping names. Both versions
expose the same 3,642 routes: one legacy hidden route and four routes with undeclared
security schemes are excluded and recorded explicitly. The other inputs are unchanged.

<a :href="withBase('/benchmarks/provider-release-results.json')">Raw public results</a>
include every surface, source counts, official input URLs/revisions/hashes, the
Cloudflare corrections and omissions, and both generator binary hashes. The current
binary hash starts with `fa86e7996693`; this is a development snapshot, not v11.0.0.
Conditions: Node.js 24.21.0, Rolldown 1.2.6, browser ESM, tree shaking and minification;
each deployed chunk is gzip-compressed at level 6 and then summed. The browser bundles
are imported in Node.js and their exact API route sets checked without network calls.
These checks measure size and construction, not provider response behavior or timing.

Reproduce with local pinned inputs, the released v10.1.1 binary and a development
binary. A manifest lists `providers` with `name`, absolute `input` and `sha256`;
`excludedRoutes`, when needed, lists each `route` and `reason`. Apply the recorded
Cloudflare corrections to a separate copy before measuring.

```sh
devtools run provider-size:perf .tmp/provider-comparison /absolute/v10-generator /absolute/current-generator /absolute/providers.json
```

This optional command checks JS and gzip against a 5% increase limit, validates
input hashes, and preserves diagnostics. It requires a fresh output directory
after changing a binary or input. An optional `baselineFull` may reuse an independently
verified v10 full SDK; all selected SDKs are generated separately. Set manifest
`"private": true` for local confidential comparisons:
SDKs, logs and reports stay in an owner-only directory and detailed failure output
is suppressed. Only public-input results are included in the report above.

## Earlier small API runtime cost {#small-api-runtime}

The October 4 development snapshot automatically selects runtime handlers from
the generated APIs. This sample has three JSON APIs with Bearer authentication,
object/array validation, string-length and object-property constraints. All four bundles perform the
same `getItem` request and check the URL, credentials, and returned DTO with
mocked Fetch. The default entry needs no selection configuration.

| Generation | All TypeScript files | All source lines | Runtime files | Runtime lines |
| --- | ---: | ---: | ---: | ---: |
| Default generation | 120 | 8,027 | 77 | 5,940 |
| Full-capability test control | 224 | 17,562 | 186 | 15,752 |

| Application entry | JS chunks | Entry JS bytes | Entry gzip bytes | All deployed JS bytes | All deployed gzip bytes |
| --- | ---: | ---: | ---: | ---: | ---: |
| Full-capability test control | 1 | 107,216 | 30,925 | 107,216 | 30,925 |
| Default root | 1 | 26,776 | 8,216 | 26,776 | 8,216 |
| Static selective | 3 | 19,330 | 6,609 | 34,358 | 12,314 |
| Lazy selective | 7 | 369 | 274 | 35,598 | 13,517 |

The default root is **26.15 KiB of JS and 8.02 KiB gzip** in this sample,
about 27% of the control's compressed size. Static and lazy preparation reduce
the entry size but deploy more compressed bytes than the root for this small
API. Entry size alone does not describe the cost of a completed call. Shared
request handling, validation, errors, and client APIs still have a fixed cost;
automatic selection does not make them disappear.

The full-capability control uses the same current runtime implementation with
all handlers connected by test preparation. It measures feature selection,
rather than comparing against an older release with different behavior. The
historical esbuild result of about 97 KB / 28.8 KB gzip uses different input and
tool settings and is not the baseline for this table.

Reproduce from a repository checkout with the TypeScript verification
dependencies installed:

```sh
devtools run runtime-features:perf
```

This optional command generates and executes 73 client/server fixtures, checks
226 scenarios against the same-source control, checks 3,290 module exclusions
with matching control inclusions, and measures the four browser bundles.
Separate regression tests check strict generated source and fresh declaration-only
consumers with TypeScript 5.7.3, 5.9.3, 6.0.3, and 7.0.2. Ordinary CI checks
behavior and dependencies without fixed byte or line-count thresholds.

Measured on October 4, 2026 UTC, in the uncommitted development tree based on
`6c349f9`. <a :href="withBase('/benchmarks/runtime-features-results.json')">Raw results</a> include the
generator/input/source hashes and every output chunk's size and hash.
The generator source hash starts with `85bacbe1902d`.
Conditions: Node.js 24.21.0, TypeScript 7.0.2, Vite 8.3.0 / Rolldown 1.2.6,
browser ESM, ES2022, Oxc minification, code splitting, and `modulePreload: false`.
Gzip level 6 is applied separately to each deployed chunk and then summed.
The raw report also records source, native module bytes, and minified bundle
bytes for all 73 fixtures. These additional bundles export the entire client
factory or webhook router, rather than the single call above. Examples of their
total deployed gzip bytes:

| Contract | Selected runtime | Full-capability control |
| --- | ---: | ---: |
| XML client | 17,146 | 30,386 |
| Multipart client | 19,204 | 30,403 |
| Mixed-media client | 21,325 | 32,270 |
| JSON webhook router | 12,199 | 23,627 |
| XML webhook router | 17,312 | 23,654 |
| Multipart webhook router | 17,278 | 23,629 |

Source counts include type-only files; source maps and HTTP overhead are excluded.
Mocked execution runs in Node.js; browser network and timing are not measured.
These are reproducible sample sizes, not a size guarantee for every API.

## One, two, or three APIs

This compares operation input binding source `b0aac7b4e740` with current resource binding source `85bacbe1902d` under the same inputs and tool settings. Each CLI operation selection bundles the default root and calls every selected API.

| Selected APIs | Before JS | Current JS | Before gzip | Current gzip | Current TS files / lines | Current runtime files / lines |
| --- | ---: | ---: | ---: | ---: | ---: | ---: |
| Get one | 25,039 | 24,364 | 8,127 | 7,883 | 107 / 7,370 | 75 / 5,964 |
| Get one with query | 28,360 | 27,678 | 9,041 | 8,789 | 109 / 7,676 | 76 / 6,222 |
| Get and list | 26,568 | 25,893 | 8,354 | 8,114 | 114 / 7,783 | 76 / 6,000 |
| Get and create | 26,036 | 25,361 | 8,263 | 8,027 | 113 / 7,739 | 76 / 5,981 |
| Get, list, and create | 27,560 | 26,885 | 8,489 | 8,245 | 120 / 8,147 | 77 / 6,017 |

Prepared resource input selects its required, empty or optional binder. Stream,
Link and pagination binding follows prepared capabilities. Generic helpers reuse
the same owners. All five final JS/gzip bundles shrink; one API saves 675 JS bytes
(2.70%) and 244 gzip bytes (3.00%). HTTP policies, validation, credentials,
undeclared response-media handling, cancellation, errors and `.raw()` are preserved.

Final deployed JS/gzip measures this improvement. Source counts are diagnostics:
one-API source grows from 286,423 to 295,645 bytes while the bundle shrinks.
Named and selective providers carry exact resource binders, so generated client
assembly also removes generic dispatch from the final bundle. Generic providers
retain their existing helper path and ABI 1.

The three-API row calls all three APIs. The root table above calls only `getItem`.
<a :href="withBase('/benchmarks/schema-operation-results.json')">Current raw results</a>
include strict compilation and actual mocked bundle calls. The
<a :href="withBase('/benchmarks/schema-operation-before-resources-results.json')">previous operation binder results</a>
are preserved. Node.js 24.21.0, TypeScript 7.0.2, Vite 8.3.0/Oxc, browser ESM
ES2022 and gzip level 6 match the preceding phase.

## Previous 1,000-API HTTP contract comparison

The HTTP policy phase (`4c47c6012ef5` → `fc3b2016bf6d`) shares six
schema programs across the complete 1,000-API fixture. Selecting ten public APIs
plus their private Link target produces four programs. The named client shares
exactly the same program files and bytes as root selection.

Numbers below show before → after that phase. Each browser bundle performs `getItem0`;
the root entries still retain the generated API surface of their scope.

| Case | TS files | TS bytes | Shared programs | Declaration bytes | Deployed gzip bytes |
| --- | ---: | ---: | ---: | ---: | ---: |
| All 1,000 APIs | 6,090 → 6,109 | 15,284,115 → 16,144,263 | 6 → 6 | 9,547,766 → 9,557,464 | 61,489 → 61,397 |
| 10 selected APIs | 134 → 145 | 421,476 → 431,883 | 4 → 4 | 223,535 → 228,215 | 9,492 → 8,632 |
| Named client with those 10 APIs | 138 → 149 | 435,141 → 445,548 | 4 → 4 | 234,497 → 239,177 | 12,107 → 11,459 |

The selected root saves 860 gzip bytes (9.06%); named saves 648 (5.35%). The
complete SDK saves only 92 gzip bytes (0.15%), and its raw JS grows by 774 bytes.
Its TypeScript source grows by 860,148 bytes (5.63%) because policy imports and
composition declarations remain in individual execution providers. Source and
declaration cost must be considered alongside delivery size. Shared schema
programs still avoid duplicating validators per operation; these results do not
establish a universal minimum. General XML, multipart and mixed client bundles
above also retain the small cost of composing their general policies.

| Current scope | Generation median ms / peak RSS KiB | Strict source ms / peak RSS KiB | Declaration emission ms / peak RSS KiB |
| --- | ---: | ---: | ---: |
| All 1,000 APIs | 3,101.4 / 59,416 | 3,522.1 / 1,191,800 | 2,427.2 / 1,199,468 |
| 10 selected APIs | 86.4 / 31,668 | 286.9 / 139,812 | 286.2 / 149,976 |
| Named client with those 10 APIs | 88.7 / 28,596 | 224.8 / 146,724 | 248.9 / 143,972 |

Generation uses three fresh processes and empty output directories. Each strict
source and declaration check uses one fresh compiler process; generated
`@ts-nocheck` is removed. Final trials run serially with a warm filesystem cache after other heavy
validation completed. These timings are observations rather than stable speedup claims. The report
includes the corresponding baseline timings, all trials, input/binary/program
hashes, source/declaration counts, and executed bundle results. Compiler version
and bundle settings match the small measurements above. This does not resolve
the earlier 10,000-API inference limit described below.

<a :href="withBase('/benchmarks/schema-programs-before-compositions-results.json')">HTTP-phase raw results</a>.
The earlier schema-program comparison is preserved in
<a :href="withBase('/benchmarks/schema-operation-before-http-results.json')">small-API results</a>
and <a :href="withBase('/benchmarks/schema-programs-before-http-results.json')">1,000-API results</a>.

```sh
devtools run schema-programs:perf
# To compare a captured source tree:
devtools run schema-programs:perf .tmp/schema-programs-measurement /path/to/captured-source
```

## Repeated execution assembly comparison

The next phase (`fc3b2016bf6d` → `20d7077bf595`) shares factory code for
providers with identical prepared features and stream capability. Each provider
still creates its own services. A single provider emits its assembly directly;
default and named client factories retain their existing execution path.
The five small default-root bundles above remain byte-for-byte the same.

The same 1,000-API comparison now produces these before → current results:

| Case | TS files | TS bytes | Declaration bytes | JS bytes | Deployed gzip bytes |
| --- | ---: | ---: | ---: | ---: | ---: |
| All 1,000 APIs | 6,109 → 6,110 | 16,144,263 → 14,783,053 | 9,557,464 → 9,557,968 | 520,173 → 520,173 | 61,397 → 61,397 |
| 10 selected APIs | 145 → 146 | 431,883 → 420,507 | 228,215 → 228,719 | 30,239 → 30,239 | 8,632 → 8,632 |
| Named client with those 10 APIs | 149 → 150 | 445,548 → 434,172 | 239,177 → 239,681 | 37,290 → 36,180 | 11,459 → 11,296 |

Full generation saves **1,361,210 source bytes (8.43%)**. Each scope adds one
2,284-byte assembly module and 504 declaration bytes. Six full-scope schema
programs, four selected/named programs and all canonical runtime source remain
identical. The full and selected-root delivery sizes are unchanged; named saves
163 gzip bytes (1.42%). This reduces generated source duplication; it does not
reduce the fixed runtime cost of a one-operation root bundle.

The generated `internal/execution-compositions/` directory owns assembly
functions. It imports only the required canonical runtime modules and initializes
no module state. Client context, credentials, custom codecs and projected schema
closures remain outside it. Regression tests cover independent instances,
asynchronous credentials, mutable input revalidation and concurrent cancellation.
Selecting one operation removes an unused shared factory from managed output;
selecting matching operations again restores the same semantic path.

<a :href="withBase('/benchmarks/schema-programs-before-callables-results.json')">Raw comparison for this phase</a>
records the six executed bundles, strict source/declarations and generation,
checking and declaration time/RSS under the conditions above. Timing is an
observation, not a speed guarantee. The previous HTTP phase is preserved in the
linked report above and
<a :href="withBase('/benchmarks/schema-operation-before-compositions-results.json')">its small-API report</a>.
These results still do not establish a universal minimum SDK size.

## Selecting binders by input contract

The preceding `b0aac7b4e740` source imports each operation's prepared input binder
directly. Separate canonical owners provide callable types, resource and stream
binding, and namespace decoration; generic helpers compose the same implementations.
Providers reuse the selected `bindBase` or `bindStream` first parameter type.
Public aliases, function names/arity, hover and HTTP error provenance remain intact.

The same 1,000-API fixture compares against `20d7077bf595`:

| Generation scope | TS files | TS bytes | Declaration bytes | JS bytes | All deployed gzip bytes |
| --- | ---: | ---: | ---: | ---: | ---: |
| All 1,000 APIs | 6,110 → 6,117 | 14,783,053 → 14,701,368 | 9,557,968 → 9,569,912 | 520,173 → 514,082 | 61,397 → 61,359 |
| 10 selected APIs | 146 → 152 | 420,507 → 420,374 | 228,719 → 229,881 | 30,239 → 29,878 | 8,632 → 8,522 |
| Named client with those 10 APIs | 150 → 156 | 434,172 → 434,039 | 239,681 → 240,843 | 36,180 → 36,021 | 11,296 → 11,264 |

Full source saves another **81,685 bytes (0.55%)**. Declarations add 11,944 bytes
for full and 1,162 bytes each for selected/named scopes, reflecting the extra owner
files and type import paths. Four compiler strict-source, fresh-declaration and
hover checks verify the public contracts. Full, selected and named gzip save 38,
110 and 32 bytes respectively. File count and delivered size move differently.

<a :href="withBase('/benchmarks/schema-programs-before-resources-results.json')">Raw results for that phase</a>
include six actual calls, programs, source/declarations, generation/check/emission
time and peak RSS. Measurement ran sequentially after regression and performance
checks. Timing is observational, not a speed guarantee. The
<a :href="withBase('/benchmarks/schema-programs-before-callables-results.json')">prior composition results</a>
are preserved. Shared HTTP, validation, error and selective/loading costs remain.

## Selecting resource input and capabilities

Current source `85bacbe1902d` compares with `b0aac7b4e740` on the same 1,000-API fixture. The table reports final deployed JS/gzip; the selected root stays unchanged and both full and named bundles shrink.

| Scope | Before JS | Current JS | Before gzip | Current gzip |
| --- | ---: | ---: | ---: | ---: |
| All 1,000 APIs | 514,082 | 513,604 | 61,359 | 61,192 |
| 10 selected APIs | 29,878 | 29,878 | 8,522 | 8,522 |
| Named client with those 10 APIs | 36,021 | 35,279 | 11,264 | 10,991 |

Named gzip saves 273 bytes (2.42%). Operation type declarations and attached JSDoc remain identical in all three scopes.

<a :href="withBase('/benchmarks/schema-programs-results.json')">Current raw results</a> include six strict-source, declaration and actual bundle-call checks, plus generation/compilation time and RSS. Timing observations may include concurrent validation load and do not establish a stable speed change.

## Earlier 1,000-operation selection measurement

The following historical benchmark compares 10 APIs from one 1,000-operation fixture, including an
operation without an ID and one private OpenAPI Link target. The fixture also
contains shared and recursive models, XML, and streaming operations. Each case
performs the same calls and receives the same responses.

::: info Measurement version
Measured on October 3, 2026, at commit `7fe7039`. Named clients and their implicit
root union are next-release features. These results describe that source version,
not every published release or every API document.
:::

## Generated source and resources

The full SDK generates every API. Root selection generates the 10 chosen APIs
and their Link dependency. Named selection assigns those same APIs to one named
client; its default root exposes their union. Selection plus named explicitly
repeats the same root choice to verify equivalent output.

<SelectionBenchmarks group="generation" />

Root and named selections reduce generation from roughly three seconds to under
0.1 seconds in this fixture. Named generation adds four entry/type files and
14,105 bytes, while sharing the API implementations. Its output is byte-identical
whether the root union is implicit or the same selection is repeated explicitly.

Selecting all APIs produces the same 6,060 files and bytes as full generation.
Selection metadata is opt-in through `--with metadata`; these measurements use
default generation without that add-on. Selecting every API is not a speedup.
Input loading and document validation still cover the whole source document.

## Application bundles

Root and named entries expose the same 10 public APIs. Static loader cases import
their operation references directly before calling `loadOperations`. Dynamic
lookup cases use `routes[route]`; their deployment includes both operation-ID
and route lookup entries for the generated scope. All cases retain the private
Link target, without exposing it as a public method.

<SelectionBenchmarks group="bundle" />

In this fixture, the named entry has the smallest deployed bundle and prepares
faster than the loader cases. The regular root entry prepares quickly, but its
deployed JS is larger than the static loader bundle. Full plus static
`loadOperations` and selection plus static `loadOperations` differ by only
35 deployed bytes: the bundler already removes unrelated APIs from static imports.
Selection still saves generation and source-checking work in that case.

Dynamic lookup initially imports about 6.2 KiB of JS. Preparation then reads
additional modules. The full lookup deployment contains 3,004 JS files for
arbitrary choices across the full API; the selected lookup deployment has 34.
The application does not load all those files to prepare these 10 APIs.
Reducing deployment assets and reducing bytes loaded by one visit are different
benefits. Multiple small files also have request and compression overhead.

## Reading the numbers

All size charts start at zero with a linear scale. Changing a metric recomputes
that scale; compare bars within the same chart. Time bars show medians, with
minimum–maximum whiskers. The adjacent table provides values without relying on
color, hover, or the graph. Narrow screens can scroll the graph horizontally.

“JS loaded to ready” includes the entry import and preparation of all 10 APIs,
before the first API request. “All deployed JS” includes unused lookup entries
and delayed chunks needed by that configuration. The raw measurements also
record the first import and loading after the private Link call.
Gzip level 6 and Brotli quality 5 are applied separately to each deployed file,
then summed. Filesystem allocation, source maps, HTML, HTTP headers, and the
documentation site's chart code are excluded from SDK bundle size.

Generation uses three fresh processes and empty output directories per case.
Bundles use three builds and six fresh execution processes per case, in rotated
order with a warm filesystem cache. Rolldown 1.2.6 produces minified, tree-shaken,
browser-platform ESM with splitting enabled. Dynamic lookup paths are preserved
by including their modules as bundler entries; no raw, unminified JS is copied
into the deployment. Mocked Fetch verifies calls and responses without real HTTP.

Execution time is measured in Node.js 24.21.0, not in a browser. It excludes
network latency, service workers, browser cache, and real server response time.
The machine uses an AMD Ryzen 7 9800X3D, about 15.2 GiB of memory, and Linux x64
under WSL2. Generation uses Go 1.27.1; emission uses TypeScript 7.0.2. Timing
emission retains generated `@ts-nocheck`; separate regression checks remove it
and verify strict TypeScript 6.0.3 and 7.0.2 types and execution behavior.

## Validation and remaining limits

The recorded checks cover overlapping selections without duplicate shared
implementations, independent root/named public types, rejection of private APIs
during direct preparation, transitive Link execution, metadata option behavior,
and byte equality for equivalent generation configurations. The full Go suite,
strict TypeScript 6/7 regression cases, documentation validation, formatting,
and Go vet passed. Earlier runtime validation passed 732 tests; seven optional
performance tests were skipped. Graph typechecking remains outside this scope.

The measurements identify useful follow-up work rather than proving that no
optimization remains. The regular root entry's bundle overhead and dynamic
lookup deployment/file costs merit further profiling. This fixture does not
measure multiple independently built pages, browser downloads, or arbitrary
selection densities.

An earlier 10,000-operation full-SDK consumer hit TypeScript inference complexity
errors with an explicitly typed `Client<Chosen>` variable and inferred
`createClient`. An explicit `createClient<Chosen>` type argument passed with
TypeScript 6 and 7 in that experiment. That large inference path was not fixed
or revalidated by this 1,000-operation benchmark. Do not interpret the focused
type regression passes as exhaustive success for all large consumers.

## Data and reproduction

<script setup>
import { withBase } from 'vitepress';
import SelectionBenchmarks from '../.vitepress/theme/components/SelectionBenchmarks.vue';
</script>

<p><a :href="withBase('/benchmarks/selection-results.json')">Summary and environment JSON</a> · <a :href="withBase('/benchmarks/selection-measurements.json.gz')">Complete current measurements (.json.gz)</a> · <a :href="withBase('/benchmarks/selection-validation-history.json.gz')">Earlier measurements and validation logs (.json.gz)</a></p>

The complete measurements include each trial, min/max values, CPU and memory
observations, file inventories and SHA-256 hashes, loaded modules, and request
traces. Earlier records retain their original version and conditions; older
named-only full-root results describe superseded behavior and are not used in
the current charts. Machine-local paths are removed from public downloads.

In a checkout with the local Go/Node toolchain and verification dependencies:

```sh
devtools run build:dev
devtools run selection:perf
```

The command writes `.tmp/selection-benchmark/report.json` and uses the repository's
fixture and execution checks. Run it without concurrent build or test workloads.
Performance measurement is optional and does not add a timing threshold to CI.
