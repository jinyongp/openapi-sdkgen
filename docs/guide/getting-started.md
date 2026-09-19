# Getting started

[`openapi-sdkgen`](../reference/cli.md) generates application-owned TypeScript client source from an
OpenAPI 3.x document. This guide starts with a small Todo API, generates the SDK
into your application, and makes the first request.

## 1. Install the CLI

For an application repository, installing the CLI as a development dependency
keeps the generator version reproducible with the rest of the project:

```sh
pnpm add -D openapi-sdkgen
pnpm exec openapi-sdkgen --version
```

The commands below use `pnpm exec openapi-sdkgen`. Homebrew and GitHub Release
installations can invoke `openapi-sdkgen` directly.

For a one-off trial, run `pnpm dlx openapi-sdkgen ...`.

On macOS or Linux, Homebrew is another installation option:

```sh
brew install jinyongp/tap/openapi-sdkgen
```

## 2. Create a Todo OpenAPI document

Save this as `openapi.yaml`:

```yaml
openapi: 3.2.0
info:
  title: Todo API
  version: 1.0.0
paths:
  /todos:
    get:
      operationId: listTodos
      responses:
        "200":
          description: Todo list
          content:
            application/json:
              schema:
                type: object
                required: [items]
                properties:
                  items:
                    type: array
                    items:
                      $ref: "#/components/schemas/Todo"
    post:
      operationId: createTodo
      requestBody:
        required: true
        content:
          application/json:
            schema:
              type: object
              required: [title]
              properties:
                title:
                  type: string
      responses:
        "201":
          description: Created todo
          content:
            application/json:
              schema:
                $ref: "#/components/schemas/Todo"
components:
  schemas:
    Todo:
      type: object
      required: [id, title, completed]
      properties:
        id:
          type: string
        title:
          type: string
        completed:
          type: boolean
```

The document gives the two operations stable `operationId` values and declares
the request and response shapes that will become TypeScript types.

## 3. Generate into your application source

Choose a new directory for the first generation:

```sh
pnpm exec openapi-sdkgen generate \
  --input ./openapi.yaml \
  --target typescript \
  --output ./src/generated/api
```

The generated directory contains normal application source, including the source
runtime. Your existing TypeScript compiler or bundler builds it together with the
rest of the application.

Regenerate generator-owned files through the CLI. When the OpenAPI document
changes, use [`--incremental`](../reference/cli.md#fresh-incremental-and-check-modes) to update the same managed directory safely. See
[Generate and verify an SDK](./generate.md) for regeneration and CI workflows.

## 4. Create the client

Create the generated client with
[`createClient`](../reference/client-api.md#createclient):

```ts
import { createClient } from "./generated/api";

const api = createClient({
  baseURL: "https://api.example.test/v1",
});
```

When the OpenAPI document declares usable Server Objects, omitting [`baseURL`](../reference/client-api.md#clientoptions) lets
the generated client use those server definitions.

::: details Running compiled output directly with Node ESM

Vite, Next.js, Nuxt, and similar bundlers resolve the generated directory entry.
For compiled output executed directly with Node ESM, import the explicit `index.js`
file:

```ts
import { createClient } from "./generated/api/index.js";
```
:::

## 5. Call the Todo API

Resource methods are the shortest interface for normal application code:

```ts
const created = await api.todos.create({
  body: { title: "Write documentation" },
});

const todos = await api.todos.list();
```

Every operation is also available through its exact HTTP route, and operations
with an `operationId` are available through [`$operations`](../reference/client-api.md#operations):

```ts
await api.$routes["GET /todos"]();
await api.$operations.listTodos();
```

Continue with [Generate and verify an SDK](./generate.md) to learn about
incremental generation, [`--check`](../reference/cli.md#fresh-incremental-and-check-modes), authenticated inputs, and remote references.
Then see [Use the generated client](./client.md) for responses, Links, streams,
and other call surfaces.
