<span id="webhook과-callback-수신"></span>

# 웹훅과 콜백 수신

기본 TypeScript 생성 대상은 외부 호출 클라이언트를 생성합니다. 애플리케이션이 API로
보낼 요청을 이 클라이언트로 호출합니다. OpenAPI 웹훅과 콜백은 다른 시스템이
우리 애플리케이션으로 보내는 HTTP 요청을 설명합니다.

문서에 수신 계약이 있고 애플리케이션이 이를 받는다면 선택적인 서버 코드를 생성합니다.

```sh
openapi-sdkgen generate \
  --input ./openapi.yaml \
  --target typescript \
  --with server \
  --output ./src/generated/api
```

[`--with server`](../reference/cli.md#with-server)는 Fetch 기반 처리 함수와 라우터 진입점을 추가합니다. HTTP 서버 리스너,
프레임워크 연결, 공개 URL, 배포 방식, 인증 정책은 애플리케이션에서 구성합니다.

클라이언트와 같은 OpenAPI 스키마를 사용해 수신 요청을 파싱하고 검증한 뒤
타입이 지정된 값을 처리 함수에 전달합니다.

서버 코드를 추가해도 클라이언트 사용법은 같습니다.

시작하기의 Todo 명세에는 수신 계약이 없습니다. 아래 웹훅 코드는
[웹훅 수신 예제](../examples/webhook-server.md)의 명세를 사용합니다. 콜백과 스트림은
각 절에 설명한 별도 선언이 필요합니다. 처음에는 예제의 `204`·`401`·`400` 응답부터 확인하세요.

## Todo 웹훅 수신

OpenAPI 문서에 `todoCompleted`라는 웹훅이 있다고 가정합니다.
[생성된 웹훅 처리 함수 타입](../reference/server-api.md#createwebhookrouter)에 맞춰 구현합니다.

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

이 웹훅 이름을 애플리케이션의 수신 경로와 연결합니다.

```ts
const router = createWebhookRouter(handlers, {
  routes: {
    todoCompleted: "/webhooks/todos/completed",
  },
});

const response = await router.fetch(request);
```

프레임워크 어댑터는 들어온 요청을 Fetch `Request`로 바꾸고
`router.fetch(request)`를 호출한 뒤 결과 Fetch `Response`를 반환하면 됩니다.
예제 명세에는 인증이 선언되어 있으므로 다음 절의 `authenticate`도 지정해야 합니다.
생략하면 처리 함수를 호출하기 전에 `401`을 반환합니다.

<span id="inbound-요청-인증"></span>

## 수신 요청 인증

들어오는 웹훅 인증은 호스트 애플리케이션의 책임입니다. OpenAPI 수신
API에 인증이 선언돼 있다면 본문을 처리 함수에 넘기기 전에 요청을
검증하는 인증 함수를 제공합니다.

웹훅 예제의 헤더 API 키를 검사한다면 `expectedToken`을 애플리케이션의 비밀
저장소에서 가져오세요. 예제 명세의 `security` 선언도 함께 사용합니다.

```ts
const router = createWebhookRouter(handlers, {
  routes: {
    todoCompleted: "/webhooks/todos/completed",
  },
  authenticate: ({ request }) =>
    request.headers.get("x-webhook-token") === expectedToken
      ? undefined
      : new Response("Unauthorized", { status: 401 }),
});
```

생성 코드는 선언된 OpenAPI 인증 요구 사항과 인증 정보 위치를 해석합니다.
애플리케이션은 사용자 식별, 접근 권한 정책, 비밀 값 조회, 서명 검증을
담당합니다.

`authenticate`는 인증이 필요한 API에서만 호출합니다. `security: []` 또는 빈
인증 대안 `{}`로 익명 접근을 허용하면 호출하지 않습니다.

잘못된 수신 입력은 타입이 지정된 값으로 처리 함수에 전달되기 전에 거부됩니다.

<span id="callback-수신"></span>

## 콜백 수신

콜백은 외부 호출 API에 선언되고, URL은 그 요청 데이터에서 정해질 수
있습니다. 예를 들어 `createTodo` 요청이 `callbackUrl`을 보내고
`statusUpdates` 콜백이 이후 그 URL로 들어올 `POST` 요청을 설명할 수 있습니다.

[생성된 콜백 처리 함수](../reference/server-api.md#createcallbackhandlers)를 구현합니다.

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

생성된 키에는 OpenAPI의 원본 `operationId`, 콜백 이름, 런타임 표현식,
HTTP 메서드가 그대로 반영됩니다. 애플리케이션의 콜백 수신 경로에 이
엔드포인트를 연결합니다.

```ts
const response =
  await callbacks.callbacks.createTodo.statusUpdates[
    "{$request.body#/callbackUrl}"
  ].POST.fetch(request);
```

런타임 표현식은 OpenAPI 계약에 그대로 유지되고, 콜백 수신 URL은
애플리케이션에서 정합니다.

<span id="complete-inbound-body-크기-제한"></span>

## 전체 수신 본문 크기 제한

생성된 웹훅과 콜백 처리 함수는 전체 수신 요청 본문 하나를
기본 8 MiB로 제한합니다. 애플리케이션의 데이터 계약이 다르면
`maxBodyBytes`를 지정합니다.

```ts
const router = createWebhookRouter(handlers, {
  routes: {
    todoCompleted: "/webhooks/todos/completed",
  },
  maxBodyBytes: 4 * 1024 * 1024,
});
```

실제 본문 스트림을 읽으면서 바이트 수를 검사하므로 `Content-Length`가 없거나
잘못돼 있어도 제한을 우회할 수 없습니다. JSON, 텍스트, 바이너리, URL 인코딩 폼,
멀티파트, 사용자 정의 미디어, 전체 데이터를 모은 순차형 본문이 제한을 넘으면
`413 Payload Too Large`를 반환합니다. `itemSchema`가 있는 스트리밍 본문에는
전체 본문 제한을 적용하지 않고 `maxStreamFrameBytes`의 프레임 단위 제한을
사용합니다.

<span id="inbound-stream-사용자-정의"></span>

## 수신 스트림 사용자 정의

OpenAPI 3.2 수신 본문에 `itemSchema`가 있으면 생성된 처리 함수는 요청
스트림을 소비하면서 검증된 항목을 받습니다. 웹훅과 콜백 옵션도 외부 호출
클라이언트와 같은 [`StreamCodec`](../reference/streaming.md#streamcodec) 모델을 사용합니다.

기본 SSE는 선언된 `itemSchema`로 이벤트 객체를 검증하며 `data`를 문자열로
보존합니다. JSON 애플리케이션 데이터는 해당 필드를 파싱하고 문자열로 변환하는
명시적 [`StreamAdapter`](../reference/streaming.md#streamadapter)를 사용합니다.

어댑터의 타입과 구현은 [스트림 어댑터 레퍼런스](../reference/streaming.md#streamadapter)를
참고하세요. 아래 `todoAdapter`는 그 방식으로 만든 애플리케이션 값 변환기입니다.

```ts
const router = createWebhookRouter(handlers, {
  routes: { todoCompleted: "/webhooks/todos/completed" },
  streamCodecs: { "text/event-stream": { adapter: todoAdapter } },
  maxStreamFrameBytes: 256 * 1024,
});
```

어댑터가 만든 값은 처리 함수에 전달되기 전에 선언된 `itemSchema` 검증과
생성된 타입에 맞는 변환을 거칩니다. 사용자 정의 순차형 미디어의 프레임 분리가
필요하면 [`StreamProtocol`](../reference/streaming.md#streamprotocol)을 사용합니다. [`maxStreamFrameBytes`](../reference/streaming.md#maxstreamframebytes)는 어댑터
적용 전의 전송 프레임 하나를 제한합니다. [`createCallbackHandlers`](../reference/server-api.md#createcallbackhandlers)도 같은
[`streamCodecs`](../reference/streaming.md#clientoptions-streamcodecs)와 프레임 크기 제한 옵션을 제공합니다.

버전별 웹훅/콜백 지원 범위는 [OpenAPI 지원 범위](../reference/capabilities.md),
생성된 수신 API는 [생성된 서버 API](../reference/server-api.md), 수신
스트림 프로토콜/어댑터 설정은 [스트리밍 API](../reference/streaming.md)에서
확인할 수 있습니다.
