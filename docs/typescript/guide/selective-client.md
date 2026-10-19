# Load only the operations you use

The generated SDK includes a selective entry alongside the regular full
client. Use it when an application needs a small part of a larger API: preparing
an operation loads its implementation and required runtime code, rather than the
full client registry. By default the generator produces every operation and all
of its TypeScript types, so changing the application's selection does not
require regenerating the SDK. To reduce the generated files themselves, choose
the APIs during generation as described below.

<span id="runtime-features-follow-the-generated-apis"></span>

## Generate only the APIs you need {#generation}

To try the examples on this page, save the
[Task example document](/examples/task-selection.json) as `openapi.yaml` in a
project.
JSON is accepted regardless of the filename extension.

To assign different API sets to separate import paths, use
[clients for each feature](/typescript/guide/named-clients). The selection below controls the
root SDK and its selective entry.

Use [Find APIs](/typescript/guide/inspect) to browse operation IDs and routes and export a
selection for this configuration.

For a large API, specify the operation IDs or routes your application uses in
its generation configuration:

```toml
source = "./openapi.yaml"
target = "typescript"
output = "./src/generated/api"

[selection]
operations = ["listTasks"]
routes = ["GET /health"]
```

Run `openapi-sdkgen generate --config ./openapi-sdkgen.toml` with this file.
The resulting SDK exposes the union of both lists and includes their required
types and runtime code. The regular client and selective entry share this
generated API set; `loadOperations` can prepare APIs within that set.

Names match exactly. Use the original `operationId`, or the method and path
template from the document, such as `GET /tasks/{task-id}`. A route uses
`{task-id}` as written in OpenAPI; the value is supplied later through
`path: { "task-id": "one" }` when calling the API. An API without an operation
ID can be selected by route. Duplicate names select an API once.

Without `[selection]` or named clients, generation includes the full API. With
named clients alone, the root exposes their selected API union. An empty selection, unknown
name, or hidden API produces an error. Document errors, including duplicate
operation IDs, must be corrected before generating a subset.

OpenAPI Links remain usable: their targets and required types are included as
internal dependencies. Select a target explicitly to call it directly or
prepare it with `loadOperations`. A Link target must be available through the
document's loaded references; see [Link support](/typescript/reference/capabilities).

With `--with server`, callbacks belonging to the selected APIs and all top-level
webhooks are generated. To change the selected set in an existing SDK, use
`--incremental`; the update replaces owned files, removes obsolete generated
runtime files, and preserves your own files. An edited generated file blocks the
update instead of being overwritten. `--check --output` verifies the existing SDK
against the requested set without changing it.

