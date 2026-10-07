# 파일, Link, 스트림 사용

API가 파일 전송, 응답에 이어지는 호출, 연속 데이터 처리를 제공할 때 참고하는
가이드입니다. 일반 JSON 호출은 [클라이언트 사용](./client.md)에서 먼저 확인하세요.

아래 예제는 시작하기의 두 API만으로 실행되지 않습니다. 각 절에서 설명하는
요청·응답과 매개변수를 명세에 선언하고 SDK를 다시 생성한 뒤 사용하세요.
`api`는 실제 API 서버 주소로 설정한 클라이언트입니다.

| 하려는 작업 | 필요한 명세 |
| --- | --- |
| 바이너리 또는 텍스트 전송 | `uploadTodoAttachment`와 두 요청 미디어 타입, `todoID` 경로 매개변수 |
| 파일 업로드 | `uploadAttachment`의 멀티파트 본문에 `file` 속성 |
| 생성한 항목 다시 조회 | 생성 응답의 `getTodo` Link와 조회 대상 API |
| 이벤트 순차 수신 | `watchTodos`의 응답 `itemSchema`와 `cursor` 쿼리 |
| 이벤트 순차 전송 | `publishTodoEvents`의 요청 `itemSchema` |

실행 가능한 스트리밍 명세와 사용 코드는
[AI 스트리밍 예제](../examples/ai-streaming.md)에 있습니다.

## 요청 미디어 타입 선택

하나의 요청 본문에 여러 미디어 타입이 선언되면 `contentType` 필드로 보낼 형식을
선택합니다. 각 형식의 데이터는 `value`에 넣습니다.

Todo 첨부 API가 바이너리와 텍스트를 함께 받는다면 다음처럼 바이너리를 선택할 수
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

선택한 콘텐츠 타입은 해당 API가 선언한 값이어야 합니다. 사용자 정의
미디어 코덱도 선언된 미디어 타입을 기준으로 선택합니다.

## 멀티파트 파일 업로드

원시 파일 파트는 `File`, `Blob`, `ArrayBuffer`, `ArrayBufferView`, 텍스트 문자열을
받습니다. 문자열은 UTF-8로 인코딩합니다. `File`은 파일명을 함께 보내고, 바이트 뷰는
선택한 바이트 범위만 보냅니다. base64로 인코딩된 스키마 값은 문자열로 유지됩니다.

멀티파트 본문에 `file` 속성이 선언된 API라면:

```ts
await api.$operations.uploadAttachment({
  body: { file: new File([new Uint8Array([0, 127, 255])], "attachment.bin") },
});
```

인코딩 객체의 `contentType`이 파트의 `contentMediaType`보다 우선합니다.
원시 파일 파트의 기본 미디어 타입은 `application/octet-stream`입니다.
같은 스키마를 JSON 본문에서 사용하면 JSON 입력 타입을 유지합니다.

<span id="openapi-links"></span>

## OpenAPI Link 따라가기

OpenAPI Link는 한 응답의 값을 사용해 다음 API를 호출하는 방법을
정의합니다. Todo 생성 응답에 `getTodo`라는 Link가 있다면 생성된
[`$links`](../reference/client-api.md#link)가 원본 응답 정보를 후속 호출에 전달합니다.

```ts
const created = await api.$operations.createTodo.raw({
  body: {
    title: "Write documentation",
  },
});

const todo = await api.$links.createTodo.getTodo(created);
```

Link 호출 인자를 지정하면 Link 객체에서 유도한 값보다 해당 인자가 우선합니다.
필수 런타임 표현식을 해석할 수 없으면 호출이 실패합니다.

## 스트리밍 응답 읽기

OpenAPI 3.2
[`itemSchema`](https://spec.openapis.org/oas/v3.2.0.html#media-type-object)가 있는
API는 `$operations`, `$routes`, 생성된 리소스 메서드에서
[`.stream(...)`](../reference/streaming.md#response-stream)이 추가됩니다. OpenAPI
3.0/3.1/3.2의 차이는
[OpenAPI 지원 범위](../reference/capabilities.md#supported-openapi-versions)에서 확인할 수
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
[`OperationStream<T>`](../reference/streaming.md#operationstream)을
반환합니다. 요청은 실제 순회나
`stream.response` 접근 시 시작됩니다. 소비자가 데이터를 읽는 속도에 맞춰 응답 본문을
읽으므로, 처리하지 않은 데이터를 계속 쌓아 두지 않습니다.
`stream.response`에서는 상태 코드, 헤더, 콘텐츠 타입, 요청 메타데이터를
확인할 수 있습니다.

```ts
const metadata = await stream.response;
console.log(metadata.status, metadata.request.id);
```

`stream.abort()`, 요청 옵션의 `AbortSignal`, 시간 제한으로 작업을 중단할 수
있습니다. `stream.toReadableStream()`은 같은 데이터 흐름을 Web Streams API로
연결합니다. 하나의 스트림은 한 곳에서만 소비할 수 있습니다.

원본 Fetch 응답 본문이 필요하면 별도의 [`.raw()`](../reference/client-api.md#raw) 호출을 사용합니다.
Server-Sent Events는 `data`, `event`, `id`, `retry`를 그대로 보존하며
재전송과 재접속 정책은 애플리케이션에서 관리합니다.

<span id="스트리밍-request-body-전송"></span>

## 스트리밍 요청 본문 전송

OpenAPI 3.2 요청 본문에 `itemSchema`가 있으면
[`StreamSource<T>`](../reference/streaming.md#streaming-request-body)를
사용합니다. 비동기 반복 가능한 객체와 웹의 `ReadableStream`을 같은 생성 타입으로
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

순차형 미디어에 `schema`만 있으면 전체 데이터를 하나의 값으로 전달합니다.
`schema`와 `itemSchema`가 함께 있으면 전체 값과
[`StreamSource<T>`](../reference/streaming.md#streaming-request-body) 중 필요한 방식을 선택할 수 있습니다.
