# Generated TypeScript types

Code on this page illustrates API shapes. Operation names, parameters, media,
and security come from your own contract. The small Todo contract in
[Getting started](../guide/getting-started.md) does not include every feature below.

The generated SDK exports request, response, component, and enum types from its main
entry point.

See [Generated client API](./client-api.md) for calling the generated client.

For the Todo type examples, generate from the
[extended Todo document](/examples/todo-types.json). It includes `completed` and
`limit` queries, `getTodo`, `updateTodo`, and the `TodoStatus` enum. Named-client
types use the separate [named-client document](/examples/named-clients.json)
and configuration from [its guide](../guide/named-clients.md).

## Compiler support

Generated client and server source requires **TypeScript 5.7.3 or later** with
`ES2022`, `DOM`, and `DOM.Iterable` libraries. Compile it with your application's
existing TypeScript compiler and bundler.

The matrix below was verified with TypeScript
**5.7.3, 5.9.3, 6.0.3, and 7.0.2**.

The generated SDK supports these module configurations with `strict`
enabled or disabled, and with or without `isolatedDeclarations`:

| Module format | `module` / `moduleResolution` | Settings |
| --- | --- | --- |
| ESM for Node.js | `NodeNext` / `NodeNext` | `"type": "module"` in `package.json` |
| ESM for a bundler | `ESNext` / `Bundler` | Your application's bundler |
| CommonJS for Node.js | `NodeNext` / `NodeNext` | `"type": "commonjs"` and `verbatimModuleSyntax: false` |

With `strict: false`, set `exactOptionalPropertyTypes: false`. Declaration
generation with `isolatedDeclarations: true` requires `declaration: true`.
These configurations cover the root SDK, API selections, named clients, server
code, and optional metadata.

## Typecheck generated source {#source-checking}

Use `--typecheck` to check the generated
SDK's implementation with your application's compiler:

```sh
openapi-sdkgen generate --input ./openapi.yaml --target typescript \
  --output ./src/generated/api --typecheck
```

For repeated generation, put the same choice in your TOML configuration:

```toml
[typescript]
typecheck = true
```

The default `false` includes `@ts-nocheck` in each generated file. Enabling
`--typecheck` removes this directive; your application's compiler checks the
generated source. An explicit `--typecheck=false` overrides a TOML value of
`true`. Both choices
retain the same API types and runtime behavior. The setting applies to the
root SDK, named clients, server code, and metadata together. Use `--incremental`
to change it in an existing managed output directory.

## Choose a type source

