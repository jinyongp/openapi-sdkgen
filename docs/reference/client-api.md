# Generated client API

Code on this page illustrates API shapes. Operation names, parameters, media,
and security come from your own contract. The small Todo contract in
[Getting started](../guide/getting-started.md) does not include every feature below.

The TypeScript SDK provides import paths for different tasks. Most applications
use `./generated/api`.

| Import path | Use it for |
| --- | --- |
| `./generated/api` | API calls, generated types, errors, Links, and streams |
| `./generated/api/clients/<name>/index.js` | A configured client's selected APIs and types |
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

If a naming or selector-type conflict prevents a resource method, generation
reports `SDKGEN-W513`. Call the operation through `$operations` or `$routes`.

### `$routes`

Call an API by its HTTP method and OpenAPI path. This also works when no
`operationId` is declared.

```ts
const todos = await api.$routes["GET /todos"]();
```

### `$operations`

Call an API by its declared `operationId`.

```ts
const todos = await api.$operations["listTodos"]();
```

### `.raw()`

Every generated operation call also exposes `.raw()`. It returns the decoded
body together with status, response headers, request metadata, selected content
type, and the original Fetch `Response`.

```ts
const result = await api.$operations.createTodo.raw({
  body: { title: "Write documentation" },
});

result.status;
result.headers;
result.response;
```

The Fetch body is normally already consumed by decoded calls. For declared
streaming responses, a separate `.raw()` request preserves the unconsumed body.

#### Status and media in successful responses

Raw responses use the successful status range `200`–`299`. With same-media
`200: Item` and `default: Problem`, `status === 200` narrows the body to `Item`,
while `202` can still return `Problem`. A same-media `2XX` response covers every
successful status, so `default` stays in the HTTP error contract and does not
broaden ordinary, raw, streaming, or pagination results.

Different media, wildcard ranges, and bodyless declarations remain distinct
where they can still be selected. Raw `contentType` is the normalized concrete
response header. Wildcard declarations use a string or template literal type.

## Security requirements

When an operation has several OpenAPI security alternatives, the generated
request options require `securityRequirement`. With one requirement, the SDK
selects it automatically. An empty requirement uses the ID `"anonymous"` when
it participates in a choice.

For `createTodo` extended with the `userAuth` and `serviceAuth` schemes from
the [authentication guide](../guide/transport.md):

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

The generated type exposes the valid requirement IDs. Use `securityProvider`
when credentials need to be acquired dynamically. See
[Authentication](../guide/transport.md#provide-ordinary-bearer-credentials) for
the full security model and examples.

## Request headers

Every declared header appears under `headerParams`. Headers controlled by Fetch are
optional caller inputs, and the active Fetch implementation decides whether they are
sent. See [Request headers](../guide/transport.md#pass-declared-request-headers).

## Links

`$links` contains follow-up calls generated from OpenAPI Link Objects. Pass the
source's `.raw()` result to carry its response values into the next request.
See [Follow OpenAPI Links](../guide/files-links-streams.md#follow-openapi-links).

An external `operationRef` needs its target loaded through a `$ref` and mounted
at its original API path. A Link alone does not load an external document.
For example, `operationRef: ./target.json#/paths/~1items/get` can resolve when
`$ref: ./target.json#/paths/~1items` mounts the Path Item at `/items`.
Unresolved or ambiguous targets produce `SDKGEN-W509` and omit that helper.
The target's server or an explicit Link server supplies the request URL.

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
  isOperationHTTPError,
  TransportErrorCode,
} from "./generated/api";
```

- `isAPIError(error)`: checks for any generated API error
- `isErrorCode(error, code)`: checks an exact error code
- `isErrorCategory(error, category)`: checks an error category
- `isOperationHTTPError(error, method)`: checks a real HTTP error from that
  generated method and narrows its declared status, body, and media union
- `TransportErrorCode`: lists errors raised while sending or receiving a request

Security selection uses `SECURITY_REQUIREMENT_REQUIRED` and
`SECURITY_REQUIREMENT_INVALID`. Credential acquisition and application use
`SECURITY_CREDENTIALS_REQUIRED` and `SECURITY_CREDENTIALS_INVALID`.

`isOperationHTTPError` matches declared HTTP errors from the specified operation,
including raw and streaming calls. It does not match transport failures.
Use `OperationHTTPError<typeof method>` to extract the same error type.

### Narrow declared HTTP errors {#declared-http-errors}

This example needs `getTodo` with a `todoID` path parameter and a JSON `404`
response containing a string `message`. Add it to the starter document and
regenerate before using this code.
The [extended Todo example](/examples/todo-types.json) includes that contract.

```ts
import { isOperationHTTPError } from "./generated/api";

try {
  await api.$operations.getTodo({ path: { todoID: "todo-1" } });
} catch (error: unknown) {
  if (!isOperationHTTPError(error, api.$operations.getTodo)) throw error;
  if (error.status === 404) console.log(error.data.message);
}
```

<span id="metadata-migration"></span>

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

With a root selection, the metadata module also exports `generationSelection`
(public routes and private Link dependencies). With named clients, it exports
`generationClients` (each client's assignment). Both records require
`--with metadata`.
