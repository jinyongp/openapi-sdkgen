# Selection and build cost

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
to selected operations. Compression, schema and URL dependencies have their own compile and memory
costs. Measure generation separately from SDK compilation, runtime allocations and final binary size.

## Compare matched builds

```sh
go clean -cache
go build -o ./app .
go build -o ./app .
go test -bench . -benchmem ./...
```

The first build is cold; the second reuses the cache. Keep the same Go version, platform, input,
addons, selected operations and build flags. Record wall time, peak memory, emitted files/bytes,
compile time and binary size separately. Do not infer a fixed speedup from source size.
For selection syntax see [selected operations](./selective-client.md) and [named clients](./named-clients.md).
