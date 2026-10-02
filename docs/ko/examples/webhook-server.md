<span id="openapi-webhook-수신"></span>

# OpenAPI 웹훅 수신

생성된 서버 코드를 애플리케이션에 연결하는 예제입니다. OpenAPI
명세가 수신 웹훅을 설명하고, openapi-sdkgen이 타입이 지정된 라우터를
생성하며, 호스트 애플리케이션이 실제 공개 경로와 실행 환경을 선택합니다.

```text
webhook-app/
  openapi.yaml
  src/generated/api/
  src/worker.ts
```

<span id="_1-webhook-선언"></span>

## 1. 웹훅 선언

OpenAPI 3.1과 3.2는 문서 최상위에 선언한 웹훅을 지원합니다. 공개 명세는
애플리케이션이 소유합니다.

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

<span id="_2-server-add-on-생성"></span>

## 2. 서버 확장 생성

```sh
pnpm exec openapi-sdkgen generate \
  --input ./openapi.yaml \
  --target typescript \
  --with server \
  --output ./src/generated/api
```

기본 생성 대상에는 외부 호출 클라이언트가 그대로 생성되고, `--with server`가
`./generated/api/server/` 아래에 웹훅/콜백 생성 파일을 추가합니다.

<span id="_3-generated-handler-구현"></span>

## 3. 생성된 처리 함수 구현

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

호스트 애플리케이션은 HTTP 서버와 실행 환경, 실제 공개 경로를 구성합니다.
생성된 라우터는 OpenAPI 요청 디코딩, 검증, 타입이 지정된 처리 함수 입력, 응답 인코딩을
담당합니다.

라우터와 옵션은 [생성된 서버 API](../reference/server-api.md), 버전별
지원 범위는 [OpenAPI 지원 범위](../reference/capabilities.md#webhook과-callback)를
참고하세요.
