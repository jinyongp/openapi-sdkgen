# 생성된 클라이언트 사용

요청을 보내고 반환된 데이터를 사용한 뒤, 호출 실패를 처리하는 방법을 설명합니다.
시작하기에서 만든 Todo SDK로 예제를 이어 갑니다.

생성된 TypeScript 소스는 OpenAPI 문서에 정의된 API를 세 가지 방식으로 호출할 수
있습니다.

- [`api.todos.create()` 같은 리소스 메서드](../reference/client-api.md#resource-methods)는 일반 애플리케이션 코드를 읽기 쉽게 만듭니다.
- [`$routes`](../reference/client-api.md#routes)는 HTTP 메서드와 OpenAPI 경로를 정확한 식별자로 사용합니다.
- [`$operations`](../reference/client-api.md#operations)는 문서에 선언된 `operationId`를 사용합니다.

세 호출 방식은 같은 API를 사용합니다. 호출하는 코드에서 가장
이해하기 쉬운 방식을 선택하면 됩니다.

이 페이지는 [시작하기](./getting-started.md)의 Todo 명세를 그대로 사용합니다.
각 호출 예제는 `src/demo.ts`에서 설정한 클라이언트로 실행합니다.
실제 API 서버가 준비됐다면 아래 설정을 사용하세요. 서버 없이 확인하려면
시작하기의 모의 `fetch` 설정을 유지하세요. `completed` 쿼리나 단건 조회 API는
이 명세에 없습니다. 필요한 API를 명세에 추가한 뒤 다시 생성해야 합니다.

## 클라이언트 설정

[`createClient`](../reference/client-api.md#createclient)로 생성된 클라이언트 하나를 설정합니다.

```ts
import { createClient } from "./generated/api/index.js";

const api = createClient({
  baseURL: "https://api.example.test",
});
```

`baseURL`은 실제 서버 주소로 바꾸세요. 예시 주소는 실행 중인 서버가 아닙니다.

[`baseURL`](../reference/client-api.md#clientoptions)을 생략하면 OpenAPI 문서에 선언된
서버 주소를 사용합니다. API별 설정, 경로별 설정, 문서 전체 설정 순으로 우선합니다.

인증 정보와 사용자 정의 Fetch 동작은 클라이언트 설정이나 요청 옵션에 둡니다.
자세한 내용은 [인증, 전송, 스트림](./transport.md)을 참고하세요.

<span id="todo-operation-호출"></span>

## Todo API 호출

생성된 리소스 트리가 애플리케이션의 표현과 잘 맞는다면 리소스 메서드를
사용하기 좋습니다.

```ts
const created = await api.todos.create({
  body: { title: "Write documentation" },
});

const todos = await api.todos.list();
```

[`$routes`](../reference/client-api.md#routes)는 HTTP 메서드와 OpenAPI 경로를 기준으로 API를 호출합니다.
`operationId`가 없는 API도 이 방식으로 호출할 수 있습니다.

```ts
const todos = await api.$routes["GET /todos"]();
```

`operationId`가 애플리케이션에서 사용할 안정적인 이름이라면 [`$operations`](../reference/client-api.md#operations)를
사용합니다.

```ts
const todos = await api.$operations.listTodos();
```

## 호출 밖에서 입력 타입 지정

요청을 별도 함수나 변수에서 준비할 때 생성된 입력 헬퍼를 사용하세요.
API가 요구하는 필드와 리터럴 선택지를 그대로 확인할 수 있습니다.

```ts
import type { OperationInput, RouteInput } from "./generated/api/index.js";

type CreateTodoInput = OperationInput<typeof api.$operations.createTodo>;
type CreateTodoBody = RouteInput<"POST /todos">["body"];
const body: CreateTodoBody = { title: "Write documentation" };
const input: CreateTodoInput = { body };
await api.$operations.createTodo(input);
```

<span id="raw-로-status와-header-확인"></span>

## `.raw()`로 상태 코드와 헤더 확인

일반 호출은 성공 응답의 생성된 값을 반환합니다. 상태 코드, 변환된 응답 헤더,
선택된 콘텐츠 타입, 원본 Fetch `Response`까지 필요하다면 [`.raw()`](../reference/client-api.md#raw)를
사용합니다.

```ts
const result = await api.$operations.createTodo.raw({
  body: { title: "Write documentation" },
});

if (result.status === 201) {
  console.log(result.data.title);
  console.log(result.response.headers.get("content-type"));
}
```

`.raw()`의 반환 타입은 상태 코드별로 구분되므로
`result.status`를 확인하면 TypeScript가 해당 응답 필드를 좁힐 수 있습니다.

## 응답 데이터 편집 {#변환된-응답-객체-사용}

반환된 객체와 배열은 로컬에서 수정할 수 있습니다. 이 수정은 서버에 저장되지
않습니다. 서버 데이터를 바꾸려면 명세에 정의한 수정 API를 호출하세요.
입출력 타입과 객체 동작은 [응답 본문 타입](../reference/typescript-types.md#response-body-types)을 참고하세요.

## 호출 실패 처리

SDK 오류를 처리할 때는 `isAPIError`로 확인하고, 처리할 수 없는 오류는 다시 던집니다.

```ts
import { isAPIError } from "./generated/api/index.js";

try {
  await api.todos.list();
} catch (error: unknown) {
  if (!isAPIError(error)) throw error;
  console.error(error.code, error.message);
}
```

명세에 오류 응답이 선언된 API에서는 `isOperationHTTPError`로 응답 본문 타입을
좁힐 수 있습니다. 사용 조건과 예제는 [오류 레퍼런스](../reference/client-api.md#errors)에 있습니다.

## 다음 작업

- 토큰, 취소, 요청 시간 제한: [인증·전송·스트림](./transport.md)
- 파일 업로드와 후속 호출, 스트리밍: [파일, Link, 스트림](./files-links-streams.md)
- 필요한 API만 사용: [선택형 클라이언트](./selective-client.md)
- 요청·응답 타입 추출: [TypeScript 타입](../reference/typescript-types.md)
