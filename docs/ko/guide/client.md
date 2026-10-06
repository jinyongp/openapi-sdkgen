# 생성된 클라이언트 사용

생성된 TypeScript 소스는 OpenAPI 문서에 정의된 API를 세 가지 방식으로 호출할 수
있습니다.

- [`api.todos.create()` 같은 리소스 메서드](../reference/client-api.md#resource-methods)는 일반 애플리케이션 코드를 읽기 쉽게 만듭니다.
- [`$routes`](../reference/client-api.md#routes)는 HTTP 메서드와 OpenAPI 경로를 정확한 식별자로 사용합니다.
- [`$operations`](../reference/client-api.md#operations)는 문서에 선언된 `operationId`를 사용합니다.

세 호출 방식은 같은 API를 사용합니다. 호출하는 코드에서 가장
이해하기 쉬운 방식을 선택하면 됩니다.

애플리케이션에서 API의 일부만 사용한다면
[필요한 API만 불러오기](./selective-client.md)를 참고하세요. 선택형 진입점은
사용할 코드를 먼저 준비한 뒤 클라이언트를 구성합니다. 아래의 기존 루트 진입점은
전체 클라이언트를 만듭니다.

## 클라이언트 설정

[`createClient`](../reference/client-api.md#createclient)로 생성된 클라이언트 하나를 설정합니다.

```ts
import { createClient } from "./generated/api";

const api = createClient({
  baseURL: "https://api.example.test/v1",
});
```

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
  body: { title: "문서 작성" },
});

const todos = await api.todos.list({
  query: { completed: false },
});
```

[`$routes`](../reference/client-api.md#routes)는 HTTP 메서드와 OpenAPI 경로를 기준으로 API를 호출합니다.
`operationId`가 없는 API도 이 방식으로 호출할 수 있습니다.

```ts
const todos = await api.$routes["GET /todos"]({
  query: { completed: false },
});
```

`operationId`가 애플리케이션에서 사용할 안정적인 이름이라면 [`$operations`](../reference/client-api.md#operations)를
사용합니다.

```ts
const todos = await api.$operations.listTodos({
  query: { completed: false },
});
```

<span id="raw-로-status와-header-확인"></span>

## `.raw()`로 상태 코드와 헤더 확인

일반 호출은 성공 응답의 생성된 값을 반환합니다. 상태 코드, 변환된 응답 헤더,
선택된 콘텐츠 타입, 원본 Fetch `Response`까지 필요하다면 [`.raw()`](../reference/client-api.md#raw)를
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

`.raw()`의 반환 타입은 상태 코드별로 구분되므로
`result.status`를 확인하면 TypeScript가 해당 응답 필드를 좁힐 수 있습니다.

## 변환된 응답 객체 사용

스키마에 따라 변환되는 응답 객체는 `Object.prototype`을 사용하는 일반 JavaScript
객체입니다. 중첩 객체, `.raw().data`, 선언된 오류 본문, multipart 결과, 스트림과
페이지네이션 항목에도 같은 규칙이 적용됩니다. `instanceof Object`로 확인하거나
일반 객체 리터럴과 엄격하게 비교할 수 있습니다. 이전에 생성된 클라이언트는 이
변환에서 프로토타입이 없는 객체를 사용했으므로, 새 동작이 필요하면 SDK를 다시 생성하세요.

`__proto__`, `constructor`, `hasOwnProperty` 같은 JSON 필드 이름도 객체 자신의
데이터 속성으로 보존됩니다. 선언된 필드가 상속된 메서드 이름을 가릴 수 있으므로
필드의 존재 여부는 `Object.hasOwn(value, name)`으로 확인하세요. 스키마 변환에서
객체를 만드는 지점에 적용되는 규칙이며, 스키마가 내부 구조를 지정하지 않는 값과
`Blob`, 타입 배열, 스트림 같은 객체는 기존 참조와 표현을 유지합니다.

응답용 출력 타입도 수정할 수 있습니다. 필드와 중첩 객체를 편집하거나 배열에
항목을 추가하고, 튜플과 맵의 값을 바꿀 수 있습니다. OpenAPI의 `readOnly`는 요청용
타입에서 해당 필드를 제외한다는 뜻이며, 반환된 필드의 수정을 막지는 않습니다.
`writeOnly` 필드는 출력 타입에서 제외됩니다. 입력 타입은 계속 readonly 배열과
객체를 받습니다. enum의 리터럴 제약, 클라이언트 목록과 메타데이터의 기존 제약도 유지됩니다.

기존 코드를 옮길 때는 응답 모의 데이터와 서버 핸들러가 출력 타입에 맞는 수정 가능한
배열을 반환하는지 확인하세요. 중첩 값까지 readonly인 테스트 데이터는 편집 가능한
복사본이 필요할 수 있습니다. 부모 객체만 펼쳐 복사하면 중첩 객체와 배열은 계속
공유되며, `structuredClone`도 TypeScript에서는 인수의 타입을 그대로 반환합니다.
반환된 DTO를 편집하는 것만으로 요청이 전송되거나 서버의 데이터가 저장되지는 않습니다.

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
    title: "문서 작성",
    callbackUrl: "https://app.example.test/todo-status",
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

## 성공 응답의 상태와 미디어 타입

raw 응답의 상태 타입은 성공 범위인 `200`~`299`로 제한됩니다. 같은 미디어에
`200: Item`, `default: Problem`이 선언되어 있으면 `status === 200`일 때
본문이 `Item`으로 좁혀집니다. `202`에서는 여전히 `Problem`을 반환할 수 있습니다.
같은 미디어의 `2XX` 선언이 있으면 모든 성공 상태를 담당하므로 `default` 본문은
HTTP 오류 계약에 남고 일반 호출·raw·스트림·pagination의 결과 타입에서는 빠집니다.

미디어가 다르거나 와일드카드 또는 본문 없는 응답이 선언된 경우에는 실제로 선택될
수 있는 분기를 유지합니다. raw의 `contentType`은 정규화된 실제 응답 헤더 값입니다.
와일드카드 선언에는 문자열이나 템플릿 리터럴 타입을 사용해 서버가 와일드카드
헤더를 반환한 것처럼 표현하지 않습니다.

## 오퍼레이션의 HTTP 오류 타입 좁히기

`isOperationHTTPError(error, api.$operations.operationID)`로 해당 오퍼레이션에
선언된 HTTP 오류인지 확인할 수 있습니다. `.raw()`, `.stream()`, 경로 값이 바인딩된
리소스 메서드도 전달할 수 있습니다. 가드 안에서는 `status`에 따라 디코딩된
`data` 타입이 좁혀집니다. 미디어가 여럿이면 `contentType`으로 구분하고, 필수 오류
코드가 리터럴로 선언되어 있으면 코드에 맞는 `details` 타입도 얻을 수 있습니다.

가드는 실제 SDK 호출에서 나온 오류인지 확인하고 현재 본문을 다시 검증합니다.
직접 만든 오류, 다른 오퍼레이션의 오류, 전송 오류, 잘못되었거나 나중에 변조된
본문은 통과하지 못합니다. `contentType`은 선택된 선언의 값입니다. 실제 응답 헤더는
`error.response.headers`에서 확인합니다. `OperationHTTPError<typeof method>`로
이 오류 타입을 추출할 수도 있습니다. 기존 `APIError<Code, Details>`와 `isErrorCode`는
문서에 없는 코드를 포함해 계속 사용할 수 있습니다.

## 다음 문서

- [인증, 전송, 스트림](./transport.md): 인증 정보, 전송 기능, 취소,
  사용자 정의 Fetch 동작
- [생성된 클라이언트 API](../reference/client-api.md): 제공되는 API와 오류 처리 함수
- [생성된 TypeScript 타입](../reference/typescript-types.md): 생성된 계약에서
  요청·응답 타입 추출
- [스트리밍 API](../reference/streaming.md): 스트림의 시작·종료 동작, 요청 데이터 공급,
  코덱, 프레임 크기 제한 레퍼런스
