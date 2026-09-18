# Webhook과 Callback 수신

기본 TypeScript target은 outbound client를 생성합니다. 애플리케이션이 API로
보낼 요청을 이 client로 호출합니다. OpenAPI Webhook과 Callback은 다른 시스템이
우리 애플리케이션으로 보내는 HTTP 요청을 설명합니다.

문서에 inbound 계약이 있고 애플리케이션이 이를 받는다면 선택적인 server
artifact set을 생성합니다.

```sh
openapi-sdkgen generate \
  --input ./openapi.yaml \
  --target typescript \
  --with server \
  --output ./src/generated/api
```

`--with server`는 Fetch 기반 handler와 router 진입점을 추가합니다. HTTP listener,
framework 연결, 공개 URL, 배포 방식, 인증 정책은 애플리케이션에서 구성합니다.

Client와 같은 OpenAPI schema를 사용해 inbound request를 파싱하고 검증한 뒤
타입이 지정된 값을 handler에 전달합니다.

## Todo Webhook 수신

OpenAPI 문서에 `todoCompleted`라는 Webhook이 있다고 가정합니다. 생성된 handler
계약을 구현합니다.

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

이 Webhook 이름을 애플리케이션의 수신 경로와 연결합니다.

```ts
const router = createWebhookRouter(handlers, {
  routes: {
    todoCompleted: "/webhooks/todos/completed",
  },
});

const response = await router.fetch(request);
```

Framework adapter는 들어온 요청을 Fetch `Request`로 바꾸고
`router.fetch(request)`를 호출한 뒤 결과 Fetch `Response`를 반환하면 됩니다.

## Inbound 요청 인증

들어오는 Webhook 인증은 호스트 애플리케이션의 책임입니다. OpenAPI inbound
operation에 security가 선언돼 있다면 body를 handler에 넘기기 전에 요청을
검증하는 authenticator를 제공합니다.

Todo Webhook에 signature를 확인한다고 가정하면:

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

생성 코드는 선언된 OpenAPI Security Requirement와 credential 위치를 해석합니다.
애플리케이션은 identity, authorization 정책, secret 조회, signature 검증을
담당합니다.

잘못된 inbound 입력은 타입이 지정된 값으로 handler에 전달되기 전에 거부됩니다.

## Callback 수신

Callback은 outbound operation에 선언되고, URL은 그 요청 데이터에서 정해질 수
있습니다. 예를 들어 `createTodo` 요청이 `callbackUrl`을 보내고
`statusUpdates` Callback이 이후 그 URL로 들어올 POST 요청을 설명할 수 있습니다.

생성된 Callback handler를 구현합니다.

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

생성된 key에는 OpenAPI의 source `operationId`, Callback 이름, runtime expression,
HTTP method가 그대로 반영됩니다. 애플리케이션의 callback 수신 route에 이
endpoint를 연결합니다.

```ts
const response =
  await callbacks.callbacks.createTodo.statusUpdates[
    "{$request.body#/callbackUrl}"
  ].POST.fetch(request);
```

Runtime expression은 OpenAPI 계약에 그대로 유지되고, callback 수신 URL은
애플리케이션에서 정합니다.

## Inbound stream 사용자 정의

OpenAPI 3.2 inbound body에 `itemSchema`가 있으면 생성된 handler는 request
stream을 소비하면서 검증된 item을 받습니다. Webhook과 Callback 옵션도 outbound
client와 같은 `StreamCodec` 모델을 사용합니다.

Wire framing은 표준이고 application event 변환만 필요하다면
`StreamAdapter`를 사용합니다. 예를 들어 Todo SSE의 문자열 `data`를
application event로 변환하면서 built-in SSE protocol을 그대로 사용할 수
있습니다.

```ts
import type {
  ServerSentEvent,
  StreamAdapter,
} from "./generated/api";

const todoAdapter: StreamAdapter<ServerSentEvent, TodoEvent> = {
  async *decode(events) {
    for await (const event of events) {
      if (event.event !== "todo") continue;
      yield JSON.parse(event.data) as TodoEvent;
    }
  },
  async *encode(items) {
    for await (const item of items) {
      yield { event: "todo", data: JSON.stringify(item) };
    }
  },
};

const router = createWebhookRouter(handlers, {
  routes: {
    todoCompleted: "/webhooks/todos/completed",
  },
  streamCodecs: {
    "text/event-stream": { adapter: todoAdapter },
  },
  maxStreamFrameBytes: 256 * 1024,
});
```

Adapter가 만든 값은 handler에 전달되기 전에 선언된 `itemSchema` 검증과
property projection을 거칩니다. 사용자 정의 sequential media의 byte framing이
필요하면 `StreamProtocol`을 사용합니다. `maxStreamFrameBytes`는 adapter
적용 전의 wire frame 하나를 제한합니다. `createCallbackHandlers`도 같은
`streamCodecs`와 frame limit 옵션을 제공합니다.

## Generated server artifact

Server artifact set은 OpenAPI에 따른 decoding, validation, 타입이 지정된 handler
계약, Fetch 기반 request/response 처리를 담당합니다.

## 애플리케이션 책임

애플리케이션은 다음 항목을 담당합니다.

- HTTP listener와 framework 연결
- 공개 route 설정
- 인증, identity, authorization
- secret과 credential 저장
- OpenAPI 요청 계약 밖의 retry 및 delivery 정책

OpenAPI 문서가 outbound operation만 설명한다면 기본 client artifact set을
사용합니다.

버전별 Webhook/Callback 지원 범위는
[OpenAPI 지원 범위](../reference/capabilities.md)에서, 생성된 import 경로는
[생성된 클라이언트 API](../reference/client-api.md)에서 확인할 수 있습니다.
