# Generated server API

The TypeScript server add-on is generated with `--with server`. It provides
Fetch-native entry points for receiving OpenAPI Webhooks and Callbacks. The
application still owns the HTTP listener, framework integration, route mounting,
and authentication policy.

For a guided setup, see
[Receive Webhooks and Callbacks](../guide/server.md).

## Import paths

| Import path | Use |
| --- | --- |
| `./generated/api/server/webhooks` | Webhook router and generated Webhook handler types |
| `./generated/api/server/callbacks` | Callback endpoints and generated Callback handler types |

## Webhooks

### createWebhookRouter

```ts
import { createWebhookRouter } from "./generated/api/server/webhooks";

const router = createWebhookRouter(
  {
    todoCompleted: {
      POST: async ({ body }) => {
        return { status: 204 };
      },
    },
  },
  {
    routes: {
      todoCompleted: "/hooks/todo-completed",
    },
  },
);
```

`createWebhookRouter(handlers, options)` returns a Fetch-native router with a
`fetch(request)` method. The generated `WebhookHandlers` type determines the
handler tree and each handler's request/response contract.

### WebhookRouterOptions

```ts
interface WebhookRouterOptions {
  readonly routes: WebhookRoutes;
  readonly authenticate?: Authenticate;
  readonly codecs?: Readonly<Record<string, MediaCodec<unknown>>>;
  readonly streamCodecs?: Readonly<Record<string, StreamCodec>>;
  readonly maxStreamFrameBytes?: number;
}
```

- `routes` binds generated Webhook identities to host-owned paths.
- `authenticate` lets the host accept or reject an inbound request.
- `codecs` handles declared custom complete media values.
- `streamCodecs` configures inbound sequential protocols/adapters by media type.
- `maxStreamFrameBytes` limits one inbound wire frame before adaptation.

For stream codec types and ordering, see [Streaming API](./streaming.md).

## Callbacks

### createCallbackHandlers

```ts
import { createCallbackHandlers } from "./generated/api/server/callbacks";

const callbacks = createCallbackHandlers({
  callbacks: {
    createTodo: {
      statusUpdates: {
        "{$request.body#/callbackUrl}": {
          POST: async ({ body }) => {
            return { status: 204 };
          },
        },
      },
    },
  },
});
```

`createCallbackHandlers(handlers, options?)` returns generated Fetch-native
endpoints. Callback URL expressions remain application-owned: the host mounts
the endpoint at the concrete URL/path that corresponds to the OpenAPI callback.

Generated callback identities are exposed by operation ID, exact route, or
component Callback identity when applicable.

### CallbackHandlerOptions

```ts
interface CallbackHandlerOptions {
  readonly authenticate?: Authenticate;
  readonly codecs?: Readonly<Record<string, MediaCodec<unknown>>>;
  readonly streamCodecs?: Readonly<Record<string, StreamCodec>>;
  readonly maxStreamFrameBytes?: number;
  readonly pathParams?: {
    // generated callback-specific path parameter values
  };
}
```

`streamCodecs` and `maxStreamFrameBytes` use the same inbound stream model as
Webhook handlers. See [Streaming API](./streaming.md).

## Authentication

Generated server entry points expose the OpenAPI security candidates to the
host-owned `authenticate` function. The host decides how credentials are
verified and can return a `Response` to reject the request.

The generated code does not create a network listener and does not choose a
framework authentication/session system.

## Framework integration

A framework adapter only needs to preserve Fetch request/response semantics:

```ts
const response = await router.fetch(request);
```

Mounting the router, converting a framework request to a Fetch `Request`, and
returning the resulting Fetch `Response` remain application responsibilities.
