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
`./src/generated/api/server/` 아래에 웹훅/콜백 생성 파일을 추가합니다.

<span id="_3-generated-handler-구현"></span>

## 3. 생성된 처리 함수 구현

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

호스트 애플리케이션은 HTTP 서버와 실행 환경, 실제 공개 경로를 구성합니다.
생성된 라우터는 OpenAPI 요청 디코딩, 검증, 타입이 지정된 처리 함수 입력, 응답 인코딩을
담당합니다.

## 4. 로컬에서 요청과 응답 확인

[시작하기](../guide/getting-started.md)의 Node.js 22+와 ESM 설정을 사용합니다.
기존 빌드 도구를 사용해도 됩니다. 아래 `tsc` 명령으로 확인할 때만 컴파일러가 필요합니다.
위 명세를 별도 프로젝트의 `openapi.yaml`, 코드를 `src/worker.ts`로 저장하고
생성 명령을 실행하세요. `src/demo.ts`에 다음 코드를 넣습니다.

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

처리 함수가 `todo-1 true`를 출력하고, 응답 상태는 차례로 `204`, `401`, `400`입니다.
HTTP 서버를 띄우지 않고 Fetch `Request`로 라우터의 인증·검증·처리 경로를 확인합니다.
예제 토큰은 로컬 검사용입니다. 배포할 때는 비밀 저장소에서 토큰을 제공하고
호스트의 HTTP 처리 함수에서 `router.fetch(request)`를 호출하세요.

`authenticate`는 명세에 인증 요구 사항이 있고 익명 접근을 허용하지 않는 API에서만
호출됩니다. 위 `security` 선언도 함께 사용해야 합니다.

라우터와 옵션은 [생성된 서버 API](../reference/server-api.md), 버전별
지원 범위는 [OpenAPI 지원 범위](../reference/capabilities.md#webhook과-callback)를
참고하세요.
