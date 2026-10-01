# Generated TypeScript types

The generated SDK exports request, response, component, and enum types from its main
entry point.

See [Generated client API](./client-api.md) for calling the generated client.

## Compiler support

Generated client and server source requires **TypeScript 5.7.3 or later** with
`ES2022`, `DOM`, and `DOM.Iterable` libraries. The runtime uses the generic typed
arrays introduced in [TypeScript 5.7](https://www.typescriptlang.org/docs/handbook/release-notes/typescript-5-7.html).
Your application owns its compiler and bundler; the generator does not install a
TypeScript compiler into the generated directory.

CI runs `just agent ts-compat` against pinned TypeScript **5.7.3, 5.9.3, 6.0.3,
and 7.0.2**. Each version checks generated client and server source plus a public
type consumer probe after removing `@ts-nocheck`, with `strict`,
`noUncheckedIndexedAccess`, `verbatimModuleSyntax`, `isolatedModules`, and
`skipLibCheck: false`. These checks cover the 3.2 normative, webhook, and nested
Schema-reference fixtures. The full runtime/conformance suite and external
compatibility benchmarks use the repository compiler, TypeScript 7.0.2.

Newer compiler versions are checked when added to this matrix. The minimum is a
consumer source contract, not a promise that every future compiler change is
already tested. TypeScript 5.6 and earlier are outside this support contract.

This matrix includes the readable-stream iterator correction recorded in
`CHANGELOG.md`. Generated source from v9.0.0 predates that correction and can
fail strict checking with older compilers after removing `@ts-nocheck`.
Regenerate with a generator release containing the correction.

## Choose a type source

| Source | Helper |
| --- | --- |
| generated client method | `Operation*<typeof method>` |
| OpenAPI `operationId` | `Operation*<"operationId">` |
| `"METHOD /path"` route | `Route*<"METHOD /path">` |
| `components.schemas` name | `ComponentInput<Name>` / `ComponentOutput<Name>` |

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

const updateTodo = api.todos("todo-1").update;
type UpdateInput = OperationInput<typeof updateTodo>;
type UpdateBody = OperationBody<typeof updateTodo>;
```

`OperationInput` is the complete argument accepted by the method. `OperationBody` is
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
```

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
