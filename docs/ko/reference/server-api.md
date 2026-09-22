# 생성된 서버 API

TypeScript server add-on은 `--with server`로 생성합니다. OpenAPI Webhook과
Callback을 수신하기 위한 Fetch 기반 진입점을 제공합니다. HTTP listener,
framework 연결, route mount, 인증 정책은 애플리케이션이 담당합니다.

설정 흐름은 [Webhook과 Callback 수신](../guide/server.md)을 참고하세요.

## Import 경로

| Import 경로 | 용도 |
| --- | --- |
| `./generated/api/server/webhooks` | Webhook router와 generated Webhook handler 타입 |
| `./generated/api/server/callbacks` | Callback endpoint와 generated Callback handler 타입 |

## Webhook

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

`createWebhookRouter(handlers, options)`는 `fetch(request)` 메서드를 가진
Fetch 기반 router를 반환합니다. Generated `WebhookHandlers` 타입이 handler
tree와 각 handler의 request/response contract를 결정합니다.

### WebhookRouterOptions

```ts
interface WebhookRouterOptions {
  readonly routes: WebhookRoutes;
  readonly authenticate?: Authenticate;
  readonly codecs?: Readonly<Record<string, MediaCodec<unknown>>>;
  readonly streamCodecs?: Readonly<Record<string, StreamCodec>>;
  readonly maxBodyBytes?: number;
  readonly maxStreamFrameBytes?: number;
}
```

- `routes`: generated Webhook identity를 host-owned path에 연결합니다.
- `authenticate`: inbound request를 host가 허용하거나 거부합니다.
- `codecs`: 선언된 custom complete media value를 처리합니다.
- `streamCodecs`: media type별 inbound sequential protocol/adapter를 설정합니다.
- `maxBodyBytes`: complete inbound request body 하나의 전체 크기를 제한합니다.
  기본값은 8 MiB입니다. JSON, text, binary, URL-encoded, multipart, custom complete
  media, complete sequential body가 실제 byte limit을 넘으면 `413 Payload Too Large`를
  반환합니다. `Content-Length`는 조기 거부에만 사용합니다.
- `maxStreamFrameBytes`: adaptation 전 inbound wire frame 하나의 크기를 제한합니다.
  Streaming request body에는 `maxBodyBytes` 전체 제한을 적용하지 않습니다.

Stream codec 타입과 처리 순서는 [스트리밍 API](./streaming.md)를 참고하세요.

## Callback

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

`createCallbackHandlers(handlers, options?)`는 generated Fetch endpoint를
반환합니다. Callback URL expression은 애플리케이션이 소유합니다. Host가
OpenAPI Callback에 대응하는 실제 URL/path에 endpoint를 mount합니다.

Generated Callback identity는 상황에 따라 operation ID, exact route,
component Callback identity 기준으로 제공됩니다.

### CallbackHandlerOptions

```ts
interface CallbackHandlerOptions {
  readonly pathParams?: {
    // generated callback별 path parameter 값
  };
  readonly authenticate?: Authenticate;
  readonly codecs?: Readonly<Record<string, MediaCodec<unknown>>>;
  readonly streamCodecs?: Readonly<Record<string, StreamCodec>>;
  readonly maxBodyBytes?: number;
  readonly maxStreamFrameBytes?: number;
}
```

`maxBodyBytes`, `streamCodecs`, `maxStreamFrameBytes`는 Webhook과 같은
inbound body/stream 모델을 사용합니다. [스트리밍 API](./streaming.md)를 참고하세요.

## 인증

Generated server 진입점은 OpenAPI security candidate를 host-owned
`authenticate` 함수에 제공합니다. Credential 검증 방식은 host가 결정하며,
요청을 거부할 때 `Response`를 반환할 수 있습니다.

Generated code는 network listener를 만들거나 framework의 authentication/session
시스템을 선택하지 않습니다.

## Framework 연결

Framework adapter는 Fetch request/response semantics를 보존하면 됩니다.

```ts
const response = await router.fetch(request);
```

Router mount, framework request를 Fetch `Request`로 변환하는 작업, 결과
Fetch `Response` 반환은 애플리케이션이 담당합니다.
