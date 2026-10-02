# Generated client API

The TypeScript SDK provides import paths for different tasks. Most applications
use `./generated/api`.

| Import path | Use it for |
| --- | --- |
| `./generated/api` | API calls, generated types, errors, Links, and streams |
| `./generated/api/clients/<name>/index.js` | A configured client's selected APIs and types (next release) |
| `./generated/api/metadata` | OpenAPI version and optional source document |

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

Each [named client](../guide/named-clients.md) also exports a synchronous
`createClient(options)` with the same `ClientOptions`. Its return type and
available methods reflect that client's selection. Create separate instances
for separate URL, authentication, and request settings.

### ClientOptions

| Option | Purpose |
| --- | --- |
| `baseURL` | explicit absolute API base URL |
| `origin` | origin used to resolve a relative OpenAPI Server URL |
| `server` | generated OpenAPI Server selection |
| `transport` | host transport with explicit capabilities |
| `fetch` | Fetch implementation or wrapper |
| `headers` | default request headers |
| `authorization` | default complete Authorization header value |
| `credentials` | default Fetch credentials mode |
| `securityProvider` | dynamic credential acquisition for the selected OpenAPI security requirement |
| `timeoutMS` | default request timeout |
| `codecs` | complete-value codecs for declared custom media types |
| `streamCodecs` | media-type defaults for sequential protocol/adapters; see [Streaming API](./streaming.md#clientoptions-streamcodecs) |
| `maxStreamFrameBytes` | default sequential frame limit; see [Streaming API](./streaming.md#maxstreamframebytes) |

## Request options

Generated calls accept per-request options where applicable.

| Option | Purpose |
| --- | --- |
| `baseURL` | override the API base URL for one call |
| `accept` | select one declared response media type |
| `headers` | additional non-contract-owned headers |
| `authorization` | Authorization header overriding the client default |
| `credentials` | Fetch credentials mode for one call |
| `csrfToken` | value for the generated `X-CSRF-Token` header |
| `requestID` | value for the generated `X-Request-Id` header |
| `signal` | caller-owned cancellation signal |
| `timeoutMS` | request timeout overriding the client default |
| `multipartHeaders` | declared additional multipart-part headers |
| `multipartContentTypes` | selected multipart-part media types |
| `streamCodec` | one-call sequential protocol/adapter override; see [Streaming API](./streaming.md#requestoptions-streamcodec) |
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

When multiple operations share one path selector, the resource method is retained
when their public selector input type is the same. Operation-specific schema
constraints and path serialization remain attached to each terminal operation;
binding the resource value does not merge or weaken those contracts. If selector
types are incompatible, the resource shortcut is omitted, exact `$operations` /
`$routes` calls remain available, and generation reports `SDKGEN-W513`.

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

External `operationRef` targets can generate helpers when a `$ref` has already
mounted the target operation in the compiled document closure with its original
path template. Resolution uses
the Link's source document and the target's exact source pointer. The target's
operation/path/document server (or an explicit Link server) supplies its base
URL. Relative inherited servers resolve against the target document's HTTP URL.
Unresolved targets, relocated paths, and targets mounted more than once produce capability-scoped
`SDKGEN-W509`; the source response and sibling helpers remain available. Link
resolution performs no additional fetch and keeps the compiler's allowlist,
lock, and offline cache policy.

This is a trust boundary: declaring a Link grants no permission to load another
document. For example, `operationRef: ./target.json#/paths/~1items/get` can resolve
when a Path Item `$ref: ./target.json#/paths/~1items` has already loaded and mounted
`GET /items` at `/items`. If only the Link names `target.json`, the generator does
not fetch it, even when its origin is allowlisted. It emits `SDKGEN-W509` and
omits that helper. Make the target part of the declared `$ref` closure to enable
it; a network allowlist alone does not satisfy this condition.

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

Every SDK exports the input document's OpenAPI version. API descriptions and
generated types remain available with the default generation settings.

```ts
import { openapi } from "./generated/api/metadata";

openapi.version;
openapi.versionLine;
```

To read the original document from the SDK, enable
[`--with metadata`](./cli.md#metadata-addon). This is useful for documentation
tools or scripts that inspect the source description and extensions.

```ts
import { openapi } from "./generated/api/metadata";

console.log(openapi.document.info.title);
```

The export contains the whole decoded JSON or YAML entry document, including APIs
excluded by selection. External `$ref` values keep their original paths. YAML
comments and formatting belong to the source file.

### Regeneration migration {#metadata-migration}

In the next major release, SDK regeneration includes the source document when
`--with metadata` is enabled. If your code reads `openapi.document`, add this
option or `addons = ["metadata"]` to your configuration before regenerating.
Existing generated SDKs retain their exports.
