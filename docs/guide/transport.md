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
require `securityRequirement` so the application chooses which alternative it
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

Use `securityProvider` when credentials are acquired dynamically for the
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

For browser-managed cookie authentication, configure Fetch credentials:

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

Headers declared as OpenAPI parameters are generated under `headerParams`.

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

A transport supplies a Fetch-compatible function and its supported capabilities.

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

Request options accept an `AbortSignal` and a timeout:

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

OpenAPI 3.2 sequential media use the same operation-centric API as other calls.
An operation with `itemSchema` exposes `.stream()`, which returns
`OperationStream<T>`. Built-in framing covers Server-Sent Events, NDJSON/JSON
Lines, JSON Sequence, and streaming multipart.

Incremental request bodies accept `StreamSource<T>`, so callers can provide an
`AsyncIterable<T>` or a Web `ReadableStream<T>`. A sequential media type with
`schema` accepts its complete application value; when both `schema` and
`itemSchema` are present, both complete and incremental request modes are
available.

Use `maxStreamFrameBytes` on the client or one request to bound a wire frame,
record, or multipart part before application adaptation.

### Adapt a built-in protocol

`StreamAdapter<Frame, Item>` handles application semantics layered on standard
framing. For example, an application can map Todo SSE data without reimplementing
the SSE parser:

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

A request can override that client default with `streamCodec`. Adapter output is
then validated and projected through the operation's declared `itemSchema`.

### Define custom framing

`StreamProtocol<Frame>` owns byte framing for a custom sequential media type.
Its bounded `StreamReader` and `StreamContext.maxFrameBytes` keep framing under
the same cancellation and size limits as built-in protocols. A `StreamCodec`
can combine a custom protocol with an optional adapter.

Stopping iteration, calling `abort()`, cancelling `toReadableStream()`, an
external `AbortSignal`, or a timeout releases the underlying body. Generated
clients do not automatically reconnect or replay Server-Sent Events.

See [Use the generated client](./client.md#consume-streaming-responses) for a
Todo stream example.
