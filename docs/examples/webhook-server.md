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
Webhook/Callback artifacts under `./generated/api/server/`.

## 3. Implement the generated handler

```ts
// webhook-app/src/worker.ts
import {
  createWebhookRouter,
  type WebhookHandlers,
} from "./generated/api/server/webhooks";

declare function verifyWebhook(request: Request): boolean | Promise<boolean>;

const handlers: WebhookHandlers = {
  todoCompleted: {
    POST: async ({ body }) => {
      console.log(body.todoID, body.completed);
      return { status: 204 };
    },
  },
};

const router = createWebhookRouter(handlers, {
  routes: {
    todoCompleted: "/hooks/todos/completed",
  },
  authenticate: async ({ request }) =>
    await verifyWebhook(request)
      ? undefined
      : new Response("Unauthorized", { status: 401 }),
});

export default {
  fetch(request: Request) {
    return router.fetch(request);
  },
};
```

The host owns the listener/runtime and the concrete public route. The generated
router owns OpenAPI request decoding, validation, typed handler input, and
response encoding.

See [Generated server API](../reference/server-api.md) for router/options lookup
and [OpenAPI support](../reference/capabilities.md#webhooks-and-callbacks) for
version-specific support.
