# OpenAPI Webhook 수신

Generated server add-on을 애플리케이션 경계에서 사용하는 예제입니다. OpenAPI
contract가 inbound Webhook을 설명하고, openapi-sdkgen이 typed router를
생성하며, host application이 실제 공개 route와 runtime을 선택합니다.

```text
webhook-app/
  openapi.yaml
  src/generated/api/
  src/worker.ts
```

## 1. Webhook 선언

OpenAPI 3.1과 3.2는 root Webhook Object를 지원합니다. Public contract는
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

## 2. Server add-on 생성

```sh
pnpm exec openapi-sdkgen generate \
  --input ./openapi.yaml \
  --target typescript \
  --with server \
  --output ./src/generated/api
```

Base target에는 outbound client가 그대로 생성되고, `--with server`가
`./generated/api/server/` 아래에 Webhook/Callback artifact를 추가합니다.

## 3. Generated handler 구현

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

Host가 listener/runtime과 실제 공개 route를 소유합니다. Generated router는
OpenAPI request decoding, validation, typed handler input, response encoding을
담당합니다.

Router/options lookup은 [생성된 서버 API](../reference/server-api.md), 버전별
지원 범위는 [OpenAPI 지원 범위](../reference/capabilities.md#webhook과-callback)를
참고하세요.
