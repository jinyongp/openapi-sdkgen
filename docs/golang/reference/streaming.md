# Streaming API

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

## Lifetime and demand

`Stream[T]` is lazy: `Next()` or `Response()` starts the exchange. `Response()` reads headers without
consuming an item. Only one `Next` may run at a time. After `Next()` returns false, inspect `Err()`.
`Close()` interrupts execution and closes owned sources; explicit close itself is not an iteration error.
Context cancellation remains observable. Backpressure follows pull demand; no background item queue is promised.

SSE, NDJSON, JSON sequence and declared incremental media follow the operation contract.
A frame limit does not bound total bytes or decompressor history. Typed errors preserve execution context.
For raw sequential output, the caller owns and closes the body. For custom `MediaCodec` and `StreamCodec`,
implement only supported directions; missing directions fail explicitly. See [files and Links](../guide/files-links-streams.md).
