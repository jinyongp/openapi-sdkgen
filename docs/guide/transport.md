# Authentication, transport, and streams

The generated client uses Fetch as its transport boundary. Most applications configure a base URL and credentials. Configure a custom transport for runtime-specific capabilities such as a cookie jar,
access to restricted response headers, or mutual TLS.

The ordinary authorization, cookie, and cancellation examples use the
[Getting started](./getting-started.md) contract. Import `createClient` from
`./generated/api/index.js`. Security alternatives and declared headers require
the contract additions shown in their sections.

Each contract addition is an independent variation of the original starter
document. Use your actual token and server address for network calls.


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

`authorization` and `headers.Authorization` are ordinary request defaults. They
are sent even when an operation declares `security: []`, and are not restricted
to the configured base URL's origin. An operation server can use another origin.
Use separate clients or `securityProvider` when credentials depend on the
operation's declared security or final origin.

## Choose among OpenAPI security alternatives

OpenAPI can declare several Security Requirement Objects for one operation. If
more than one effective requirement is available, the generated request options
require [`securityRequirement`](../reference/client-api.md#security-requirements) so the application chooses which alternative it
is satisfying.

For `createTodo`, add these fields to the starter contract and regenerate with
`--incremental`. Keep its existing request and response schemas:

```yaml
# Merge securitySchemes into components, and security into /todos POST.
components:
  securitySchemes:
    userAuth:
      type: http
      scheme: bearer
    serviceAuth:
      type: apiKey
      in: header
      name: x-service-token
paths:
  /todos:
    post:
      security:
        - userAuth: []
        - serviceAuth: []
```

```ts
await api.$operations.createTodo(
  {
    body: { title: "Write documentation" },
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

`getTodoServiceToken` and `getTodoUserToken` are application-owned token lookup
functions that you must supply. This example uses the two schemes above.

```ts
const api = createClient({
  baseURL: "https://api.example.test",
  securityProvider: async ({ operation, requirement, origin }) => {
    if (origin !== "https://api.example.test") {
      throw new Error("Untrusted API origin");
    }
    if (requirement.id === "serviceAuth") {
      return {
        [requirement.id]: {
          kind: "api-key",
          value: await getTodoServiceToken(operation, origin),
        },
      };
    }

    return {
      [requirement.id]: {
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

The provider runs when the selected non-empty security requirement is not already
satisfied by available credentials. Anonymous operations and requirements already
satisfied by client or request credentials skip it. Decide which origins may
receive provider credentials inside the provider. Its origin check does not apply
to credentials supplied through other options or headers.

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

Add this `parameters` field to `/todos` → `post` in the starter contract and regenerate:

```yaml
parameters:
  - name: Idempotency-Key
    in: header
    required: true
    schema:
      type: string
```

```ts
await api.$operations.createTodo({
  headerParams: { "Idempotency-Key": "request-1" },
  body: {
    title: "Write documentation",
  },
});
```

The active Fetch environment controls headers such as `Origin`, `Host`, `Cookie`,
and `Sec-*`, including whether caller-provided values can be applied.

## Configure a custom transport

[`ClientOptions.transport`](../reference/client-api.md#clientoptions) supplies a Fetch-compatible function and its supported capabilities.

```ts
import { createClient } from "./generated/api/index.js";

async function loggingFetch(input: RequestInfo | URL, init?: RequestInit) {
  const response = await fetch(input, init);
  console.log(response.status);
  return response;
}

const api = createClient({
  baseURL: "https://api.example.test",
  transport: { fetch: loggingFetch },
});
```

`capabilities.cookieJar`, `readableResponseHeaders`, and `mutualTLS` describe
features already implemented by the transport. Setting them does not install a
cookie jar or configure client certificates. Configure those features in the
host transport first.


## Cancel a request or set a timeout

[Request options](../reference/client-api.md#request-options) accept an `AbortSignal` and a timeout:

```ts
const controller = new AbortController();

const todos = await api.todos.list(
  {},
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

Built-in SSE preserves `data` as a string. Use the
[stream adapter example](../reference/streaming.md#streamadapter) to convert JSON
or select named events. Set `streamCodec` for one request or `streamCodecs`
for the entire client.

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
