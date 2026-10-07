# Files, Links, and streams

Use this guide when your API offers file transfers, follow-up calls, or sequential
data. Start with [basic client usage](./client.md) for ordinary JSON calls.

These snippets need more than the two operations in Getting started. Declare each
section's inputs, outputs, and parameters in your document and regenerate the SDK
before using them. `api` is a client configured for your real API server.

| Task | Required document contract |
| --- | --- |
| Send binary data or text | `uploadTodoAttachment` with both request media types and a `todoID` path parameter |
| Upload a file | A multipart `file` property on `uploadAttachment` |
| Retrieve the created item | A `getTodo` Link on the creation response and its target operation |
| Receive events sequentially | Response `itemSchema` and a `cursor` query on `watchTodos` |
| Send events sequentially | Request `itemSchema` on `publishTodoEvents` |

For a streaming document and its consumer code, see the
[AI streaming example](../examples/ai-streaming.md).

## Choose a request media type

If one request body declares several media types, the generated body input is a
discriminated value. Its `contentType` field selects the representation.

For a Todo attachment operation that accepts binary data or text:

```ts
await api.$operations.uploadTodoAttachment({
  path: { todoID: "todo-1" },
  body: {
    contentType: "application/octet-stream",
    value: new Uint8Array([1, 2, 3]),
  },
});
```

Use a content type declared by the operation. Custom media codecs are selected by
the same media type.

## Upload a multipart file

Raw file parts accept `File`, `Blob`, `ArrayBuffer`, `ArrayBufferView`, or a text
string. Strings are encoded as UTF-8. A `File` carries its filename; byte views
send only their selected byte range. Base64-encoded schema values remain strings.

For an operation whose multipart body declares a `file` property:

```ts
await api.$operations.uploadAttachment({
  body: { file: new File([new Uint8Array([0, 127, 255])], "attachment.bin") },
});
```

An Encoding Object's `contentType` takes precedence over the part's
`contentMediaType`. Raw file parts default to `application/octet-stream`.
The same schema used in a JSON body keeps its JSON input type.

## Follow OpenAPI Links

An OpenAPI Link describes a follow-up operation using values from a response.
When a Todo creation response defines a Link named `getTodo`, the generated
[`$links`](../reference/client-api.md#links) helper carries the source response context into that follow-up call:

```ts
const created = await api.$operations.createTodo.raw({
  body: {
    title: "Write documentation",
  },
});

const todo = await api.$links.createTodo.getTodo(created);
```

Values passed to the Link call take precedence over values derived from the Link
Object.
If a required runtime expression cannot be resolved, the call fails.

## Consume streaming responses

An operation with an OpenAPI 3.2
[`itemSchema`](https://spec.openapis.org/oas/v3.2.0.html#media-type-object)
exposes [`.stream(...)`](../reference/streaming.md#response-streams) on its
operation, exact-route, and generated resource call surfaces. See
[OpenAPI support](../reference/capabilities.md#supported-openapi-versions) for
the 3.0/3.1/3.2 version split.

```ts
const stream = api.$operations.watchTodos.stream({
  query: { cursor: "0" },
});

for await (const event of stream) {
  console.log(event.todoID, event.completed);
}
```

`.stream()` returns an
[`OperationStream<T>`](../reference/streaming.md#operationstream).
It starts lazily, preserves Fetch backpressure, and owns one response body. Use `stream.response` when status,
headers, content type, or request metadata are needed after the response opens.

```ts
const metadata = await stream.response;
console.log(metadata.status, metadata.request.id);
```

Call `stream.abort()`, pass an `AbortSignal`, or use a request timeout to stop
the operation. `stream.toReadableStream()` adapts the same single-consumer
source to the Web Streams API.

For the unconsumed Fetch response body, make a separate [`.raw()`](../reference/client-api.md#raw) call.
Server-Sent Events preserve `data`, `event`, `id`, and `retry`; replay and
reconnect policy stays in application code.

## Send streaming request bodies

An OpenAPI 3.2 request body with `itemSchema` accepts
[`StreamSource<T>`](../reference/streaming.md#streaming-request-bodies),
so an async iterable and a Web `ReadableStream` use the same generated input
type.

```ts

async function* todoEvents() {
  yield { todoID: "todo-1", completed: false };
  yield { todoID: "todo-1", completed: true };
}

await api.$operations.publishTodoEvents({
  body: todoEvents(),
});
```

If the sequential media type declares only `schema`, pass the complete schema
value instead. When it declares both `schema` and `itemSchema`, the generated
request body accepts either the complete value or a [`StreamSource<T>`](../reference/streaming.md#streaming-request-bodies).
