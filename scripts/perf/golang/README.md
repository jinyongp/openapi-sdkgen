# Optional Go measurements

Run `devtools run golang:perf runtime` for actual generated-SDK microbenchmarks
(five samples each) and separate forced-GC stream retention diagnostics.
`devtools run golang:perf generation` measures full and one-/ten-operation
selection on fixed 1,000/10,000-operation shared recursive schemas, five times.
`devtools run golang:perf micro` retains the narrow prepare/emit microbenchmarks.

Each run creates an owned directory under `.tmp/golang-perf`; `--output DIR`
requires a new directory. It preserves source, input, generator and generated
artifact hashes, Go/Node versions, flags, exact commands, exit codes, GNU time
metrics and raw output. A failure or timeout stops the run and is not a passing
sample. Command children are terminated as a process group on timeout.

Run measurements sequentially on an otherwise idle host. Keep diagnostic heap
sampling separate from timed workloads. Total allocations, live heap and peak
RSS describe different costs. Cold/warm consumer builds, provider generation,
cross-builds and dependency/symbol inspection are separate integration evidence;
these modes alone do not establish the entire performance plan.

These are local measurements, without fixed timing, percentage or source-size
CI gates. Public numeric claims require the project's GitHub Actions benchmark
workflow and a link to the measured run.

`devtools run golang:perf builds` measures four real consumers: a direct call in
full generation, generation-time selection, an independent named client, and
computed dynamic lookup. The named case configures both a Todo entry and a second
entry with the other XML/stream/regex APIs, then imports only the Todo entry.
Its source union therefore includes the full fixture. Each gets five cold builds with separate empty
`GOCACHE` directories and five warm builds using the corresponding same cache.
It runs every binary, records dependency graphs and unstripped symbols, then
builds and runs a separately labelled stripped binary. Inspect actual symbol
and dependency evidence before claiming any unused feature was removed.

`devtools run golang:perf platform` is a compatibility check, not a performance
comparison. It builds a generated consumer and server runtime with CGO disabled
for Linux/macOS/Windows on amd64/arm64, runs the native Linux amd64 binary, and
builds the real playground WASM entry. Off-host binaries are only cross-built.

`devtools run golang:perf providers` requires the already fetched, original
regression corpus under `.tmp/compatibility-regression`. It checks each manifest's
SHA256/git-blob pins, generates every fixed provider in full and with one/ten exact
sorted original routes, and takes five samples for each. Emission is formatted
and hashed through an artifact sink; it does not write the very large SDK trees.
The report labels this `golang-generation`; it proves no cross-target parity or
consumer compilation. Inputs are never edited. These runs use the same recorded
`GOMEMLIMIT=5GiB` and `GOMAXPROCS=2`; huge providers can take several minutes per
sample. Allow this mode to finish before starting another costly project command.
Provider processes have a 15-minute deadline and an observed 7GiB RSS process-group
safety bound. Crossing either stops owned children and records a failed sample;
resource-bound failures are not dropped from the raw results.
The small HTTP/JSON baseline constructs a fresh request, reads the same payload,
and checks its expected fields on every iteration. SDK request construction and
schema validation remain enabled. A separate concurrency run captures mutex and
block profiles; its profiled timings are not mixed into the five ordinary samples.

Stream timing and forced-GC retention use the public generated Events().Stream() path,
including HTTP request construction, declared item-schema validation and typed DTO
conversion. An invalid item is rejected in the diagnostic test before timings are
accepted. Framing-only historical samples are retained separately.
