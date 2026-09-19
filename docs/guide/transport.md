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

openapi-sdkgen supports OpenAPI 3.0.x, 3.1.x, and 3.2.x. For known sequential
content types, a normal `schema` can describe the complete buffered value on
all supported version lines. OpenAPI 3.2 adds the Media Type Object
[`itemSchema`](https://spec.openapis.org/oas/v3.2.0.html#media-type-object),
which enables typed incremental calls through
[`.stream()`](../reference/client-api.md#links-and-streams).

An operation with `itemSchema` exposes `.stream()`, which returns
[`OperationStream<T>`](../reference/typescript-types.md#stream-types).
Built-in framing covers Server-Sent Events, NDJSON/JSON Lines, JSON Sequence,
and streaming multipart. The exact version split is listed in
[OpenAPI support](../reference/capabilities.md#supported-openapi-versions).

Incremental request bodies accept
[`StreamSource<T>`](../reference/typescript-types.md#stream-types),
so callers can provide an `AsyncIterable<T>` or a Web `ReadableStream<T>`.
A sequential media type with `schema` accepts its complete application value;
when both `schema` and `itemSchema` are present, both complete and incremental
request modes are available.

Use
[`maxStreamFrameBytes`](../reference/client-api.md#links-and-streams) on the
client or one request to bound a wire frame, record, or multipart part before
application adaptation.

### Adapt a built-in protocol

[`StreamAdapter<Frame, Item>`](../reference/typescript-types.md#stream-types)
handles application semantics layered on standard framing. For example, an
application can map Todo SSE data without reimplementing the SSE parser:

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

### Bridge an AI event stream to AI SDK UI

AI APIs often layer application events such as text deltas or tool-input deltas
over SSE. Keep that provider or application protocol in a
[`StreamAdapter`](../reference/typescript-types.md#stream-types),
then bridge the typed generated stream at the application boundary.

This example maps JSON-in-SSE to the `itemSchema` type generated for an
operation named `generate`, then forwards text deltas into the AI SDK UI message
stream protocol:

```ts
import {
  createUIMessageStream,
  createUIMessageStreamResponse,
} from "ai";
import {
  createClient,
  type OperationStreamItem,
  type ServerSentEvent,
  type StreamAdapter,
} from "./generated/api";

type AiEvent = OperationStreamItem<"generate">;

const aiAdapter: StreamAdapter<ServerSentEvent, AiEvent> = {
  async *decode(events) {
    for await (const event of events) {
      if (event.data === "[DONE]") return;
      yield JSON.parse(event.data) as AiEvent;
    }
  },
  async *encode(items) {
    for await (const item of items) {
      yield { data: JSON.stringify(item) };
    }
  },
};

const api = createClient({
  baseURL,
  streamCodecs: {
    "text/event-stream": { adapter: aiAdapter },
  },
});

export async function POST() {
  const upstream = api.$operations.generate.stream({
    body: { prompt: "Summarize the release notes." },
  });

  const stream = createUIMessageStream({
    async execute({ writer }) {
      const id = "answer";
      writer.write({ type: "text-start", id });

      for await (const event of upstream) {
        if (event.type === "text-delta") {
          writer.write({ type: "text-delta", id, delta: event.text });
        }
      }

      writer.write({ type: "text-end", id });
    },
    onError: () => "Upstream generation failed",
  });

  return createUIMessageStreamResponse({ stream });
}
```

Tool calls, reasoning, sources, and custom data remain application-level mapping
decisions. openapi-sdkgen only owns HTTP framing, adapter composition, generated
types, validation, and lifecycle. See the AI SDK references for
[`createUIMessageStream`](https://ai-sdk.dev/docs/reference/ai-sdk-ui/create-ui-message-stream)
and
[`createUIMessageStreamResponse`](https://ai-sdk.dev/docs/reference/ai-sdk-ui/create-ui-message-stream-response).

The Playground includes an **AI event stream** OpenAPI 3.2 example. Open it
directly at [Playground → AI event stream](../playground.md?example=ai-event-stream).

### Define custom framing

[`StreamProtocol<Frame>`](../reference/typescript-types.md#stream-types)
owns byte framing for a custom sequential media type. Its bounded `StreamReader`
and `StreamContext.maxFrameBytes` keep framing under the same cancellation and
size limits as built-in protocols. A
[`StreamCodec`](../reference/typescript-types.md#stream-types) can
combine a custom protocol with an optional adapter.

Stopping iteration, calling `abort()`, cancelling `toReadableStream()`, an
external `AbortSignal`, or a timeout releases the underlying body. Generated
clients do not automatically reconnect or replay Server-Sent Events.

See [Use the generated client](./client.md#consume-streaming-responses) for a
Todo stream example.
