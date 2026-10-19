# 웹훅 수신

<span id="_1-webhook-선언"></span>
<span id="_1-웹훅-선언"></span>
<span id="_2-server-add-on-생성"></span>
<span id="_2-서버-확장-생성"></span>
<span id="_3-generated-handler-구현"></span>
<span id="_3-생성된-처리-함수-구현"></span>
<span id="_4-로컬에서-요청과-응답-확인"></span>
<span id="openapi-webhook-수신"></span>
<span id="openapi-웹훅-수신"></span>

OpenAPI 웹훅에서 타입이 있는 요청 처리기를 생성합니다.
[전체 예제 ZIP](/downloads/typescript-webhook-app.zip)에는 명세, 수신 코드, 인증과
요청 검증을 실행해보는 코드가 들어 있습니다. 실행 방법은 포함된 README를 따르세요.

## 처리기 연결 {#handler}

예제의 `openapi.yaml`을 `--with server`로 생성합니다. 다음 코드는 `todoCompleted`
웹훅을 처리하며, `expectedToken`은 애플리케이션의 인증 설정에서 가져옵니다.

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

호스트의 HTTP 처리기에서 `router.fetch(request)`를 호출하고 반환된 `Response`를 전달합니다.
SDK는 요청 해석과 스키마 검증을 수행하며, 애플리케이션은 수신 경로와 인증 정책을 정합니다.

## 인증과 검증 {#validation}

이 명세는 `x-webhook-token` 헤더를 요구합니다. 인증이 통과하고 본문이 유효하면
처리기가 실행되어 `204`를 반환합니다. 잘못된 인증은 `401`, 잘못된 본문은 `400`입니다.
`authenticate`는 인증이 필요한 작업에서만 호출되므로 명세의 `security` 선언을 유지하세요.

라우터 옵션과 콜백 수신은 [서버 API](../reference/server-api.md),
요청 크기 제한은 [웹훅과 콜백 가이드](../guide/server.md)를 참고하세요.
