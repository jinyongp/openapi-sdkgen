# Receive Webhooks and Callbacks

The normal TypeScript target generates an outbound client that sends requests to
an API. OpenAPI Webhooks and Callbacks describe HTTP requests that another system
sends to your application.

When your document contains those inbound contracts, generate the optional
server artifact set:

```sh
openapi-sdkgen generate \
  --input ./openapi.yaml \
  --target typescript \
  --with server \
  --output ./src/generated/api
```

`--with server` adds Fetch-native handler and router entry points. Your application
provides the HTTP listener, framework integration, public URL, deployment model,
and authentication policy.

The same OpenAPI schemas used by the client are used to parse and validate
inbound requests before typed values reach your handlers.

## Receive a Todo Webhook

Suppose the OpenAPI document defines a Webhook named `todoCompleted`.
Implement the generated handler contract:

```ts
import {
  createWebhookRouter,
  type WebhookHandlers,
} from "./generated/api/server/webhooks";

const handlers: WebhookHandlers = {
  todoCompleted: {
    POST: async ({ body }) => {
      console.log(body.todoID, body.completed);
      return { status: 204 };
    },
  },
};
```

Map the Webhook name to the application path that receives it:

```ts
const router = createWebhookRouter(handlers, {
  routes: {
    todoCompleted: "/webhooks/todos/completed",
  },
});

const response = await router.fetch(request);
```

A framework adapter converts the incoming request to a Fetch `Request`, calls
`router.fetch(request)`, and returns the resulting Fetch `Response`.

## Authenticate inbound requests

Authentication of incoming Webhooks is a host responsibility. If the OpenAPI
inbound operation is secured, provide an authenticator that verifies the
request before the body reaches the application handler.

For a signature-style Todo webhook:

```ts
const router = createWebhookRouter(handlers, {
  routes: {
    todoCompleted: "/webhooks/todos/completed",
  },
  authenticate: ({ request }) =>
    request.headers.get("x-todo-signature") === expectedSignature
      ? undefined
      : new Response("Unauthorized", { status: 401 }),
});
```

Generated code interprets the declared OpenAPI security requirements and
credential locations. The application handles identity, authorization policy,
secret lookup, and signature verification.

Malformed input is rejected before your handler receives a typed value.

## Receive a Callback

Callbacks are declared on an outbound operation and their URL may come from
that operation's request data. For example, a `createTodo` request can provide
a `callbackUrl`, while the OpenAPI Callback named `statusUpdates` describes
what will later be POSTed to that URL.

Implement the generated Callback handler:

```ts
import {
  createCallbackHandlers,
  type CallbackHandlers,
} from "./generated/api/server/callbacks";

const handlers: CallbackHandlers = {
  callbacks: {
    createTodo: {
      statusUpdates: {
        "{$request.body#/callbackUrl}": {
          POST: async ({ body }) => {
            console.log(body.todoID, body.completed);
            return { status: 204 };
          },
        },
      },
    },
  },
};

const callbacks = createCallbackHandlers(handlers);
```

The generated key preserves the source `operationId`, Callback name, runtime
expression, and HTTP method from the OpenAPI document. Connect that endpoint to the route where your application receives the callback:

```ts
const response =
  await callbacks.callbacks.createTodo.statusUpdates[
    "{$request.body#/callbackUrl}"
  ].POST.fetch(request);
```

The runtime expression remains part of the OpenAPI contract, while your application
chooses the deployment URL that receives the callback.

## Generated server artifacts

The server artifact set owns OpenAPI-specific decoding, validation, typed
handler contracts, and Fetch-native request/response handling.

## Application responsibilities

The application owns:

- the HTTP listener and framework integration;
- public route configuration;
- authentication, identity, and authorization;
- secrets and credential storage;
- retries or delivery semantics outside the OpenAPI request contract.

For OpenAPI documents limited to outbound calls, use the base client artifact set.

See [OpenAPI support](../reference/capabilities.md) for version-specific Webhook
and Callback support and [Generated client API](../reference/client-api.md) for
the generated import paths.
