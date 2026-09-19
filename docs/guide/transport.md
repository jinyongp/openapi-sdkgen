# Authentication, transport, and streams

The generated client uses Fetch as its transport boundary. Most applications configure a base URL and credentials. Configure a custom transport for runtime-specific capabilities such as a cookie jar,
access to restricted response headers, or mutual TLS.

## Provide ordinary Bearer credentials

When one Bearer credential is enough for the operation, pass the complete
Authorization header value:

```ts
const api = createClient({
  baseURL: "https://api.example.test",
  authorization: "Bearer example-token",
});
```

Applications handle login, token refresh, and credential storage.

## Choose among OpenAPI security alternatives

OpenAPI can declare several Security Requirement Objects for one operation. If
more than one effective requirement is available, the generated request options
require [`securityRequirement`](../reference/client-api.md#security-requirements) so the application chooses which alternative it
is satisfying.

For a Todo update operation that accepts either `userAuth` or `serviceAuth`:

```ts
await api.$operations.updateTodo(
  {
    path: { todoID: "todo-1" },
    body: { completed: true },
  },
  {
    securityRequirement: "userAuth",
    authorization: "Bearer example-token",
  },
);
```

The generated TypeScript type exposes the allowed requirement IDs for
autocomplete and static checking. With one effective requirement, the SDK selects
it automatically. An empty requirement represents anonymous access and uses the
ID `"anonymous"` when it participates in a choice.

## Load credentials with `securityProvider`

Use [`securityProvider`](../reference/client-api.md#clientoptions) when credentials are acquired dynamically for the
selected requirement.

```ts
const api = createClient({
  baseURL: "https://api.example.test",
  securityProvider: async ({ operation, requirement, origin }) => {
    if (requirement.id === "serviceAuth") {
      return {
        serviceAuth: {
          kind: "api-key",
          value: await getTodoServiceToken(operation, origin),
        },
      };
    }

    return {
      userAuth: {
        kind: "http-bearer",
        token: await getTodoUserToken(operation, origin),
      },
    };
  },
});
```

The provider receives the final operation, selected requirement, and origin.
The client validates returned credential shapes and applies them to the declared
OpenAPI security scheme.

Generated clients support API keys, HTTP Basic and Bearer authentication,
OAuth2, OpenID Connect, and mTLS. OAuth/login UX, token refresh, and persistent
credential storage remain host concerns.

## Cookie authentication

For browser-managed cookie authentication, set
[`ClientOptions.credentials`](../reference/client-api.md#clientoptions):

```ts
const api = createClient({
  baseURL: "https://api.example.test",
  credentials: "include",
});
```

The browser and Fetch policy decide which ambient cookies are sent.

For cookie-jar behavior outside the browser, use a transport that provides that
capability.

## Pass declared request headers

Headers declared as OpenAPI parameters are generated under [`headerParams`](../reference/client-api.md#request-headers).

```ts
await api.$operations.createTodo({
  headerParams: { "Idempotency-Key": requestID },
  body: {
    title: "Write documentation",
    callbackUrl: "https://app.example.test/todo-status",
  },
});
```

The active Fetch environment controls headers such as `Origin`, `Host`, `Cookie`,
and `Sec-*`, including whether caller-provided values can be applied.

## Configure a custom transport

[`ClientOptions.transport`](../reference/client-api.md#clientoptions) supplies a Fetch-compatible function and its supported capabilities.

```ts
const api = createClient({
  baseURL: "https://api.example.test",
  transport: {
    fetch: undiciFetch,
    capabilities: {
      cookieJar: true,
      readableResponseHeaders: ["set-cookie"],
      mutualTLS: true,
    },
  },
});
```

Environment-specific request behavior can also live in the transport:

```ts
const api = createClient({
  baseURL: "https://api.example.test",
  transport: {
    async fetch(input, init = {}) {
      const headers = new Headers(init.headers);
      headers.set("Origin", trustedOrigin);
      return fetch(input, { ...init, headers });
    },
  },
});
```

## Cancel a request or set a timeout

[Request options](../reference/client-api.md#request-options) accept an `AbortSignal` and a timeout:

```ts
const controller = new AbortController();

const todos = await api.todos.list(
  { query: { completed: false } },
  { signal: controller.signal, timeoutMS: 5_000 },
);
```

The same request options are available to operation, route, resource, Link, and
stream calls where applicable.

## Streaming behavior

openapi-sdkgen supports OpenAPI 3.0.x, 3.1.x, and 3.2.x. A normal `schema`
can describe the complete buffered value for known sequential content types on
all supported version lines. OpenAPI 3.2 `itemSchema` adds typed incremental
input and output.

See [Streaming API](../reference/streaming.md#openapi-version-support) for the
exact version and media-type contract.

An operation with `itemSchema` exposes `.stream()` and returns an
[`OperationStream<T>`](../reference/streaming.md#operationstream). Incremental
request bodies accept
[`StreamSource<T>`](../reference/streaming.md#streaming-request-bodies), so
callers can provide an `AsyncIterable<T>` or a Web `ReadableStream<T>`.
When a sequential Media Type Object declares both `schema` and `itemSchema`,
the generated request type accepts both complete and incremental inputs.

[`maxStreamFrameBytes`](../reference/streaming.md#maxstreamframebytes) limits
one wire frame, record, or multipart part before application adaptation.

### Adapt a built-in protocol

Use
[`StreamAdapter<Frame, Item>`](../reference/streaming.md#streamadapter) when
the wire framing is already supported but the application has another semantic
layer. For example, an application can decode JSON carried in Todo SSE `data`
without reimplementing the SSE parser:

```ts
import type { ServerSentEvent, StreamAdapter } from "./generated/api";

const todoAdapter: StreamAdapter<ServerSentEvent, TodoEvent> = {
  async *decode(events) {
    for await (const event of events) {
      if (event.event !== "todo") continue;
      yield JSON.parse(event.data) as TodoEvent;
    }
  },
  async *encode(items) {
    for await (const item of items) {
      yield { event: "todo", data: JSON.stringify(item) };
    }
  },
};

const api = createClient({
  baseURL,
  streamCodecs: {
    "text/event-stream": { adapter: todoAdapter },
  },
});
```

A request can override the client media-type default with
[`streamCodec`](../reference/streaming.md#requestoptions-streamcodec).
Adapter output is validated and projected through the operation's declared
`itemSchema`.

### Define custom framing

Use [`StreamProtocol<Frame>`](../reference/streaming.md#streamprotocol) when a
custom sequential media type needs its own byte framing. A
[`StreamCodec`](../reference/streaming.md#streamcodec) can combine a custom
protocol with an optional adapter.

The protocol receives a bounded `StreamReader` and
`StreamContext.maxFrameBytes`, so custom framing follows the same cancellation
and frame-size contract as built-in protocols.

Stopping iteration, calling `abort()`, cancelling `toReadableStream()`, an
external `AbortSignal`, or a timeout releases the underlying body. Generated
clients do not automatically reconnect or replay Server-Sent Events. See
[Streaming API](../reference/streaming.md) for the lifecycle and configuration
reference.

### Integration examples

Provider or framework integrations belong in the Examples section rather than
the transport contract. See
[AI streaming API with a generated client](../examples/ai-streaming.md) for a
server application that uses the AI SDK and a separate consumer application
that uses the generated client.