The tables below describe the root SDK's type helpers. For a configured client
entry, use the [named client types](#named-clients) described below.

| Source | Helper |
| --- | --- |
| generated client method | `Operation*<typeof method>` |
| OpenAPI `operationId` | `Operation*<"operationId">` |
| `"METHOD /path"` route | `Route*<"METHOD /path">` |
| `components.schemas` name | `ComponentInput<Name>` / `ComponentOutput<Name>` |

## Types for a named client {#named-clients}

Each named entry exports a concrete `Client` and the
component types required by its selected APIs and Link helpers. Import these
types from the same entry as `createClient`. For a catalog API using the
`Product` component:

```ts
import {
  createClient,
  type Client,
  type ComponentOutput,
} from "./generated/api/clients/catalog/index.js";

type Product = ComponentOutput<"Product">;
type GetProductInput = Parameters<Client["$routes"]["GET /products/{id}"]>[0];
type GetProductOutput = Awaited<ReturnType<Client["$routes"]["GET /products/{id}"]>>;
```

`ComponentInput` describes a model's request representation and
`ComponentOutput` its response representation. A component name used solely by
another client is absent; a representation unused by this client's APIs is
`never`. Use `Parameters` and `ReturnType` on this client's methods to extract
their exact call types. For optional-input methods, `Parameters<Method>[0]`
also includes transport options because they can be passed as the only argument.
The `Operation*` and `Route*` helpers below belong to the root SDK; named entries
export their selected client and component contracts.

## Response body types

Response objects and arrays are mutable. Editing them changes only local data;
it does not send a request or save changes on the server.

Input types accept readonly values. OpenAPI `readOnly` properties are omitted
from request types, and `writeOnly` properties are omitted from response types.
`readOnly` does not make a returned field immutable.

## Extract from a generated method

Pass the type of a generated method to an `Operation*` helper.

```ts
import {
  createClient,
  type OperationBody,
  type OperationInput,
  type OperationQuery,
} from "./generated/api";

const api = createClient({
  baseURL: "https://api.example.test/v1",
});

const listTodos = api.$operations.listTodos;
type TodoFilters = OperationQuery<typeof listTodos>;

const updateTodo = api.todos("todo-1").patch;
type UpdateInput = OperationInput<typeof updateTodo>;
type UpdateBody = OperationBody<typeof updateTodo>;
```

`OperationInput` is the complete generated input accepted by the method. `OperationBody` is
its request body. A resource-tree method includes the arguments that remain after
its selectors have been applied.

```ts
const filters = { completed: false } satisfies TodoFilters;

async function update(body: UpdateBody) {
  return updateTodo({ body });
}
```

## Extract by operation ID or route

Use an OpenAPI `operationId` with `Operation*` helpers, or a `"METHOD /path"` string
with `Route*` helpers.

```ts
import type {
  OperationBody,
  OperationInput,
  OperationOutput,
  OperationQuery,
  RouteBody,
  RouteInput,
  RouteOutput,
  RouteParameter,
} from "./generated/api";

type ListInput = OperationInput<"listTodos">;
type ListOutput = OperationOutput<"listTodos">;
type ListQuery = OperationQuery<"listTodos">;
type CreateBody = OperationBody<"createTodo">;

type UpdateInput = RouteInput<"PATCH /todos/{todoID}">;
type UpdateOutput = RouteOutput<"PATCH /todos/{todoID}">;
type UpdateBody = RouteBody<"PATCH /todos/{todoID}">;
type TodoID = RouteParameter<
  "PATCH /todos/{todoID}",
  "path",
  "todoID"
>;
```

## Request and response helpers

| Value | Operation helper | Route helper |
| --- | --- | --- |
| complete call input | `OperationInput` | `RouteInput` |
| successful output | `OperationOutput` | `RouteOutput` |
| streaming item | `OperationStreamItem` | `RouteStreamItem` |
| request body | `OperationBody` | `RouteBody` |
| path parameters | `OperationPath` | `RoutePath` |
| query parameters | `OperationQuery` | `RouteQuery` |
| query-string parameters | `OperationQuerystring` | `RouteQuerystring` |
| headers | `OperationHeaders` | `RouteHeaders` |
| cookies | `OperationCookies` | `RouteCookies` |
| one parameter | `OperationParameter` | `RouteParameter` |
| complete contract | `OperationContract` | `RouteContract` |

Parameter helpers take a location and parameter name:

```ts
type Limit = OperationParameter<"listTodos", "query", "limit">;
```

Optional properties and schema-declared `null` values are preserved. Use
`OperationInput` or `RouteInput` to check whether a complete call can omit a request
section.

## Component types

Use component helpers for schemas declared in `components.schemas`.
Only components needed by generated APIs and their dependencies are included.

```ts
import type { ComponentInput, ComponentOutput } from "./generated/api";

type TodoInput = ComponentInput<"Todo">;
type TodoOutput = ComponentOutput<"Todo">;
```

`readOnly` fields appear in output types, and `writeOnly` fields appear in input
types.

## Enum values and types

A component enum provides a TypeScript type and runtime values.

```yaml
components:
  schemas:
    TodoStatus:
      type: string
      enum: [TODO, DONE]
    Todo:
      type: object
      properties:
        status:
          $ref: "#/components/schemas/TodoStatus"
```

Merge these declarations into the document and use `Todo` in an API schema, as
in the extended Todo example. An unused component enum is not generated.

```ts
import { Enums, isEnumValue, type EnumValue } from "./generated/api/enums";

type TodoStatus = EnumValue<"TodoStatus">;
// "TODO" | "DONE"

const defaultStatus: TodoStatus = Enums.TodoStatus.TODO;
const completedStatus = Enums.TodoStatus.DONE;

for (const status of Enums.TodoStatus) {
  console.log(status);
}

const options = Array.from(Enums.TodoStatus);

declare const input: unknown;

if (isEnumValue(Enums.TodoStatus, input)) {
  input satisfies TodoStatus;
}
```

The same `Enums`, `EnumValue`, and `isEnumValue` exports remain available from the
main `./generated/api` entry, so existing imports remain valid. Use the dedicated
`./generated/api/enums` entry for modules that need generated enum runtime values
and types.

`Enums` contains enums declared as component schemas. Inline and nested enums remain
available through their generated request, response, or component types.

## Additional exported types

The generated entry point also exports types for raw calls, resource-tree calls,
pagination, Links, and streams.

| Type | Use |
| --- | --- |
| `OperationMethod<Route>` | generated operation call |
| `OperationRawCall<Route>` | raw operation call |
| `ResourceCall<Route>` | resource-tree call |
| `RawCall<Route>` | raw resource-tree call |
| `PaginateCall<Route>` | pagination call |
| `LinkCalls<Route>` | OpenAPI Link calls |
| `StreamCall<Route>` | streaming call |
| `RouteStreamItem<Route>` | item emitted by one exact route's stream |
| `OperationStreamItem<Source>` | stream item selected by operation ID or generated method |
| `OperationStream<T>` | lazy single-consumer streaming response handle |
| `StreamSource<T>` | `AsyncIterable<T>` or `ReadableStream<T>` request source |
| `ServerSentEvent` | standard SSE frame value |
| `StreamProtocol<Frame>` | custom sequential-media byte framing |
| `StreamAdapter` | application transform layered on a stream protocol |
| `StreamCodec` | optional protocol/adapter combination |
| `StreamResponseMetadata` | status, headers, content type, and request metadata for an open stream |
| `CursorPaginationInput` | cursor pagination input |
| `OffsetPaginationInput` | offset pagination input |
| `BothPaginationInput` | cursor or offset pagination input |
| `SortDirection` | `"asc" | "desc"` and runtime constants |

Stream lifecycle, request sources, SSE frames, protocol/adapter contracts,
and codec configuration are documented in [Streaming API](./streaming.md).

`Components`, `Operations`, and `Routes` provide complete generated type maps.
