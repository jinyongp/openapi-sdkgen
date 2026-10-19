# Generate and use a Go SDK

Generate typed Go source, call your API with `context.Context`, and mount webhook or
callback handlers in your own `net/http` server. SDK consumption requires **Go 1.27+**.
A downloaded generator executable can generate source without Go, Node.js, or TypeScript.
The npm launcher requires Node.js; the generated Go SDK does not.

For a complete local run, use the [Go Todo example](https://github.com/jinyongp/openapi-sdkgen/tree/main/examples/golang-todo-app).
It runs Raw, Links, resources, streaming, static selection and a webhook against local
servers and prints `created: 201`, `linked: todo-1 Ship the SDK`, and the remaining results.
Go support is included in this source revision and will be available in the next release.

## Generate into your module

Start in an existing Go module, or run `go mod init example.com/consumer`.
Save the [example document](/examples/golang-todo.json) as `openapi.json`. This example
contains a webhook, so enable the `server` addon:

```sh
openapi-sdkgen generate \
  --input ./openapi.json \
  --target golang \
  --go-import-path example.com/consumer/internal/api \
  --output ./internal/api \
  --with server \
  --incremental
```

`--go-import-path` is the generated package's full import path, including its directory
inside your module. It is required and independent of the output filesystem path.
`--go-package` sets the root package name and defaults to `api`. Generation writes no
`go.mod` or `go.sum`; your module owns dependency versions. Use `--incremental` to update
managed files while retaining your own files.

The example's generated source uses these pinned dependencies:

```sh
go get github.com/andybalholm/brotli@v1.2.6 \
  github.com/klauspost/compress@v1.20.3 \
  github.com/nlnwa/whatwg-url@v0.6.2 \
  github.com/go-quickjs/go-quickjs@v0.27.0 \
  github.com/bits-and-blooms/bitset@v1.20.0 \
  golang.org/x/net@v0.61.0 golang.org/x/text@v0.43.0
go mod tidy
```

Compression and URL parsing are shared by HTTP clients. ECMAScript regular expressions
and some format checks use `go-quickjs` through the generated schema layer; only generated
features that import a dependency need it. No separate SDK runtime module is installed.

## Typed calls and presence

Import your generated root as `api`, then construct a client and call an operation.
The snippets below run inside a function with a `ctx context.Context` and an `error` return.

```go
client, err := api.NewClient(api.ClientOptions{BaseURL: "https://api.example.com"})
if err != nil {
    return err
}
call := client.Operations.CreateTodo(api.CreateTodoInput{
    Body: api.CreateTodoBody{Title: "Ship the SDK"},
})
todo, err := call.Do(ctx)
if err != nil {
    return err
}
fmt.Println(todo.ID, todo.Title)
```

`Operations` preserves the operation API; `Routes` provides method/path entry points.
Resource methods bind path values, for example `client.Todos.ByID("todo-1").Get().Do(ctx)`.
Constructing a client, selector or call does not send HTTP. Every `Do`, `Raw` or stream
execution uses its own context. Calls can execute again when their input sources can reopen.

Required members use their generated type. Optional members use `types.Optional[T]`:
`types.Some(value)` is present, the zero value is absent. A nullable value uses
`types.Nullable[T]`; null and absence remain distinct. `types.Value` is an immutable JSON
wire tree, not a container for files or live streams. Input validation happens before HTTP;
response decoding preserves allowed unknown response properties. Standalone model codecs
and server output encoding still enforce their contracts.

```go
input := api.CreateTodoInput{Body: api.CreateTodoBody{Title: "Ship the SDK"}}
call := client.Operations.CreateTodo(input)
call = call.WithOptions(types.RequestOptions{RequestID: types.Some("example")})
```

## Raw responses and Links

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

## Streams, files, and custom codecs

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

## Selection and compilation cost

Static references preserve concrete operation types:

```go
ref := operations.GetTodo()
prepared, err := selective.Prepare(ref)
if err != nil {
    return err
}
client, err := selective.NewClient(selective.ClientOptions{BaseURL: "https://api.example.com"}, prepared)
if err != nil {
    return err
}
operation, err := selective.Bind(client, ref)
if err != nil {
    return err
}
todo, err := operation.Call(api.GetTodoInput{Path: api.GetTodoPath{ID: "todo-1"}}).Do(ctx)
```

Import `operations` and `selective` under your generated root. The linker removes unreachable
code, so Go needs no JavaScript bundler or tree-shaking configuration.
Imported package initialization can remain even when unused methods disappear.
Generation selection can also avoid importing unused runtime dependencies.
However, the compiler still checks imported packages. Use generation selection
(`--operation getTodo` or `--route 'GET /todos/{id}'`) to reduce generated source and
compile work. Link dependencies remain internal. Named clients in TOML have independent
entry packages. Computed `LookupOperation`/`LookupRoute` selection initializes its registry
lazily and returns a dynamic operation with `types.Value` or `types.NativeInput` inputs.
No fixed binary-size or build-speed improvement is promised.

Generation selection reduces emitted source, but the input document still needs to be
parsed and analyzed. Generation memory therefore need not fall in proportion to the
number of selected operations. For large specifications, measure generation and SDK
compilation separately, using the same input and selection for each comparison.

## Security and transport

Use `ClientOptions.SecurityProvider` to return credentials for the declared scheme identities.
Direct credentials and scoped server selection use generated types; alternatives and scheme
metadata are available to your provider. `HTTPClient` and `Transport` are alternatives.
A custom transport must declare whether response bytes are encoded or already decoded with
original headers. The SDK decodes response compression and keeps original headers. Do not
silently decompress while retaining an encoded capability declaration.

Request timeout and cancellation use the execution context. HTTP/1 connection reuse needs a
consumed or closed response. Host transports and hooks must support concurrent invocations.
Go accepts valid HTTP token methods and does not apply browser Fetch's forbidden-header or
method rules. A separate CONNECT tunnel API is not provided.

## Webhooks and callbacks

Import `server/webhooks` or `server/callbacks` from the generated root. Mount a typed handler
at a host-selected path; your application owns the listener and callback URL registration:

```go
handler, err := webhooks.NewCreatedPostHandler("/hooks/created", webhooks.Options{},
    func(ctx context.Context, input webhooks.CreatedPostInput) (webhooks.CreatedPostResponse, error) {
        fmt.Println(input.Body.ID)
        return webhooks.NewCreatedPost204NoContentResponse(), nil
    })
if err != nil {
    return err
}
mux.Handle("/hooks/created", handler)
```

Required authentication uses `Options.Authenticate`. Parameters are decoded before this
hook; it receives original security requirements/candidates, five parameter groups, context,
and an HTTP metadata snapshot with `http.NoBody`. A nil rejection continues; a rejection
returns its status/headers/source. Anonymous alternatives skip the hook. Missing authentication
returns401; a hook error returns500. `CloneBody(ctx)` returns an independent reader for
bounded complete input; the caller closes the clone. Incremental input cannot be cloned.

Complete input defaults to8 MiB, with413 on overflow. Incremental frames default to1 MiB,
with400 on overflow; the aggregate stream has no complete-body cap. Override the limits with
`MaxBodyBytes` and `MaxStreamFrameBytes`. HTTP/1 duplex requires a writer that supports
`EnableFullDuplex`, and streaming needs flush support. HTTP/2 uses native multiplexing.
A failure after headers aborts the response and reaches `OnError`; it cannot replace the
already sent status. Keep input sources alive until the response finishes when echoing input.
The SDK closes the lease after sending.

`server/runtime.CompileSchema` is the separate generic path for normalized schemas and a
definition map. It accepts `types`, `reference`, `constValue` and other normalized keywords,
not raw JSON Schema `type` or `$ref`. It never fetches URLs or executes schema extensions:

```go
schemaJSON := []byte(`{"types":["object"],"properties":{"count":{"types":["integer"]}},"required":["count"]}`)
normalized, err := types.ParseValue(schemaJSON)
if err != nil {
    return err
}
contract, err := runtime.CompileSchema(normalized, nil)
if err != nil {
    return err
}
value, err := contract.Decode(ctx, []byte(`{"count":2}`))
if err != nil {
    return err
}
return contract.Validate(ctx, value)
```

## Troubleshooting

A missing import path is a generation error. A missing dependency is resolved in your own
module; generation does not install it. For incremental output, keep the managed manifest
with the directory. Inspect `types.APIError` using `errors.As` and stable error codes; error
text omits private request/response data:

```go
var failure *types.APIError
if errors.As(err, &failure) {
    fmt.Println(failure.Code)
}
```

XML supports the documented namespace/attribute/CDATA/wrapped and typed composition paths.
Arbitrary additional-property and positional prefix-item XML mappings retain the existing
mapping limits; select JSON or an explicit codec when XML cannot express the schema.
Use the [Go Todo example](https://github.com/jinyongp/openapi-sdkgen/tree/main/examples/golang-todo-app)
for the complete import set, local server and executable error/cleanup handling.
