# 시작하기

[`openapi-sdkgen`](../reference/cli.md)은 OpenAPI 3.x 문서에서 애플리케이션이 소유하는 TypeScript
클라이언트 소스를 생성합니다. 이 가이드에서는 작은 Todo API를 정의하고 SDK를
애플리케이션 소스 안에 생성한 뒤 첫 요청까지 호출합니다.

## 1. CLI 설치

애플리케이션 저장소에서는 CLI를 개발 의존성으로 설치하면 다른 개발 도구와 함께
생성기 버전도 고정할 수 있습니다.

```sh
pnpm add -D openapi-sdkgen
pnpm exec openapi-sdkgen --version
```

이 페이지의 명령은 `pnpm exec openapi-sdkgen`을 기준으로 설명합니다. Homebrew나
GitHub Release 실행 파일로 설치했다면 앞의 `pnpm exec` 없이
`openapi-sdkgen`을 사용하면 됩니다.

일회성 실행에는 `pnpm dlx openapi-sdkgen ...`을 사용할 수 있습니다.

macOS와 Linux에서는 Homebrew로 설치할 수도 있습니다.

```sh
brew install jinyongp/tap/openapi-sdkgen
```

## 2. Todo OpenAPI 문서 만들기

다음 내용을 `openapi.yaml`로 저장합니다.

```yaml
openapi: 3.2.0
info:
  title: Todo API
  version: 1.0.0
paths:
  /todos:
    get:
      operationId: listTodos
      responses:
        "200":
          description: Todo list
          content:
            application/json:
              schema:
                type: object
                required: [items]
                properties:
                  items:
                    type: array
                    items:
                      $ref: "#/components/schemas/Todo"
    post:
      operationId: createTodo
      requestBody:
        required: true
        content:
          application/json:
            schema:
              type: object
              required: [title]
              properties:
                title:
                  type: string
      responses:
        "201":
          description: Created todo
          content:
            application/json:
              schema:
                $ref: "#/components/schemas/Todo"
components:
  schemas:
    Todo:
      type: object
      required: [id, title, completed]
      properties:
        id:
          type: string
        title:
          type: string
        completed:
          type: boolean
```

두 operation에 안정적인 `operationId`를 주고, 요청과 응답 스키마를 선언했습니다.
이 정보가 생성된 TypeScript 타입과 클라이언트 API의 기준이 됩니다.

## 3. 애플리케이션 소스 안에 생성

최초 실행에서는 새 출력 디렉터리를 지정합니다.

```sh
pnpm exec openapi-sdkgen generate \
  --input ./openapi.yaml \
  --target typescript \
  --output ./src/generated/api
```

생성된 디렉터리는 client, type, source runtime을 포함한 일반 애플리케이션
소스입니다. 기존 TypeScript 컴파일러나 번들러가 나머지 코드와 함께 빌드합니다.

생성기가 소유한 파일은 CLI로 다시 생성합니다. OpenAPI 문서가 바뀌어 같은
디렉터리를 갱신할 때는 [`--incremental`](../reference/cli.md#fresh-incremental-and-check-modes)을 사용합니다. 안전한 재생성과 CI
검증 흐름은 [SDK 생성과 검증](./generate.md)에서 설명합니다.

## 4. 클라이언트 만들기

```ts
import { createClient } from "./generated/api";

const api = createClient({
  baseURL: "https://api.example.test/v1",
});
```

OpenAPI 문서에 사용할 수 있는 Server Object가 선언되어 있다면 [`baseURL`](../reference/client-api.md#client)을
생략할 때 그 서버 정의를 사용합니다.

::: details 컴파일된 코드를 Node ESM으로 실행할 때

Vite, Next.js, Nuxt 같은 번들러는 생성 디렉터리의 진입점을 찾습니다. Node
ESM으로 컴파일된 코드를 실행할 때는 `index.js` 파일을 명시합니다.

```ts
import { createClient } from "./generated/api/index.js";
```
:::

## 5. Todo API 호출

일반 애플리케이션 코드에서는 리소스 메서드가 짧고 읽기 쉽습니다.

```ts
const created = await api.todos.create({
  body: { title: "문서 작성" },
});

const todos = await api.todos.list();
```

모든 operation은 HTTP method/path로도 호출할 수 있고, `operationId`가 있으면
[`$operations`](../reference/client-api.md#operations)에서도 사용할 수 있습니다.

```ts
await api.$routes["GET /todos"]();
await api.$operations.listTodos();
```

다음으로 [SDK 생성과 검증](./generate.md)에서 증분 생성, `--check`, 인증이
필요한 입력, 원격 참조를 확인하세요. 응답, Link, stream 등 생성된 호출 API는
[생성된 클라이언트 사용](./client.md)에서 이어서 설명합니다.
