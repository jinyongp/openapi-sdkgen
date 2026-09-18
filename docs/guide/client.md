# Use the generated client

The generated TypeScript source exposes three complementary ways to call an
operation:

- resource methods such as `api.todos.create()` for readable application code;
- `$routes` when the HTTP method and OpenAPI path are the stable identifier;
- `$operations` when the document declares an `operationId`.

All three surfaces call the same OpenAPI operations. Choose the one that makes the
caller easiest to understand.

## Configure a client

```ts
import { createClient } from "./generated/api";

const api = createClient({
  baseURL: "https://api.example.test/v1",
});
```

When `baseURL` is omitted, the client uses applicable OpenAPI Server Objects.
An operation-level server takes precedence over a path-level server, which takes
precedence over a root server.

Authentication and custom Fetch behavior belong in client configuration or
request options. See [Authentication, transport, and streams](./transport.md).

## Call Todo operations

Resource methods are the most readable choice when the generated resource tree
matches the way your application talks about the API:

```ts
const created = await api.todos.create({
  body: { title: "Write documentation" },
});

const todos = await api.todos.list({
  query: { completed: false },
});
```

Use `$routes` to identify an operation by HTTP method and OpenAPI path, including
operations with no `operationId`:

```ts
const todos = await api.$routes["GET /todos"]({
  query: { completed: false },
});
```

Use `$operations` when `operationId` is the stable application-facing name:

```ts
const todos = await api.$operations.listTodos({
  query: { completed: false },
});
```

## Read status and headers with `.raw()`

A normal call returns the generated successful output value. Use `.raw()` when
the application also needs the exact status, decoded response headers, selected
content type, or original Fetch `Response`.

```ts
const result = await api.$operations.getTodo.raw({
  path: { todoID: "todo-1" },
});

if (result.status === 200) {
  console.log(result.data.title);
  console.log(result.response.headers.get("etag"));
}
```

The generated raw response is status-aware, so TypeScript can narrow fields
based on `result.status`.

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

## Follow OpenAPI Links

An OpenAPI Link describes a follow-up operation using values from a response.
When a Todo creation response defines a Link named `getTodo`, the generated
`$links` helper carries the source response context into that follow-up call:

```ts
const created = await api.$operations.createTodo.raw({
  body: {
    title: "Write documentation",
    callbackUrl: "https://app.example.test/todo-status",
  },
});

const todo = await api.$links.createTodo.getTodo(created);
```

Values passed to the Link call take precedence over values derived from the Link
Object.
If a required runtime expression cannot be resolved, the call fails.

## Consume streaming responses

An operation with an OpenAPI 3.2 `itemSchema` exposes `.stream(...)` on its
operation, exact-route, and generated resource call surfaces.

```ts
const stream = api.$operations.watchTodos.stream({
  query: { cursor: "0" },
});

for await (const event of stream) {
  console.log(event.todoID, event.completed);
}
```

`.stream()` returns an `OperationStream<T>`. It starts lazily, preserves Fetch
backpressure, and owns one response body. Use `stream.response` when status,
headers, content type, or request metadata are needed after the response opens.

```ts
const metadata = await stream.response;
console.log(metadata.status, metadata.request.id);
```

Call `stream.abort()`, pass an `AbortSignal`, or use a request timeout to stop
the operation. `stream.toReadableStream()` adapts the same single-consumer
source to the Web Streams API.

For the unconsumed Fetch response body, make a separate `.raw()` call.
Server-Sent Events preserve `data`, `event`, `id`, and `retry`; replay and
reconnect policy stays in application code.

## Send streaming request bodies

An OpenAPI 3.2 request body with `itemSchema` accepts `StreamSource<T>`, so an
async iterable and a Web `ReadableStream` use the same generated input type.

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
request body accepts either the complete value or a `StreamSource<T>`.

## Where to go next

- [Authentication, transport, and streams](./transport.md) explains credentials,
  transport capabilities, cancellation, and custom Fetch behavior.
- [Generated client API](../reference/client-api.md) is the lookup reference for
  exports and error helpers.
- [Generated TypeScript types](../reference/typescript-types.md) shows how to
  extract request and response types from the generated contract.
