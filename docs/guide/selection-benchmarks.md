# Compare selection costs

Choose APIs during generation to reduce generated source and checking work.
Choose an entry point to control what the application imports and deploys.
These are separate decisions: a small application bundle can come from a large
generated SDK, and a small initial import can still require many deployment files.

This benchmark compares 10 APIs from one 1,000-operation fixture, including an
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
devtools run dev:build
devtools run perf:selection
```

The command writes `.tmp/selection-benchmark/report.json` and uses the repository's
fixture and execution checks. Run it without concurrent build or test workloads.
Performance measurement is optional and does not add a timing threshold to CI.
