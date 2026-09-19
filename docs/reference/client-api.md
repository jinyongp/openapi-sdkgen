# Generated client API

The TypeScript SDK provides import paths for different tasks. Most applications
use `./generated/api`.

| Import path | Use it for |
| --- | --- |
| `./generated/api` | API calls, generated types, errors, Links, and streams |
| `./generated/api/metadata` | Reading the source OpenAPI file and version |

For inbound Webhook and Callback imports, see
[Generated server API](./server-api.md).

::: details Running directly in Node ESM

Use an explicit `.js` path when running compiled files with Node ESM.

```ts
import { createClient } from "./generated/api/index.js";
```
:::

## Client

### createClient

```ts
import { createClient } from "./generated/api";

const api = createClient({
  baseURL: "https://api.example.test/v1",
});
```

See [transport, authentication, and streams](../guide/transport.md) for guided
configuration examples.

### ClientOptions

| Option | Purpose |
| --- | --- |
| `baseURL` | explicit absolute API base URL |
| `origin` | origin used to resolve a relative OpenAPI Server URL |
| `server` | generated OpenAPI Server selection |
| `codecs` | complete-value codecs for declared custom media types |
| `streamCodecs` | media-type defaults for sequential protocol/adapters; see [Streaming API](./streaming.md#clientoptions-streamcodecs) |
| `transport` | host transport with explicit capabilities |
| `fetch` | Fetch implementation or wrapper |
| `headers` | default request headers |
| `authorization` | default complete Authorization header value |
| `credentials` | default Fetch credentials mode |
| `securityProvider` | dynamic credential acquisition for the selected OpenAPI security requirement |
| `timeoutMS` | default request timeout |
| `maxStreamFrameBytes` | default sequential frame limit; see [Streaming API](./streaming.md#maxstreamframebytes) |

## Request options

Generated calls accept per-request options where applicable.

| Option | Purpose |
| --- | --- |
| `baseURL` | override the API base URL for one call |
| `signal` | caller-owned cancellation signal |
| `timeoutMS` | request timeout overriding the client default |
| `headers` | additional non-contract-owned headers |
| `authorization` | Authorization header overriding the client default |
| `accept` | select one declared response media type |
| `streamCodec` | one-call sequential protocol/adapter override; see [Streaming API](./streaming.md#requestoptions-streamcodec) |
| `csrfToken` | value for the generated `X-CSRF-Token` header |
| `requestID` | value for the generated `X-Request-Id` header |
| `credentials` | Fetch credentials mode for one call |
| `multipartHeaders` | declared additional multipart-part headers |
| `multipartContentTypes` | selected multipart-part media types |
| `maxStreamFrameBytes` | one-call sequential frame limit |

Operation-specific input sections such as `path`, `query`, `headerParams`, and
`body` are generated from the OpenAPI operation rather than from
`RequestOptions`.

## TypeScript types

The generated SDK exposes component-, route-, operation-, request-section-, and
parameter-based type helpers. See
[Generated TypeScript types](./typescript-types.md) for the complete type API and
examples.

## Call an API

### Resource methods

Use path-based resource methods for normal application code.

```ts
const todo = await api.todos.create({
  body: { title: "Write documentation" },
});
```

### `$routes`

Call an API by its HTTP method and OpenAPI path. This also works when no
`operationId` is declared.

```ts
const todos = await api.$routes["GET /todos"]({
  query: { limit: 20 },
});
```

### `$operations`

Call an API by its declared `operationId`.

```ts
const todos = await api.$operations["listTodos"]({
  query: { limit: 20 },
});
```

### `.raw()`

Every generated operation call also exposes `.raw()`. It returns the decoded
body together with status, response headers, request metadata, selected content
type, and the original Fetch `Response`.

```ts
const result = await api.$operations.getTodo.raw({
  path: { todoID: "todo-1" },
});

result.status;
result.headers;
result.response;
```

The Fetch body is normally already consumed by decoded calls. For declared
streaming responses, a separate `.raw()` request preserves the unconsumed body.

## Security requirements

When an operation has several OpenAPI security alternatives, the generated
request options require `securityRequirement`. With one requirement, the SDK
selects it automatically. An empty requirement uses the ID `"anonymous"` when
it participates in a choice.

For a Todo operation that accepts `userAuth` or `serviceAuth`:

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

The generated type exposes the valid requirement IDs. Use `securityProvider`
when credentials need to be acquired dynamically. See
[Authentication](../guide/transport.md#provide-ordinary-bearer-credentials) for
the full security model and examples.

## Request headers

Every declared header appears under `headerParams`. Headers controlled by Fetch are
optional caller inputs, and the active Fetch implementation decides whether they are
sent. See [Request headers](../guide/transport.md#pass-declared-request-headers).

## Links

`$links` contains typed follow-up calls generated from OpenAPI Link Objects.
Each helper carries the source response context needed to resolve Link runtime
expressions.

See [Follow OpenAPI Links](../guide/client.md#follow-openapi-links) for a
complete example.

## Streaming

Generated sequential-media APIs, lifecycle, protocol/adapter extension points,
request sources, and frame limits are documented in the dedicated
[Streaming API](./streaming.md) reference.

## Errors

```ts
import {
  isAPIError,
  isErrorCategory,
  isErrorCode,
  TransportErrorCode,
} from "./generated/api";
```

- `isAPIError(error)`: checks for any generated API error
- `isErrorCode(error, code)`: checks an exact error code
- `isErrorCategory(error, category)`: checks an error category
- `TransportErrorCode`: lists errors raised while sending or receiving a request

Security selection uses `SECURITY_REQUIREMENT_REQUIRED` and
`SECURITY_REQUIREMENT_INVALID`. Credential acquisition and application use
`SECURITY_CREDENTIALS_REQUIRED` and `SECURITY_CREDENTIALS_INVALID`.

## OpenAPI metadata

```ts
import { openapi } from "./generated/api/metadata";

openapi.document;
openapi.version;
openapi.versionLine;
```

`openapi.document` contains the OpenAPI content used to generate the SDK.
