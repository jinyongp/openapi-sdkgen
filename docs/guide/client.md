# Use the generated client

The generated TypeScript source exposes three complementary ways to call an
operation:

- [resource methods](../reference/client-api.md#resource-methods) such as `api.todos.create()` for readable application code;
- [`$routes`](../reference/client-api.md#routes) when the HTTP method and OpenAPI path are the stable identifier;
- [`$operations`](../reference/client-api.md#operations) when the document declares an `operationId`.

All three surfaces call the same OpenAPI operations. Choose the one that makes the
caller easiest to understand.

For an application that uses only part of the API, see
[Load only the operations you use](./selective-client.md). That entry prepares a
selection before creating the client; the regular root entry below remains the
full client.

## Configure a client

Use [`createClient`](../reference/client-api.md#createclient) to configure one generated client.

```ts
import { createClient } from "./generated/api";

const api = createClient({
  baseURL: "https://api.example.test/v1",
});
```

When [`baseURL`](../reference/client-api.md#clientoptions) is omitted, the client uses applicable OpenAPI Server Objects.
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

Use [`$routes`](../reference/client-api.md#routes) to identify an operation by HTTP method and OpenAPI path, including
operations with no `operationId`:

```ts
const todos = await api.$routes["GET /todos"]({
  query: { completed: false },
});
```

Use [`$operations`](../reference/client-api.md#operations) when `operationId` is the stable application-facing name:

```ts
const todos = await api.$operations.listTodos({
  query: { completed: false },
});
```

## Read status and headers with `.raw()`

A normal call returns the generated successful output value. Use [`.raw()`](../reference/client-api.md#raw) when
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

## Work with decoded response objects

Schema-defined response records use ordinary JavaScript objects with
`Object.prototype`, including nested records, `.raw().data`, declared error
bodies, multipart results, stream items, and pagination items. They support
`instanceof Object` and strict deep comparison with ordinary object literals.
Earlier generated clients used null-prototype records for these mappings;
regenerate the SDK if your application relies on the new behavior.

JSON names such as `__proto__`, `constructor`, and `hasOwnProperty` remain own
data properties. Use `Object.hasOwn(value, name)` when testing for a field, since
a declared field can shadow an inherited object method. This conversion happens
where schema mappings already construct records. Opaque values and native
objects such as `Blob`, typed arrays, and streams retain their existing identity
and representation.

Response/output schema types are mutable too: you can edit fields and nested
records, push into arrays, update tuples, and assign map entries. `readOnly` in
OpenAPI means a property is omitted from the request projection; it does not
make the returned field immutable. `writeOnly` properties remain absent from
output types. Input projections still accept readonly arrays and objects.
Enum literals, client catalogs, and metadata keep their existing constraints.

When migrating, response mocks and server handlers must supply mutable arrays
where the output contract declares arrays. A deeply readonly fixture may need
an editable copy. Spreading its parent only copies the parent; nested objects
and arrays are still shared, and `structuredClone` keeps its argument's static
TypeScript type. Editing a returned DTO does not send a request or persist the
change on the server.

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
    callbackUrl: "https://app.example.test/todo-status",
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

## Status and media in successful responses

Raw responses use the finite successful status range `200`–`299`. An exact
status narrows its body when a more general response uses the same media type:
with `200: Item` and `default: Problem`, `status === 200` gives `Item`, while
`202` can still return `Problem`. A same-media `2XX` response covers every
successful status, so its `default` body stays in the HTTP error contract and
does not broaden normal, raw, streaming, or pagination results.

Different media, wildcard ranges, and bodyless declarations remain distinct
where they can still be selected. Raw `contentType` is the normalized concrete
response header. Wildcard declarations use a string or a template literal type,
rather than claiming that the server returned a wildcard header.

## Narrow declared HTTP errors

Use `isOperationHTTPError(error, api.$operations.operationID)` to narrow a failure
to that operation's declared HTTP responses. The same helper accepts `.raw()`,
`.stream()`, and bound resource methods. Within the guard, `status` narrows the
decoded `data`; `contentType` distinguishes declared media representations, and
a required literal error code narrows its corresponding `details`.

The guard checks the actual SDK call and validates the current decoded body.
Manually constructed errors, failures from another operation, transport failures,
and invalid or subsequently modified bodies do not pass. `contentType` is the
selected declaration; use `error.response.headers` for the actual response header.
`OperationHTTPError<typeof method>` extracts this declared union. Existing
`APIError<Code, Details>` and `isErrorCode` calls remain available, including codes
that are not declared in the document.

## Where to go next

- [Authentication, transport, and streams](./transport.md) explains credentials,
  transport capabilities, cancellation, and custom Fetch behavior.
- [Generated client API](../reference/client-api.md) is the lookup reference for
  exports and error helpers.
- [Generated TypeScript types](../reference/typescript-types.md) shows how to
  extract request and response types from the generated contract.
- [Streaming API](../reference/streaming.md) is the lookup reference for stream
  lifecycle, request sources, codecs, and frame limits.
