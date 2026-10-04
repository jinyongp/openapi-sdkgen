# Compare selection costs

Choose APIs during generation to reduce generated source and checking work.
Choose an entry point to control what the application imports and deploys.
These are separate decisions: a small application bundle can come from a large
generated SDK, and a small initial import can still require many deployment files.

## Small API runtime cost {#small-api-runtime}

The current development generator automatically selects runtime handlers from
the generated APIs. This sample has three JSON APIs with Bearer authentication,
object/array validation, string-length and object-property constraints. All four bundles perform the
same `getItem` request and check the URL, credentials, and returned DTO with
mocked Fetch. The default entry needs no selection configuration.

| Generation | All TypeScript files | All source lines | Runtime files | Runtime lines |
| --- | ---: | ---: | ---: | ---: |
| Default generation | 94 | 7,754 | 49 | 5,623 |
| Full-capability test control | 180 | 16,275 | 142 | 14,467 |

| Application entry | JS chunks | Entry JS bytes | Entry gzip bytes | All deployed JS bytes | All deployed gzip bytes |
| --- | ---: | ---: | ---: | ---: | ---: |
| Full-capability test control | 1 | 106,930 | 30,805 | 106,930 | 30,805 |
| Default root | 1 | 31,904 | 9,603 | 31,904 | 9,603 |
| Static selective | 3 | 22,224 | 7,343 | 39,326 | 13,806 |
| Lazy selective | 7 | 369 | 274 | 40,536 | 15,029 |

The default root is **31.16 KiB of JS and 9.38 KiB gzip** in this sample,
about one third of the control's compressed size. Static and lazy preparation reduce
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
The generator source hash starts with `4c47c6012ef5`.
Conditions: Node.js 24.21.0, TypeScript 7.0.2, Vite 8.3.0 / Rolldown 1.2.6,
browser ESM, ES2022, Oxc minification, code splitting, and `modulePreload: false`.
Gzip level 6 is applied separately to each deployed chunk and then summed.
The raw report also records source, native module bytes, and minified bundle
bytes for all 73 fixtures. These additional bundles export the entire client
factory or webhook router, rather than the single call above. Examples of their
total deployed gzip bytes:

| Contract | Selected runtime | Full-capability control |
| --- | ---: | ---: |
| XML client | 16,989 | 30,163 |
| Multipart client | 19,032 | 30,179 |
| Mixed-media client | 21,021 | 31,897 |
| JSON webhook router | 12,199 | 23,627 |
| XML webhook router | 17,312 | 23,654 |
| Multipart webhook router | 17,278 | 23,629 |

Source counts include type-only files; source maps and HTTP overhead are excluded.
Mocked execution runs in Node.js; browser network and timing are not measured.
These are reproducible sample sizes, not a size guarantee for every API.

## One, two, or three APIs

The following comparison uses the captured working tree immediately before the
schema-program change and the current generator, with identical inputs and tool
settings. CLI `--operation` selects the APIs; each bundle calls every selected
API through the default root. The query case adds one required integer query
parameter. Bytes below use decimal units.

| Selected APIs | Before JS | Current JS | Before gzip | Current gzip | Current TS files / lines | Current runtime files / lines |
| --- | ---: | ---: | ---: | ---: | ---: | ---: |
| Get one | 31,082 | 29,569 | 10,160 | 9,283 | 80 / 7,043 | 48 / 5,655 |
| Get one with query | 34,536 | 32,890 | 11,072 | 10,174 | 82 / 7,348 | 49 / 5,912 |
| Get and list | 31,574 | 31,015 | 10,228 | 9,494 | 86 / 7,414 | 48 / 5,655 |
| Get and create | 32,011 | 30,575 | 10,289 | 9,430 | 86 / 7,406 | 49 / 5,672 |
| Get, list, and create | 32,495 | 32,013 | 10,358 | 9,633 | 92 / 7,772 | 49 / 5,672 |

All five bundles shrink in both JS and gzip. Responsibility-based modules
increase the file count; file count alone does not indicate delivered size.
The generator emits contract-specific validation/transformation programs and
shares equivalent programs. Ordinary JSON imports the selected execution
operators without the generic schema interpreter. Explicit arbitrary-schema
helpers retain their complete compatibility implementation.

The three-API row calls all three APIs. The default-root table above calls only
`getItem`, so its bundle size differs. Generated source inventories can also
differ when the fixture options differ.
<a :href="withBase('/benchmarks/schema-operation-results.json')">Before/after raw measurements</a>
record strict source compilation and actual mocked bundle calls for every row.
These sample results do not establish a universal minimum SDK size.

## Current 1,000-API schema-program comparison

Using the same captured source baseline, the current generator shares six
schema programs across the complete 1,000-API fixture. Selecting ten public APIs
plus their private Link target produces four programs. The named client shares
exactly the same program files and bytes as root selection.

Numbers below show before → current. Each browser bundle performs `getItem0`;
the root entries still retain the generated API surface of their scope.

| Case | TS files | TS bytes | Shared programs | Declaration bytes | Deployed gzip bytes |
| --- | ---: | ---: | ---: | ---: | ---: |
| All 1,000 APIs | 6073 → 6090 | 14,909,005 → 15,284,115 | 0 → 6 | 9,451,987 → 9,547,766 | 61,027 → 61,489 |
| 10 selected APIs | 119 → 134 | 407,194 → 421,476 | 0 → 4 | 215,670 → 223,535 | 10,152 → 9,492 |
| Named client with those 10 APIs | 123 → 138 | 420,829 → 435,141 | 0 → 4 | 226,615 → 234,497 | 12,672 → 12,107 |

The complete SDK grows by 462 gzip bytes (0.76%) and about 375 KB of TypeScript
source. The selected root saves 660 gzip bytes (6.50%); named saves 565 (4.46%).
Contract-specific generation improves selected delivery size here, while adding
source and declaration cost. Sharing avoids one program copy per operation;
it does not guarantee that every full SDK becomes smaller.

| Current scope | Generation median ms / peak RSS KiB | Strict source ms / peak RSS KiB | Declaration emission ms / peak RSS KiB |
| --- | ---: | ---: | ---: |
| All 1,000 APIs | 3,065.8 / 60,440 | 3,171.2 / 1,049,868 | 2,708.3 / 1,139,744 |
| 10 selected APIs | 104.5 / 32,304 | 295.9 / 148,640 | 287.3 / 141,664 |
| Named client with those 10 APIs | 102.6 / 32,092 | 304.4 / 146,656 | 295.5 / 153,312 |

Generation uses three fresh processes and empty output directories. Each strict
source and declaration check uses one fresh compiler process; generated
`@ts-nocheck` is removed. Runs are serial with a warm filesystem cache, so
these timings are observations rather than stable speedup claims. The report
includes the corresponding baseline timings, all trials, input/binary/program
hashes, source/declaration counts, and executed bundle results. Compiler version
and bundle settings match the small measurements above. This does not resolve
the earlier 10,000-API inference limit described below.

<a :href="withBase('/benchmarks/schema-programs-results.json')">Current comparison raw results</a>.

```sh
devtools run schema-programs:perf
# To compare a captured source tree:
devtools run schema-programs:perf .tmp/schema-programs-measurement /path/to/captured-source
```

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
