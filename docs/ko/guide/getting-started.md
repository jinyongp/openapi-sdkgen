# 시작하기

작은 Todo 명세에서 SDK를 생성합니다. 원하는 경우 호출 코드를 실행해 모의 응답도
읽어 볼 수 있습니다. 별도 API 서버 없이 끝까지 실행할 수 있습니다. 실제 서버 연결은
마지막 단계에서 설명합니다.

## 1. 실행 환경과 프로젝트 준비

Node.js **22 이상**과 `pnpm`이 필요합니다. Node.js 조건은 npm CLI 실행기에
적용됩니다. SDK 생성에는 TypeScript 설치가 필요하지 않습니다. 생성된 소스는
프로젝트에서 쓰는 빌드 도구로 빌드하세요. 아래 선택 단계에서는 별도 컴파일러로
호출 코드를 확인하는 방법을 보여 줍니다.

빈 디렉터리에서 시작합니다.

```sh
mkdir sdkgen-todo
cd sdkgen-todo
```

다음 내용을 `package.json`으로 저장합니다.

```json
{
  "private": true,
  "type": "module"
}
```

### npm

CLI를 프로젝트의 개발 의존성으로 설치합니다.

```sh
pnpm add -D openapi-sdkgen
pnpm exec openapi-sdkgen --version
```

### Homebrew

macOS와 Linux에서는 Homebrew로 설치할 수 있습니다.

```sh
brew install jinyongp/tap/openapi-sdkgen
```

### 실행 파일 다운로드

[GitHub Releases](https://github.com/jinyongp/openapi-sdkgen/releases)에서 운영체제에
맞는 실행 파일을 받아 `PATH`에 추가합니다.

Homebrew나 다운로드한 실행 파일을 사용하면 아래 생성 명령에서 `pnpm exec`을
빼고 `openapi-sdkgen`을 직접 실행하세요. 두 방법은 CLI 실행에 Node.js가 필요하지
않지만, 아래 선택 단계에서 애플리케이션을 실행할 때는 Node.js를 사용합니다.

## 2. Todo 명세 저장

다음 내용을 `openapi.yaml`로 저장합니다. 목록 조회와 생성, 두 API만 정의합니다.

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

## 3. SDK 생성

```sh
pnpm exec openapi-sdkgen generate \
  --input ./openapi.yaml \
  --target typescript \
  --output ./src/generated/api \
  --incremental
```

`src/generated/api/index.ts`를 비롯한 클라이언트 소스와 타입이 생성되면 성공입니다.
`--incremental`은 처음에는 디렉터리를 만들고 이후에는 안전하게 갱신합니다.
생성 파일은 CLI로 다시 생성하세요. 자세한 갱신 조건은
[SDK 생성과 검증](./generate.md)에 있습니다.

## 4. 선택: 모의 응답으로 호출 확인

SDK 생성은 3단계에서 완료됩니다. 호출도 확인하려면 프로젝트의 기존 TypeScript
환경을 사용하세요. 컴파일러가 없는 빈 프로젝트라면 아래 타입 검사와 컴파일을
위해 선택적으로 설치할 수 있습니다.

```sh
pnpm add -D typescript
```

다음 내용을 `src/demo.ts`로 저장합니다. `fetch`에 지정한 함수가 생성에는 `201`,
목록 조회에는 `200` 응답을 돌려줍니다. 이 예제는 요청 생성과 응답 해석을 확인하며,
데이터를 저장하거나 실제 네트워크 요청을 보내지는 않습니다.

```ts
import { createClient } from "./generated/api/index.js";

const todo = {
  id: "todo-1",
  title: "Write documentation",
  completed: false,
};
async function mockFetch(
  _input: RequestInfo | URL,
  init?: RequestInit,
) {
  if (init?.method === "POST") return Response.json(todo, { status: 201 });
  return Response.json({ items: [todo] }, { status: 200 });
}
const api = createClient({
  baseURL: "https://api.example.test",
  fetch: mockFetch,
});

const created = await api.todos.create({
  body: { title: "Write documentation" },
});
const todos = await api.todos.list();
console.log(created.title);
console.log(todos.items.length);
```

타입 검사와 컴파일 후 실행합니다. `ES2022`, `DOM`, `DOM.Iterable`은 생성된
코드가 사용하는 표준 API의 타입을 제공합니다.

```sh
pnpm exec tsc --strict --target ES2022 \
  --module NodeNext --moduleResolution NodeNext \
  --lib ES2022,DOM,DOM.Iterable --outDir dist src/demo.ts
node dist/demo.js
```

기대 출력:

```text
Write documentation
1
```

이 출력까지 확인했다면 SDK 생성, 호출 코드 타입 검사, 모의 응답 해석을 완료했습니다.

## 5. 실제 API 연결과 다음 작업

실제 호출에는 이 명세의 `GET /todos`, `POST /todos`를 구현한 서버가 필요합니다.
`baseURL`을 그 서버의 API 기본 주소로 바꾸고 `fetch: mockFetch`를 제거하세요.
`api.example.test`는 예시 주소입니다. 생성기는 API 서버를 실행하지 않습니다.

- 호출 방식과 오류 처리: [생성된 클라이언트 사용](./client.md)
- 명세 변경 후 갱신과 CI 검사: [SDK 생성과 검증](./generate.md)
- 토큰과 요청 시간 제한 설정: [인증·전송·스트림](./transport.md)
