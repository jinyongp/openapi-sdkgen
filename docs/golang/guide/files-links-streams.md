# Files, Links and streams

<span id="streams-files-and-custom-codecs"></span>

[Download the Todo example](/downloads/golang-todo-app.zip) to run Links and streams.
The snippets below run inside a function receiving `ctx context.Context`.

## Follow a response Link

Retrieve an item through the creation response's Link. `call` is a prepared `CreateTodo` call.

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

With the Todo example server, the output is:

```text
201 application/json
todo-1
```

`Raw` and `Do` each send a request. Reuse the `Raw` result when following its Link.

## Read a stream

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

With the Todo example server, the output is:

```text
Ship the SDK
```

The first `Next()` starts the exchange. Close the stream on early exits and inspect
`Err()` after iteration. `MaxStreamFrameBytes` limits one frame, not the complete response.

## Files and custom media

Prepare file input with `types.BinaryFromBytes`, `BinaryFromString` or `BinarySource`.
The SDK closes readers it opens during execution. One-shot readers cannot be replayed.

Multipart input supports named fields, positional parts and complete binary bodies.
For a complete binary body, supply the already framed bytes and their `Content-Type`,
including the boundary, with `types.BinaryFromBytesWithMetadata` or `BinarySource`.
The SDK preserves the bytes and checks the declared media type and boundary before sending.

See [Streaming API](../reference/streaming.md) for request streams, codecs and buffer ownership.
