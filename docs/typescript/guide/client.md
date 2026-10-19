# Use the generated client

Send a request, use the decoded response, and handle a failed call. The examples
below continue the Todo SDK from Getting started.

The generated TypeScript source exposes three complementary ways to call an
operation:

- [resource methods](/typescript/reference/client-api#resource-methods) such as `api.todos.create()` for readable application code;
- [`$routes`](/typescript/reference/client-api#routes) when the HTTP method and OpenAPI path are the stable identifier;
- [`$operations`](/typescript/reference/client-api#operations) when the document declares an `operationId`.

All three surfaces call the same OpenAPI operations. Choose the one that makes the
caller easiest to understand.

These snippets use the [Todo contract](https://github.com/jinyongp/openapi-sdkgen/blob/main/examples/typescript-todo-app/openapi.json).
[Download the Todo application](/downloads/typescript-todo-app.zip) to run it with a local server.

## Configure a client

Use [`createClient`](/typescript/reference/client-api#createclient) to configure one generated client.

```ts
import { createClient } from "./generated/api/index.js";

const api = createClient({
  baseURL: "https://api.example.test",
});
```

Replace `baseURL` with your server address. The example address is a placeholder.

When [`baseURL`](/typescript/reference/client-api#clientoptions) is omitted, the client uses applicable OpenAPI Server Objects.
An operation-level server takes precedence over a path-level server, which takes
precedence over a root server.

Authentication and custom Fetch behavior belong in client configuration or
request options. See [Authentication, transport, and streams](/typescript/guide/transport).

## Call Todo operations

Resource methods are the most readable choice when the generated resource tree
matches the way your application talks about the API:

```ts
const created = await api.todos.create({
  body: { title: "Write documentation" },
});

const todos = await api.todos.list();
```

Use [`$routes`](/typescript/reference/client-api#routes) to identify an operation by HTTP method and OpenAPI path, including
operations with no `operationId`:

```ts
const todos = await api.$routes["GET /todos"]();
```

Use [`$operations`](/typescript/reference/client-api#operations) when `operationId` is the stable application-facing name:

```ts
const todos = await api.$operations.listTodos();
```

## Name an input outside the call

Use the generated input helpers when preparing a request in a separate function
or variable. They preserve the operation's required fields and literal choices:

```ts
import type { OperationInput, RouteInput } from "./generated/api/index.js";

type CreateTodoInput = OperationInput<typeof api.$operations.createTodo>;
type CreateTodoBody = RouteInput<"POST /todos">["body"];
const body: CreateTodoBody = { title: "Write documentation" };
const input: CreateTodoInput = { body };
await api.$operations.createTodo(input);
```

## Read status and headers with `.raw()`

A normal call returns the generated successful output value. Use [`.raw()`](/typescript/reference/client-api#raw) when
the application also needs the exact status, decoded response headers, selected
content type, or original Fetch `Response`.

```ts
const result = await api.$operations.createTodo.raw({
  body: { title: "Write documentation" },
});

if (result.status === 201) {
  console.log(result.data.title);
  console.log(result.response.headers.get("content-type"));
}
```

The generated raw response is status-aware, so TypeScript can narrow fields
based on `result.status`.

## Edit response data {#work-with-decoded-response-objects}

Returned objects and arrays can be edited locally. This does not save changes on
the server; call an update operation declared in your document to persist them.
See [response body types](/typescript/reference/types#response-body-types) for
input/output types and object behavior.

## Handle call failures

Use `isAPIError` to recognize SDK errors and rethrow failures you cannot handle.

```ts
import { isAPIError } from "./generated/api/index.js";

try {
  await api.todos.list();
} catch (error: unknown) {
  if (!isAPIError(error)) throw error;
  console.error(error.code, error.message);
}
```

For an operation with declared error responses, `isOperationHTTPError` can narrow
the body type. See the [error reference](/typescript/reference/client-api#errors) for conditions and an example.

## Next tasks

- Tokens, cancellation, and timeouts: [Authentication and transport](/typescript/guide/transport)
- File uploads, follow-up calls, and streaming: [Files, Links, and streams](/typescript/guide/files-links-streams)
- Use only the APIs you need: [Selected operations](/typescript/guide/selective-client)
- Extract request and response types: [TypeScript types](/typescript/reference/types)
