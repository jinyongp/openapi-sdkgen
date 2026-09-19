# 생성된 클라이언트 사용

생성된 TypeScript 소스는 같은 OpenAPI operation을 세 가지 방식으로 호출할 수
있습니다.

- `api.todos.create()` 같은 리소스 메서드는 일반 애플리케이션 코드를 읽기
  쉽게 만듭니다.
- `$routes`는 HTTP 메서드와 OpenAPI 경로를 정확한 식별자로 사용합니다.
- `$operations`는 문서에 선언된 `operationId`를 사용합니다.

세 호출 표면은 같은 OpenAPI operation을 사용합니다. 호출하는 코드에서 가장
이해하기 쉬운 방식을 선택하면 됩니다.

## 클라이언트 설정

```ts
import { createClient } from "./generated/api";

const api = createClient({
  baseURL: "https://api.example.test/v1",
});
```

`baseURL`을 생략하면 적용 가능한 OpenAPI Server Object를 사용합니다. 더 구체적인
operation server가 path server보다 우선하고, path server는 root server보다
우선합니다.

인증 정보와 사용자 정의 Fetch 동작은 클라이언트 설정이나 요청 옵션에 둡니다.
자세한 내용은 [인증, 전송, 스트림](./transport.md)을 참고하세요.

## Todo operation 호출

생성된 resource tree가 애플리케이션의 표현과 잘 맞는다면 리소스 메서드를
사용하기 좋습니다.

```ts
const created = await api.todos.create({
  body: { title: "문서 작성" },
});

const todos = await api.todos.list({
  query: { completed: false },
});
```

`$routes`는 HTTP method와 OpenAPI path를 기준으로 operation을 호출합니다.
`operationId`가 없는 operation도 이 방식으로 호출할 수 있습니다.

```ts
const todos = await api.$routes["GET /todos"]({
  query: { completed: false },
});
```

`operationId`가 애플리케이션에서 사용할 안정적인 이름이라면 `$operations`를
사용합니다.

```ts
const todos = await api.$operations.listTodos({
  query: { completed: false },
});
```

## `.raw()`로 status와 header 확인

일반 호출은 성공 응답의 생성된 값을 반환합니다. 상태 코드, 변환된 응답 header,
선택된 content type, 원본 Fetch `Response`까지 필요하다면 `.raw()`를
사용합니다.

```ts
const result = await api.$operations.getTodo.raw({
  path: { todoID: "todo-1" },
});

if (result.status === 200) {
  console.log(result.data.title);
  console.log(result.response.headers.get("etag"));
}
```

생성된 raw response는 status를 기준으로 구분되므로
`result.status`를 확인하면 TypeScript가 해당 응답 필드를 좁힐 수 있습니다.

## 요청 미디어 타입 선택

하나의 request body에 여러 미디어 타입이 선언되면 생성된 body 입력은
discriminated value가 됩니다. `contentType` 필드로 보낼 representation을
명시합니다.

Todo 첨부 API가 binary와 text를 함께 받는다면 다음처럼 binary를 선택할 수
있습니다.

```ts
await api.$operations.uploadTodoAttachment({
  path: { todoID: "todo-1" },
  body: {
    contentType: "application/octet-stream",
    value: new Uint8Array([1, 2, 3]),
  },
});
```

선택한 content type은 해당 operation이 선언한 값이어야 합니다. 사용자 정의
media codec도 선언된 media type을 기준으로 선택합니다.

## OpenAPI Link 따라가기

OpenAPI Link는 한 응답의 값을 사용해 다음 operation을 호출하는 방법을
정의합니다. Todo 생성 응답에 `getTodo`라는 Link가 있다면 생성된 `$links`
helper가 원본 응답 context를 후속 호출에 전달합니다.

```ts
const created = await api.$operations.createTodo.raw({
  body: {
    title: "문서 작성",
    callbackUrl: "https://app.example.test/todo-status",
  },
});

const todo = await api.$links.createTodo.getTodo(created);
```

Link 호출 인자를 지정하면 Link Object에서 유도한 값보다 해당 인자가 우선합니다.
필수 runtime expression을 해석할 수 없으면 호출이 실패합니다.

## 스트리밍 응답 읽기

OpenAPI 3.2
[`itemSchema`](https://spec.openapis.org/oas/v3.2.0.html#media-type-object)가 있는
operation에는 operation, exact route, 생성된 resource 호출에
[`.stream(...)`](../reference/client-api.md#link와-스트림)이 추가됩니다. OpenAPI
3.0/3.1/3.2의 차이는
[OpenAPI 지원 범위](../reference/capabilities.md#지원-openapi-버전)에서 확인할 수
있습니다.

```ts
const stream = api.$operations.watchTodos.stream({
  query: { cursor: "0" },
});

for await (const event of stream) {
  console.log(event.todoID, event.completed);
}
```

`.stream()`은
[`OperationStream<T>`](../reference/typescript-types.md#스트림-타입)을
반환합니다. 요청은 실제 순회나
`stream.response` 접근 시 시작되고 Fetch backpressure를 유지합니다.
`stream.response`에서는 status, header, content type, request metadata를
확인할 수 있습니다.

```ts
const metadata = await stream.response;
console.log(metadata.status, metadata.request.id);
```

`stream.abort()`, 요청 옵션의 `AbortSignal`, timeout으로 작업을 중단할 수
있습니다. `stream.toReadableStream()`은 같은 단일 소비자 source를 Web
Streams API로 연결합니다.

원본 Fetch response body가 필요하면 별도의 `.raw()` 호출을 사용합니다.
Server-Sent Events는 `data`, `event`, `id`, `retry`를 그대로 보존하며
replay와 reconnect 정책은 애플리케이션에서 관리합니다.

## 스트리밍 request body 전송

OpenAPI 3.2 request body에 `itemSchema`가 있으면
[`StreamSource<T>`](../reference/typescript-types.md#스트림-타입)를
사용합니다. Async iterable과 Web `ReadableStream`을 같은 생성 타입으로
전달할 수 있습니다.

```ts
async function* todoEvents() {
  yield { todoID: "todo-1", completed: false };
  yield { todoID: "todo-1", completed: true };
}

await api.$operations.publishTodoEvents({
  body: todoEvents(),
});
```

Sequential media에 `schema`만 있으면 complete schema value를 전달합니다.
`schema`와 `itemSchema`가 함께 있으면 complete value와
`StreamSource<T>` 중 필요한 방식을 선택할 수 있습니다.

## 다음 문서

- [인증, 전송, 스트림](./transport.md): credential, transport capability, 취소,
  사용자 정의 Fetch 동작
- [생성된 클라이언트 API](../reference/client-api.md): export와 오류 helper 레퍼런스
- [생성된 TypeScript 타입](../reference/typescript-types.md): 생성된 계약에서
  요청·응답 타입 추출
