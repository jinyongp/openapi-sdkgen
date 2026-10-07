# Getting started

Generate an SDK from a small Todo document, then optionally run a call with a mock
response. You can complete every step without an API server. The final step
explains how to connect your own server.

## 1. Prepare the environment and project

You need **Node.js 22 or later** and `pnpm`. The Node.js requirement applies to
the npm CLI launcher. SDK generation does not require a TypeScript installation.
Use your existing application toolchain to build the generated source; the optional
call example below shows a standalone compiler setup.

Start in an empty directory:

```sh
mkdir sdkgen-todo
cd sdkgen-todo
```

Save this as `package.json`:

```json
{
  "private": true,
  "type": "module"
}
```

### npm

Install the CLI as a development dependency:

```sh
pnpm add -D openapi-sdkgen
pnpm exec openapi-sdkgen --version
```

### Homebrew

On macOS or Linux, you can install through Homebrew:

```sh
brew install jinyongp/tap/openapi-sdkgen
```

### Download an executable

Download the executable for your platform from
[GitHub Releases](https://github.com/jinyongp/openapi-sdkgen/releases) and put it
on your `PATH`.

With Homebrew or a downloaded executable, use `openapi-sdkgen` directly in the
generation commands below. These CLI installations do not require Node.js;
the optional call example uses Node.js to run the application.

## 2. Save the Todo document

Save this as `openapi.yaml`. It defines just two operations: list and create.

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

## 3. Generate the SDK

```sh
pnpm exec openapi-sdkgen generate \
  --input ./openapi.yaml \
  --target typescript \
  --output ./src/generated/api \
  --incremental
```

Generation succeeds when client source and types, including
`src/generated/api/index.ts`, appear. `--incremental` creates the directory on the
first run and safely updates it later. Regenerate generated files through the CLI.
See [Generate and verify](./generate.md) for update conditions.

## 4. Optional: verify calls with mock responses

SDK generation is complete at step 3. To try a call, use your application's
existing TypeScript setup. For an empty project without a compiler, optionally
install one for the type-checking and compilation commands below:

```sh
pnpm add -D typescript
```

Save this as `src/demo.ts`. The supplied `fetch` function returns `201` for creation
and `200` for listing. This checks request construction and response decoding;
the mock does not store data or send a network request.

```ts
import { createClient } from "./generated/api/index.js";

const todo = {
  id: "todo-1",
  title: "Write documentation",
  completed: false,
};
async function mockFetch(
  _input: RequestInfo | URL,
  init?: RequestInit,
) {
  if (init?.method === "POST") return Response.json(todo, { status: 201 });
  return Response.json({ items: [todo] }, { status: 200 });
}
const api = createClient({
  baseURL: "https://api.example.test",
  fetch: mockFetch,
});

const created = await api.todos.create({
  body: { title: "Write documentation" },
});
const todos = await api.todos.list();
console.log(created.title);
console.log(todos.items.length);
```

Type-check, compile, and run. `ES2022`, `DOM`, and `DOM.Iterable` provide types for
the standard APIs used by the generated source.

```sh
pnpm exec tsc --strict --target ES2022 \
  --module NodeNext --moduleResolution NodeNext \
  --lib ES2022,DOM,DOM.Iterable --outDir dist src/demo.ts
node dist/demo.js
```

Expected output:

```text
Write documentation
1
```

Once you see this output, you have generated the SDK, type-checked the calls, and
decoded mock responses.

## 5. Connect a real API and continue

Real calls need a server implementing this document's `GET /todos` and `POST /todos`.
Replace `baseURL` with that server's API base address and remove `fetch: mockFetch`.
`api.example.test` is a placeholder; the generator does not start an API server.

- Call surfaces and error handling: [Use the generated client](./client.md)
- Updates after document changes and CI checks: [Generate and verify](./generate.md)
- Tokens and timeouts: [Authentication, transport, and streams](./transport.md)
