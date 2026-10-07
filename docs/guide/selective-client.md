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
separate project with the installation and ESM setup from Getting started.
JSON is accepted regardless of the filename extension.

To assign different API sets to separate import paths, use
[clients for each feature](./named-clients.md). The selection below controls the
root SDK and its selective entry.

Use [Find APIs](./inspect.md) to browse operation IDs and routes and export a
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
document's loaded references; see [Link support](../reference/capabilities.md).

With `--with server`, callbacks belonging to the selected APIs and all top-level
webhooks are generated. To change the selected set in an existing SDK, use
`--incremental`; the update replaces owned files, removes obsolete generated
runtime files, and preserves your own files. An edited generated file blocks the
update instead of being overwritten. `--check --output` verifies the existing SDK
against the requested set without changing it.

The CLI also accepts repeatable `--operation` and `--route` flags. See the
[CLI reference](../reference/cli.md#api-selection) for overrides and syntax.

The examples below assume a document with `GET /tasks` (`operationId: listTasks`, optional integer query `limit`)
and `GET /health` (no operation ID). Generate the SDK as described in
[Generate and verify](./generate.md). Compile the generated TypeScript to ESM
JavaScript before serving it directly to a browser.
Include all generated `.ts` files in your compiler input, for example with
`"include": ["src/**/*.ts"]` in `tsconfig.json`. Namespace lookups load modules
dynamically, so compiling only a consumer entry does not emit the complete tree.

## Prepare code, then configure a client {#prepare}

See [Choose a selection method](./selection-benchmarks.md) to compare generation,
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
await api.$routes["GET /health"]();
```

`operations` uses exact OpenAPI operation IDs. `routes` uses the exact HTTP
method and OpenAPI path template, including operations without an ID. Selecting
an operation through both names registers it once. A local variable or export
name does not rename its generated client method.

`loadOperations` prepares code; it does not send an API request. `createClient`
is synchronous and supplies the base URL, Fetch implementation, authentication,
and other [client options](../reference/client-api.md#clientoptions). One prepared
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
import tasks from "./tasks.operations.js";

const api = createClient({
  baseURL: "https://api.example.test/v1",
  operations: await loadOperations([tasks, routes["GET /health"]]),
});
await api.$operations.listTasks({ query: { limit: 20 } });
```

Nested arrays and objects are composed by value, so spreading a feature array is
unnecessary. Named array exports and module namespaces also work when their
exported values are selections. Groups use their own string-keyed properties;
array selections use their indexed elements. Getters are read normally, and the
values obtained are reused throughout that preparation. Getter effects belong
to application code, and thrown errors are preserved.

A selection must contain operation references, arrays, or groups of selections.
Do not mix unrelated metadata or functions into a feature module passed as a
selection. Await asynchronous configuration yourself before passing its result;
the loader does not invoke functions, await promises, or consume arbitrary
iterables on your behalf. Cyclic containers are invalid, while reusing the same
feature in several groups is supported.

## Preserve guaranteed and computed membership {#types}

Inline tuples and feature arrays declared `as const` preserve which operations
are definitely present. Their methods remain required, with the original input,
output, error, `.raw()`, pagination, and stream types. The corresponding resource
methods are included only when their operations are selected, using the same
resource naming and collision rules as the full client.

A runtime filter cannot guarantee that an operation is present. Its candidate
methods are optional and must be checked before use:

```ts
const candidates = [operations.listTasks, routes["GET /health"]];
const selection = candidates.filter(() => Math.random() > 0.5);
const dynamic = createClient({
  baseURL: "https://api.example.test/v1",
  operations: await loadOperations(selection),
});

await dynamic.$operations.listTasks?.({ query: { limit: 20 } });
```

Keep an array as a tuple at its declaration when its membership is fixed. Passing
an already widened array later does not restore information that TypeScript has
lost. Types do not prove the behavior of a getter or the truth of a user-written
type assertion.

## Enumerate routes explicitly {#enumeration}

The default reference objects do not carry a runtime list of every operation.
Import the opt-in names-only entry for enumeration and filtering:

```ts
import { routes as allRoutes } from "./generated/api/selective/all.js";
import { createClient, loadOperations } from "./generated/api/selective/index.js";

const taskRoutes = Object.entries(allRoutes)
  .filter(([route]) => route.startsWith("GET /tasks"))
  .map(([, operation]) => operation);

const preparedTasks = await loadOperations(taskRoutes);
const api = createClient({ baseURL: "https://api.example.test/v1", operations: preparedTasks });
await api.$operations.listTasks?.({ query: { limit: 20 } });
```

`all.js` adds the names to the downloaded code, but does not import every
operation implementation. Computed selections still have optional candidate
methods. Use this entry rather than enumerating or testing membership on the
small default reference objects.

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
[OperationStream](../reference/streaming.md#operationstream), not a promise of a
stream. [Request cancellation and timeouts](./transport.md) continue to apply to
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

`OperationPreparationError` from the selective entry exposes `stage` (`INPUT`,
`MODULE_LOAD`, `IDENTITY`, or `BINDING`) and retains the original cause when one
is available. A native import failure does not always reveal an HTTP status or
whether the cause was CSP, offline access, or a missing file; use the browser's
network diagnostics.

Measure initial preparation, subsequent feature preparation, the first Link
invocation, and repeat visits separately. Shared code is reused, but a selection
containing most operations can cost more than the full client because of lookup
and module overhead. Keep the regular full entry when that better fits the
application's measured workload.
