# 생성된 서버 API

이 페이지의 코드는 API 형태를 설명하는 예시입니다. API 이름·매개변수·미디어·인증은
사용하는 명세에서 생성됩니다. [시작하기](../guide/getting-started.md)의 작은 Todo 명세에는
아래 확장 기능이 모두 포함되어 있지 않습니다.

TypeScript 서버 확장은 `--with server`로 생성합니다. OpenAPI 웹훅과
콜백을 수신하기 위한 Fetch 기반 진입점을 제공합니다. HTTP 서버 리스너,
프레임워크 연결, 경로 연결, 인증 정책은 애플리케이션이 담당합니다.

설정 흐름은 [웹훅과 콜백 수신](../guide/server.md)을 참고하세요.

<span id="import-경로"></span>

## 모듈 경로

| 모듈 경로 | 용도 |
| --- | --- |
| `./generated/api/server/webhooks` | 웹훅 라우터와 생성된 웹훅 처리 함수 타입 |
| `./generated/api/server/callbacks` | 콜백 엔드포인트와 생성된 콜백 처리 함수 타입 |
| `./generated/api/server/runtime` | 지원하는 임의 스키마를 처리하는 범용 수신 디코딩·검증·응답 함수 |

## 웹훅

### `createWebhookRouter`

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
Fetch 기반 라우터를 반환합니다. 생성된 `WebhookHandlers` 타입에 처리 함수의
중첩 구조와 각 함수의 요청·응답 타입이 정의되어 있습니다.

### `WebhookRouterOptions`

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

- `routes`: 생성된 웹훅 식별자를 애플리케이션이 정한 경로에 연결합니다.
- `authenticate`: 수신 요청을 호스트가 허용하거나 거부합니다.
- `codecs`: 선언된 사용자 정의 미디어 전체 값을 처리합니다.
- `streamCodecs`: 미디어 타입별 수신 순차형 프로토콜/어댑터를 설정합니다.
- `maxBodyBytes`: 전체 수신 요청 본문 하나의 전체 크기를 제한합니다.
  기본값은 8 MiB입니다. JSON, 텍스트, 바이너리, URL 인코딩 폼, 멀티파트, 사용자 정의
  미디어, 전체 데이터를 모은 순차형 본문이 크기 제한을 넘으면 `413 Payload Too Large`를
  반환합니다. `Content-Length`는 조기 거부에만 사용합니다.
- `maxStreamFrameBytes`: 애플리케이션 값으로 변환하기 전 수신 프레임 하나의 크기를 제한합니다.
  스트리밍 요청 본문에는 `maxBodyBytes` 전체 제한을 적용하지 않습니다.

스트림 코덱 타입과 처리 순서는 [스트리밍 API](./streaming.md)를 참고하세요.

<span id="callback"></span>

## 콜백

### `createCallbackHandlers`

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

`createCallbackHandlers(handlers, options?)`는 생성된 Fetch 엔드포인트를
반환합니다. 콜백 URL을 결정하는 표현식은 OpenAPI 문서에 선언합니다.
애플리케이션에서 실제 수신 URL과 경로를 정하고 해당 엔드포인트를 연결합니다.

생성된 콜백 식별자는 상황에 따라 API 식별자, 정확한 경로,
구성 요소의 콜백 식별자 기준으로 제공됩니다.

### `CallbackHandlerOptions`

```ts
interface CallbackHandlerOptions {
  readonly pathParams?: {
    // generated callback-specific path parameter values
  };
  readonly authenticate?: Authenticate;
  readonly codecs?: Readonly<Record<string, MediaCodec<unknown>>>;
  readonly streamCodecs?: Readonly<Record<string, StreamCodec>>;
  readonly maxBodyBytes?: number;
  readonly maxStreamFrameBytes?: number;
}
```

`maxBodyBytes`, `streamCodecs`, `maxStreamFrameBytes`는 웹훅과 같은
수신 본문·스트림 모델을 사용합니다. [스트리밍 API](./streaming.md)를 참고하세요.

## 인증

생성된 서버 진입점은 OpenAPI 인증 후보를 애플리케이션이 관리하는
`authenticate` 함수에 제공합니다. 인증 정보 검증 방식은 호스트가 결정하며,
요청을 거부할 때 `Response`를 반환할 수 있습니다.

HTTP 서버와 프레임워크의 인증·세션 시스템은 애플리케이션에서 구성합니다.

`authenticate`는 인증이 필요한 API에서만 호출합니다. `security: []`나 빈 대안
`{}`로 익명 접근을 허용하면 생략하며, 인증이 필요한 API에 함수가 없으면 `401`을
반환합니다. 명세의 인증 선언과 기대 응답은
[실행 가능한 웹훅 예제](../examples/webhook-server.md)에서 확인하세요.

<span id="framework-연결"></span>

## 프레임워크 연결

프레임워크 어댑터는 Fetch 요청·응답 동작을 보존하면 됩니다.

```ts
const response = await router.fetch(request);
```

라우터 연결, 프레임워크 요청을 Fetch `Request`로 변환하는 작업, 결과
Fetch `Response` 반환은 애플리케이션이 담당합니다.
