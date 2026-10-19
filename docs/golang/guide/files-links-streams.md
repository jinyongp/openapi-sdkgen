# Files, Links and streams

Continue [calls and errors](./client.md) with the [Todo document](/examples/golang-todo.json).
Run snippets inside a function with `ctx context.Context` and an `error` return; `client` and `call`
are the configured client and CreateTodo call. Import `types` from your generated root when needed.

<span id="streams-files-and-custom-codecs"></span>


`Raw` retains status, headers and the full decoded value. Declared variant accessors such
as `As201JSON()` return a typed value and a boolean. Use the original raw response to follow
a Link; the generated Link resolves inputs and validates the target before a later execution.

```go
raw, err := call.Raw(ctx)
if err != nil {
    return err
}
fmt.Println(raw.Status, raw.Headers.Get("Content-Type"))
todo, err := client.Links.CreateTodo.GetTodo.Follow(raw).Do(ctx)
if err != nil {
    return err
}
fmt.Println(todo.ID)
```

Call `Raw` or `Do` once for a desired exchange: each method starts a separate execution.
For untyped sequential `Raw`, close its owned `Body` after reading. `Stream` closes its
response/source lease when exhausted, canceled, or explicitly closed; close it when stopping early.

## Streaming and file ownership

Sequential output is pull based. The first `Next` starts execution; processing each item
controls demand. `MaxStreamFrameBytes` bounds one frame, not the total response or
a compression decoder's history window. Request options override client defaults;
the SDK chooses a stream `Accept` only when neither sets it. Always inspect `Err`
after the loop:

```go
events := client.Operations.WatchTodos().Stream(ctx)
defer events.Close()
for events.Next() {
    fmt.Println(events.Item().Title)
}
if err := events.Err(); err != nil {
    return err
}
```

Finite files use `types.Binary`; use `BinaryFromBytes`, `BinaryFromString`, or a
`BinarySource` that supplies metadata and `Open(context.Context)`. The source owns no
request before execution. The SDK owns and closes each successfully opened reader.
A one-shot reader created with `BinaryFromReader` cannot be replayed. Byte-backed inputs
borrow their bytes; keep them unchanged during execution. Accessors do not open a source.

Typed incremental input uses `types.SourceFactory[T]`. Provide a fresh source on every
execution or explicitly use `types.SourceOnce`. The SDK cancels the execution context before
closing owned sources. Use `types.MediaCodec` for custom media and
`types.NewStreamCodec(protocol, adapter)` for framing and item conversion. A nil direction
is unsupported. Codecs receive HTTP metadata snapshots and borrowed body leases; the SDK
retains the final Close. Shared mutable state in hooks must be safe for concurrent calls.
Open media values use `types.MediaValue[T]`: `AsValue`, `AsBinary` and `AsMultipart` distinguish
validated JSON-shaped values, buffered bytes and native file slots.

See [streaming API](../reference/streaming.md) for framing, cancellation and codecs.
