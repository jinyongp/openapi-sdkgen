# Client API

## `NewClient` and `ClientOptions` {#clientoptions}

`api.NewClient(options)` returns `(*api.Client, error)` and sends no request.
`BaseURL` is an absolute HTTP(S) URL without credentials, query or fragment. Omit it to use
applicable OpenAPI servers; operation scope wins over path scope, then root scope.

| Option | Purpose |
| --- | --- |
| `Server`, `Origin`, `BaseURL` | Scoped OpenAPI server selection and URL resolution |
| `HTTPClient` or `Transport` | Host HTTP implementation; mutually exclusive |
| `SecurityProvider` | Resolve declared security requirements and scheme identities |
| `Headers`, `Accept`, `Authorization` | Default headers and media preferences |
| `Credentials`, `CSRFToken`, `RequestID` | Declared credential/request policies |
| `Timeout`, `MaxStreamFrameBytes` | Execution timeout and per-frame bound |
| `MultipartHeaders`, `MultipartContentTypes` | Declared multipart part overrides |
| `Codecs`, `StreamCodecs` | Custom media and stream adapters |

Optional request settings use `types.Some(value)`. Call `WithOptions(types.RequestOptions{...})`
to create a call with overrides. Option containers are copied; DTOs and byte buffers remain
application-owned. Keep borrowed data unchanged during execution. Shared providers/codecs/transports
must support concurrent calls. See [security and transport](../guide/transport.md).

## Operations, routes and resources

`client.Operations.CreateTodo(input)` and generated `Routes` methods create typed calls.
Resource selectors bind path values, such as `client.Todos.ByID("todo-1").Get()`.
Names follow the document; `inspect --target golang` reports them.
`Do(ctx)` executes and returns successful output; `Raw(ctx)` retains the complete decoded response,
status and headers. Each execution sends a separate request. Declared response accessors return
`(value, bool)`. Input, encoding, transport, HTTP and decoding failures return errors.

## Errors {#errors}

Use `errors.As` for `*types.APIError` or a generated operation HTTP error. Stable helpers include
`IsAPIError`, `GetErrorCode`, `IsErrorCode`, `GetRequestID`. Cancellation/deadlines are mapped to
SDK codes while preserving the cause. Metadata snapshots do not hand ownership of live bodies to hooks.
See [calls and errors](../guide/client.md) for an executable starting point.

## Links and pagination

`client.Links.CreateTodo.GetTodo.Follow(raw)` prepares the Link target from its source response.
It validates resolved inputs before execution. A declared `x-pagination` produces `Paginate(ctx)`;
when mode is `both`, pass the generated cursor/offset constant. Paginators use `Next`, `Item`, `Err`
and `Close`; cancellation stops subsequent requests. See [extensions](./extensions.md).

## OpenAPI metadata {#openapi-metadata}

With `--with metadata`, import the generated `metadata` package. `Version` and `VersionLine`
identify the OpenAPI version. `Document()` returns an independent `json.RawMessage` copy of the
original entry document. Unknown `x-*` fields remain data and do not change SDK behavior.
This addon is separate from the model/client runtime and is not required to make API calls.