The CLI also accepts repeatable `--operation` and `--route` flags. See the
[CLI reference](/typescript/reference/cli#api-selection) for overrides and syntax.

The examples below assume a document with `GET /tasks` (`operationId: listTasks`, optional integer query `limit`)
and `GET /health` (no operation ID). Generate the SDK as described in
[Generate and verify](/typescript/guide/generate). Compile the generated TypeScript to ESM
JavaScript before serving it directly to a browser.
Include all generated `.ts` files in your compiler input, for example with
`"include": ["src/**/*.ts"]` in `tsconfig.json`. Namespace lookups load modules
dynamically, so compiling only a consumer entry does not emit the complete tree.

## Prepare code, then configure a client {#prepare}

See [Choose a selection method](/typescript/guide/selection-cost) to compare generation,
static imports, and dynamic lookups.

```ts
import {
  createClient,
  loadOperations,
  operations,
  routes,
} from "./generated/api/selective/index.js";

const prepared = await loadOperations([
  operations.listTasks,
  routes["GET /health"],
]);

const api = createClient({
  baseURL: "https://api.example.test/v1",
  operations: prepared,
});

const tasks = await api.$operations.listTasks({ query: { limit: 20 } });
console.log(tasks);
const health = await api.$routes["GET /health"]();
```

Example server responses: `tasks` is `["Write documentation", "Ship the SDK"]`, and `GET /health` returns the string `"ok"`. The Task contract returns a string array directly, unlike the Todo contract's `{ items: [...] }` wrapper.

`operations` uses exact OpenAPI operation IDs. `routes` uses the exact HTTP
method and OpenAPI path template, including operations without an ID. Selecting
an operation through both names registers it once. A local variable or export
name does not rename its generated client method.

`loadOperations` prepares code; it does not send an API request. `createClient`
is synchronous and supplies the base URL, Fetch implementation, authentication,
and other [client options](/typescript/reference/client-api#clientoptions). One prepared
value can be reused by separately configured clients without sharing their
credentials. Prepare an empty array to create an empty selective client.

Use the selective entry's `createClient`, not the regular root entry's factory.
The root factory remains the full client and does not infer a selection from
which methods application code later calls.

## Compose features without repeating names {#features}

Keep each feature's selection next to its application code:

```ts
// tasks.operations.ts
import { operations } from "./generated/api/selective/index.js";

export default [operations.listTasks] as const;
```

```ts
import {
  createClient,
  loadOperations,
  routes,
} from "./generated/api/selective/index.js";
import taskOperations from "./tasks.operations.js";

const api = createClient({
  baseURL: "https://api.example.test/v1",
  operations: await loadOperations([taskOperations, routes["GET /health"]]),
});
const tasks = await api.$operations.listTasks({ query: { limit: 20 } });
```

`tasks` is the same string array as in the first call; composing feature selections does not wrap or transform the API response.

Pass nested arrays and groups directly. Await asynchronously loaded selections first.
See [selective client inputs](../reference/client-api.md#selection-input) for accepted
values and cyclic-container handling.

## Preserve guaranteed and computed membership {#types}

Inline tuples and feature arrays declared `as const` preserve which operations
are definitely present. Their methods remain required, with the original input,
output, error, `.raw()`, pagination, and stream types. The corresponding resource
methods are included only when their operations are selected, using the same
resource naming and collision rules as the full client.

Computed selections cannot guarantee a particular operation, so their methods are optional.
See [computed selection types](../reference/client-api.md#computed-selection) for an example.

## Enumerate routes explicitly {#enumeration}

Use `selective/all.js` when you need to enumerate or filter the route names.
See [route enumeration](../reference/client-api.md#selection-enumeration) for imports and calls.

## Use static references with a bundler {#bundlers}

For a bundled application, import generated operation modules directly so the
bundler can see their dependency edges:

```ts
// tasks.operations.ts
import { operation } from "./generated/api/selective/operations/tasks/get.js";

export default [operation] as const;
```

Pass this array to the same `loadOperations` and `createClient` functions. The
module exports one reference named `operation`; the generated operation ID still
determines the client method. Use the actual generated path, particularly when
OpenAPI paths require escaping or have naming collisions.

With these static imports, Vite includes the selected generated modules in the
application bundle.

The default namespace-based references compute lookup URLs at runtime. A bundler
must not be assumed to discover and copy that entire lookup tree automatically.
Either use static references, or preserve the generated native ESM tree as an
external asset tree. A bundler's chunking and preload settings affect when code
is downloaded; inspect the resulting application rather than assuming every
configuration has the same lazy-loading behavior.

## Follow Links and open streams {#helpers}

Prepared operations retain their existing helper APIs. In the native ESM path,
an OpenAPI Link's target code is loaded when the helper is invoked, not merely
because the source operation was prepared. The first invocation can therefore
incur a module download or a module-loading error. It uses the same client's
request configuration. A target needed only by a Link does not silently become
an entry in that client's public operation or route maps.

An already prepared target is reused. A selected operation's synchronous
`.stream()` surface is ready after preparation and still returns an
[OperationStream](/typescript/reference/streaming#operationstream), not a promise of a
stream. [Request cancellation and timeouts](/typescript/guide/transport) continue to apply to
requests; they do not promise cancellation of the browser's native module
network request.

## Serve a consistent generated tree {#deployment}

For native ESM, publish the complete compiled generated tree, preserving relative
paths and `.js` extensions. Code is resolved relative to the generated module,
not relative to the API's `baseURL`. Use HTTPS in production: namespace lookup
uses Web Crypto. Serve JavaScript with the correct MIME type and permit its
origins under your script policy; cross-origin modules also need appropriate
CORS responses.

Deploy each generated revision under a versioned asset URL and retain the old
revision while existing pages can still use it. Do not replace individual files
in place with a different generated revision.

For preparation failures and `OperationPreparationError.stage`, see
[input and preparation errors](../reference/client-api.md#selection-input).

Measure initial preparation, subsequent feature preparation, the first Link
invocation, and repeat visits separately. Shared code is reused, but a selection
containing most operations can cost more than the full client because of lookup
and module overhead. Keep the regular full entry when that better fits the
application's measured workload.
