# Generated client API

The TypeScript SDK provides import paths for different tasks. Most applications
use `./generated/api`.

| Import path | Use it for |
| --- | --- |
| `./generated/api` | API calls, generated types, errors, Links, and streams |
| `./generated/api/metadata` | Reading the source OpenAPI file and version |
| `./generated/api/server/webhooks` | Handling Webhooks; generated with `--with server` |
| `./generated/api/server/callbacks` | Handling Callbacks; generated with `--with server` |

::: details Running directly in Node ESM

Use an explicit `.js` path when running compiled files with Node ESM.

```ts
import { createClient } from "./generated/api/index.js";
```
:::

## Client

```ts
import { createClient } from "./generated/api";

const api = createClient({
  baseURL: "https://api.example.test/v1",
});
```

See [transport, authentication, and streams](../guide/transport.md) for client
configuration.

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
sent. See [Request headers](../guide/transport.md#request-headers).

## Links and streams

- `$links`: follow-up requests defined by OpenAPI Links
- `.stream(...)`: typed streaming capability on generated operation, route, and resource calls
- `OperationStream<T>`: lazy single-consumer stream handle with `response`, `abort()`, and `toReadableStream()`
- `StreamSource<T>`: `AsyncIterable<T> | ReadableStream<T>` for incremental request bodies
- `RouteStreamItem<Route>`: extracts the item type for an exact route
- `OperationStreamItem<Source>`: extracts the item type from an operation ID or generated operation method
- `ServerSentEvent`: standard SSE value with string `data` and optional `event`, `id`, and `retry`

Use `ClientOptions.streamCodecs` for media-type defaults and
`RequestOptions.streamCodec` for one-call protocol/adapter overrides.
`maxStreamFrameBytes` bounds one wire frame before adaptation.

See [Use the generated client](../guide/client.md) and
[Authentication, transport, and streams](../guide/transport.md) for examples.

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

## Webhooks and Callbacks

When generation includes `--with server`, use these imports:

```ts
import { createWebhookRouter } from "./generated/api/server/webhooks";
import { createCallbackHandlers } from "./generated/api/server/callbacks";
```

See [Receive Webhooks and Callbacks](../guide/server.md) for setup and examples.
