# Receive an OpenAPI Webhook

This example shows the generated server add-on as an application boundary. The
OpenAPI contract describes an inbound Webhook, openapi-sdkgen generates the
typed router, and the host application chooses the public route and runtime.

```text
webhook-app/
  openapi.yaml
  src/generated/api/
  src/worker.ts
```

## 1. Declare the Webhook

OpenAPI 3.1 and 3.2 support root Webhook Objects. The application owns the
public contract:

```yaml
openapi: 3.1.1
info:
  title: Todo Webhooks
  version: 1.0.0

paths: {}

webhooks:
  todoCompleted:
    post:
      security:
        - webhookAuth: []
      requestBody:
        required: true
        content:
          application/json:
            schema:
              type: object
              required: [todoID, completed]
              properties:
                todoID:
                  type: string
                completed:
                  type: boolean
      responses:
        "204":
          description: Accepted

components:
  securitySchemes:
    webhookAuth:
      type: apiKey
      in: header
      name: x-webhook-token
```

## 2. Generate the server add-on

```sh
pnpm exec openapi-sdkgen generate \
  --input ./openapi.yaml \
  --target typescript \
  --with server \
  --output ./src/generated/api
```

The base target still contains the outbound client. `--with server` adds the
Webhook/Callback artifacts under `./src/generated/api/server/`.

## 3. Implement the generated handler

```ts
// webhook-app/src/worker.ts
import {
  createWebhookRouter,
  type WebhookHandlers,
} from "./generated/api/server/webhooks.js";

const handlers: WebhookHandlers = {
  todoCompleted: {
    POST: async ({ body }) => {
      console.log(body.todoID, body.completed);
      return { status: 204 };
    },
  },
};

export function createReceiver(expectedToken: string) {
  return createWebhookRouter(handlers, {
    routes: {
      todoCompleted: "/hooks/todos/completed",
    },
    authenticate: ({ request }) =>
      request.headers.get("x-webhook-token") === expectedToken
        ? undefined
        : new Response("Unauthorized", { status: 401 }),
  });
}
```

The host owns the listener/runtime and the concrete public route. The generated
router owns OpenAPI request decoding, validation, typed handler input, and
response encoding.

## 4. Check requests and responses locally

Use Node.js 22+ and the ESM setup from
[Getting started](../guide/getting-started.md). Use your existing build tool;
the `tsc` commands below need a compiler only if you choose that route. In a separate project, save the
contract as `openapi.yaml` and the handler as `src/worker.ts`, then generate it.
Save this as `src/demo.ts`:

```ts
import { createReceiver } from "./worker.js";

const router = createReceiver("local-example-token");

async function send(token: string, body: unknown) {
  const response = await router.fetch(new Request(
    "http://localhost/hooks/todos/completed",
    {
      method: "POST",
      headers: {
        "content-type": "application/json",
        "x-webhook-token": token,
      },
      body: JSON.stringify(body),
    },
  ));
  return response.status;
}

console.log(await send("local-example-token", { todoID: "todo-1", completed: true }));
console.log(await send("wrong-token", { todoID: "todo-1", completed: true }));
console.log(await send("local-example-token", { todoID: "todo-1" }));
```

```sh
pnpm exec tsc --strict --target ES2022 --module NodeNext --moduleResolution NodeNext \
  --lib ES2022,DOM,DOM.Iterable --outDir dist src/demo.ts src/worker.ts
node dist/demo.js
```

The handler logs `todo-1 true`; response statuses are `204`, `401`, and `400`.
Fetch `Request` objects exercise authentication, validation, and the handler
without starting an HTTP listener. The token is for local testing. For deployment,
load the token from your secret store and call `router.fetch(request)` from your
host's HTTP handler.

`authenticate` runs only for operations that require declared security and do
not allow anonymous access. Keep the `security` declaration in the contract.

See [Generated server API](../reference/server-api.md) for router/options lookup
and [OpenAPI support](../reference/capabilities.md#webhooks-and-callbacks) for
version-specific support.
