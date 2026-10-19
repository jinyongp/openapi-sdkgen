# Receive an OpenAPI Webhook

<span id="_1-declare-the-webhook"></span>
<span id="_2-generate-the-server-add-on"></span>
<span id="_3-implement-the-generated-handler"></span>
<span id="_4-check-requests-and-responses-locally"></span>

Generate a typed request handler from an OpenAPI Webhook.
[Download the complete example](/downloads/typescript-webhook-app.zip) for the contract,
receiver and runnable authentication/validation checks. Follow the included README to run it.

## Connect the handler {#handler}

Generate the example's `openapi.yaml` with `--with server`. The following code handles
`todoCompleted`; load `expectedToken` from your application's authentication configuration.

```ts
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

const router = createWebhookRouter(handlers, {
  routes: { todoCompleted: "/hooks/todos/completed" },
  authenticate: ({ request }) =>
    request.headers.get("x-webhook-token") === expectedToken
      ? undefined
      : new Response("Unauthorized", { status: 401 }),
});
```

Call `router.fetch(request)` from your HTTP host and forward its returned `Response`.
The SDK decodes and validates the request; your application chooses the route and authentication policy.

## Authentication and validation {#validation}

This contract requires an `x-webhook-token` header. Valid credentials and input reach
the handler, which returns `204`. Invalid credentials return `401`; invalid input returns `400`.
`authenticate` runs only for secured operations, so retain the contract's `security` declaration.

See [Server API](../reference/server-api.md) for router options and callbacks, and the
[webhook and callback guide](../guide/server.md) for request size limits.
