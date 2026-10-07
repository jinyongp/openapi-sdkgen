# Generate a client for each feature

When different pages use different parts of an API, assign each page's APIs to a
named client. Each client gets its own import path and exposes only its assigned
API methods and types.

## Assign APIs to clients

Try the [named-client example document](/examples/named-clients.json) in a
separate project. Save it as `openapi.yaml` and use the installation and ESM
setup from Getting started. JSON content is accepted with this filename.

This example assumes an OpenAPI document with `listOrders`, `createOrder`, and
`GET /products/{id}`. All clients use the same input document and generation
settings. Each client currently accepts a `selection` containing operation IDs,
routes, or both.

```toml
source = "./openapi.yaml"
target = "typescript"
output = "./src/generated/api"

[clients.orders.selection]
operations = ["listOrders", "createOrder"]

[clients.catalog.selection]
routes = ["GET /products/{id}"]
```

Save this as `openapi-sdkgen.toml` and generate the SDK:

```sh
openapi-sdkgen generate --config ./openapi-sdkgen.toml
```

The client entry points are `src/generated/api/clients/orders/index.ts` and
`src/generated/api/clients/catalog/index.ts`. Use [Find APIs](./inspect.md) to
look up IDs and routes. The selection uses exact names from the document,
including `{id}`; supply the actual ID when calling the generated method.

Client names start with a lowercase letter and contain lowercase ASCII letters,
digits, or hyphens, up to 64 characters. Windows device names such as `con` are
reserved. Each client needs a nonempty selection. Unknown or hidden APIs and
unrecognized client properties produce an error.

## Import the client where it is used

For a document whose `GET /products/{id}` resource method is `get`, the catalog
page can use its client directly:

```ts
import { createClient } from "./generated/api/clients/catalog/index.js";

const api = createClient({ baseURL: "https://api.example.test/v1" });
const product = await api.products("product-1").get();
```

`createClient` is synchronous. Its resource tree, `$routes`, and `$operations`
contain the selected APIs. Resource method names follow the document's naming
rules; use [API inspection](./inspect.md#typescript) to check the generated call
paths. Every instance accepts the normal [client options](../reference/client-api.md#clientoptions)
and has its own URL, authentication, Fetch implementation, and request state.

To load the page's client on demand, use a dynamic import:

```ts
const { createClient } = await import("./generated/api/clients/catalog/index.js");
const api = createClient({ baseURL: "https://api.example.test/v1" });
const product = await api.$routes["GET /products/{id}"]({
  path: { id: "product-1" },
});
```

Importing the entry prepares its selected code; the method call sends the API
request. You can use all clients across an application while loading each
page's entry separately. Shared dependencies remain shared. A client imports
the models and input/output representations its APIs need, so a model used
exclusively by another client stays outside its initial dependencies. Large
models can still have large dependency graphs. Bundler settings that deliberately
combine pages into one chunk also combine their dependencies.

## Root SDK and generation scope

See [Choose a selection method](./selection-benchmarks.md) to compare named
clients, root selection, and `loadOperations`.

Without a root selection, the root SDK exposes the union of the named clients' selections. Assign each API once in its named client configuration:

| Configuration | Result |
| --- | --- |
| `[clients.orders.selection]` | APIs exposed by `clients/orders` |
| `[selection]` | APIs exposed by the root SDK and its selective entry |
| Named clients without `[selection]` | The root SDK exposes their selected API union |
| Neither named clients nor `[selection]` | The root SDK exposes the full document |

Generation includes APIs selected by the root or named clients and their required
Link dependencies. An ordinary `[selection]` without named clients follows the
same generation rule.

For example, add `[selection]` with `operations = ["listOrders"]` if the root SDK
should expose only order listing. The catalog client still exposes its selected
product API. CLI `--operation` and `--route` overrides apply to the root selection.

Use named clients when assignments belong in generation configuration. Use
[`loadOperations`](./selective-client.md#prepare) when application code needs to
choose APIs at runtime. Named entries prepare their fixed selection directly.

## Links, types, and updates

OpenAPI Link helpers keep their target APIs usable. A target selected only as a
Link dependency loads when the helper is invoked and uses the source client's
configuration. Select the target explicitly in that client to expose it as a
normal method. See [Link support](../reference/capabilities.md) for reference
requirements.

Import each client's `Client`, `ComponentInput`, and `ComponentOutput` types from
its own entry; see [named client types](../reference/typescript-types.md#named-clients).
With `addons = ["server"]`, the shared server includes directly selected APIs
and their callbacks from the root and all clients, plus top-level webhooks.

After changing a client selection or name, regenerate with `--incremental`.
Managed entries are updated together, and your own files are preserved.
`--check --output` verifies that the generated SDK matches the configuration.
