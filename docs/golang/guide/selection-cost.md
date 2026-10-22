# Selection and build cost

<span id="selection-and-compilation-cost"></span>


Go's linker removes unreachable code. Imported package initialization can remain, and the compiler
still processes imported packages. JavaScript tree-shaking and browser download chunk sizes are not
Go build settings. Choose the smallest suitable surface:

| Choice | Use | Cost boundary |
| --- | --- | --- |
| Root client | Broad application use | Full selected source dependencies |
| Named clients | Fixed feature boundaries | Independent entries, shared packages |
| Static `operations` refs + `selective` | Concrete operation types | Import only references used by application code |
| `LookupOperation` / `LookupRoute` | Computed selection | Lazy dynamic registry; dynamic values |
| Generation selection | Known API subset | Less emitted source and compile work |

Generation must still parse and analyze the input. Memory does not necessarily fall in proportion
to selected operations. Owned schema, URL and Unicode source still has compile and memory costs;
URL normalization is part of the HTTP client even with few operations. Static patterns are compiled
during generation. A runtime pattern compiler is included only for dynamic server schemas or
`regex` format assertions. Measure generation separately from SDK compilation, runtime allocations
and final binary size.

## Compare matched builds

In a POSIX shell, use a temporary cache to preserve other projects' build caches:

```sh
build_cache="$(mktemp -d)"
GOCACHE="$build_cache" go build -o ./app .
GOCACHE="$build_cache" go build -o ./app .
rm -rf "$build_cache"
go test -bench . -benchmem ./...
```

The first build uses an empty, isolated Go build cache, including standard-library work;
the second reuses it without source changes. This does not clear the operating system's file cache.
Use your shell's temporary-directory and environment-variable syntax on other platforms.
A faster unchanged build does not establish faster recompilation after editing an API.
Keep the same Go version, platform, input,
addons, selected operations and build flags. Record wall time, peak memory, emitted files/bytes,
compile time and binary size separately. Do not infer a fixed speedup from source size.
For selection syntax see [selected operations](./selective-client.md) and [named clients](./named-clients.md).

## Generated files and change costs

Generated declarations can share a source file. Fewer files can reduce file handling without
proportionally reducing source bytes, compiler memory, compiled package artifacts or the linked executable.
Changing one API can rewrite a larger shared file. Go caches compiled packages, so file-count
reductions alone do not establish cheaper builds after a change.
Generation can also take longer even when the output contains fewer files.

Compare the original and updated generators on the same document and options, alternating
their order over repeated runs. Use fresh outputs for generation timing. Measure builds with
an empty cache, unchanged builds that reuse it, and builds after source edits separately.
Report memory's measurement boundary: a process's maximum RSS,
the sum for concurrent processes, and retained runtime heap are different metrics.
Summing RSS can count shared pages more than once.
The [published compatibility results](../reference/compatibility.md) link their Actions runs;
they establish the recorded checks, not a before/after speedup across generator versions.
