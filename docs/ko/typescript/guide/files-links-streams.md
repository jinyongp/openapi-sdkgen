# 파일, Link, 스트림 사용

API가 파일 전송, 응답에 이어지는 호출, 연속 데이터 처리를 제공할 때 참고하는
가이드입니다. 일반 JSON 호출은 [클라이언트 사용](/ko/typescript/guide/client)에서 먼저 확인하세요.

아래 예제는 시작하기의 두 API만으로 실행되지 않습니다. 각 절에서 설명하는
요청·응답과 매개변수를 명세에 선언하고 SDK를 다시 생성한 뒤 사용하세요.
`api`는 실제 API 서버 주소로 설정한 클라이언트입니다.

| 하려는 작업 | 필요한 명세 |
| --- | --- |
| 바이너리 또는 텍스트 전송 | `uploadTodoAttachment`와 두 요청 미디어 타입, `todoID` 경로 매개변수 |
| 파일 업로드 | `uploadAttachment`의 멀티파트 본문에 `file` 속성 |
| 생성한 항목 다시 조회 | 생성 응답의 `getTodo` Link와 조회 대상 API |
| 이벤트 순차 수신 | `watchTodos`의 응답 `itemSchema`와 `cursor` 쿼리 |
| 이벤트 순차 전송 | `importTodos`의 NDJSON 요청 `itemSchema` |

실행 가능한 스트리밍 명세와 사용 코드는
[AI 스트리밍 예제](/ko/typescript/examples/ai-streaming)에 있습니다.

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

`todo-1` 경로에 `Content-Type: application/octet-stream`과 바이트 `01 02 03`을 보냅니다. 반환값은 업로드 API의 응답 스키마에 따릅니다. 요청 미디어 타입으로 반환값이 정해지지는 않습니다.

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

서버는 파일명이 `attachment.bin`이고 내용이 바이트 `00 7f ff`인 멀티파트 `file` 파트를 받습니다. 멀티파트 경계는 Fetch가 정합니다. 반환값은 해당 API에 선언된 응답에 따릅니다.

인코딩 객체의 `contentType`이 파트의 `contentMediaType`보다 우선합니다.
원시 파일 파트의 기본 미디어 타입은 `application/octet-stream`입니다.
같은 스키마를 JSON 본문에서 사용하면 JSON 입력 타입을 유지합니다.

<span id="openapi-links"></span>

## OpenAPI Link 따라가기

OpenAPI Link는 한 응답의 값을 사용해 다음 API를 호출하는 방법을
정의합니다. Todo 생성 응답에 `getTodo`라는 Link가 있다면 생성된
[`$links`](/ko/typescript/reference/client-api#link)가 원본 응답 정보를 후속 호출에 전달합니다.

```ts
const created = await api.$operations.createTodo.raw({
  body: {
    title: "Write documentation",
  },
});

const todo = await api.$links.createTodo.getTodo(created);
```

생성 응답의 ID가 `todo-1`이면 Link는 그 값을 대상 API의 경로 매개변수로 전달할 수 있습니다. 후속 조회에서 반환되는 `todo` 값의 예시입니다.

```json
{
  "id": "todo-1",
  "title": "Write documentation",
  "completed": false
}
```

Link 호출 인자를 지정하면 Link 객체에서 유도한 값보다 해당 인자가 우선합니다.
필수 런타임 표현식을 해석할 수 없으면 호출이 실패합니다.

## 스트리밍 응답 읽기

OpenAPI 3.2
[`itemSchema`](https://spec.openapis.org/oas/v3.2.0.html#media-type-object)가 있는
API는 `$operations`, `$routes`, 생성된 리소스 메서드에서
[`.stream(...)`](/ko/typescript/reference/streaming#response-stream)이 추가됩니다. OpenAPI
3.0/3.1/3.2의 차이는
[OpenAPI 지원 범위](/ko/typescript/reference/capabilities#supported-openapi-versions)에서 확인할 수
있습니다.

```ts
const stream = api.$operations.watchTodos.stream({
  query: { cursor: "0" },
});

for await (const event of stream) {
  console.log(event.todoID, event.completed);
}
```

서버가 NDJSON으로 `{"todoID":"todo-1","completed":false}`와 `{"todoID":"todo-1","completed":true}`를 한 줄씩 보내면 다음처럼 출력됩니다.

```text
todo-1 false
todo-1 true
```

각 `event`는 변환된 객체입니다. 전체 응답이 끝날 때까지 기다리지 않고 도착한 항목부터 처리합니다.

`.stream()`은
[`OperationStream<T>`](/ko/typescript/reference/streaming#operationstream)을
반환합니다. 요청은 실제 순회나
`stream.response` 접근 시 시작됩니다. 소비자가 데이터를 읽는 속도에 맞춰 응답 본문을
읽으므로, 처리하지 않은 데이터를 계속 쌓아 두지 않습니다.
`stream.response`에서는 상태 코드, 헤더, 콘텐츠 타입, 요청 메타데이터를
확인할 수 있습니다.

```ts
const metadata = await stream.response;
console.log(metadata.status, metadata.contentType);
```

상태 코드가 `200`인 NDJSON 응답이라면 `200 application/x-ndjson`이 출력됩니다. 헤더를 받은 시점부터 메타데이터를 읽을 수 있으며 모든 항목을 읽을 때까지 기다릴 필요가 없습니다.

`stream.abort()`, 요청 옵션의 `AbortSignal`, 시간 제한으로 작업을 중단할 수
있습니다. `stream.toReadableStream()`은 같은 데이터 흐름을 Web Streams API로
연결합니다. 하나의 스트림은 한 곳에서만 소비할 수 있습니다.

원본 Fetch 응답 본문이 필요하면 별도의 [`.raw()`](/ko/typescript/reference/client-api#raw) 호출을 사용합니다.
Server-Sent Events는 `data`, `event`, `id`, `retry`를 그대로 보존하며
재전송과 재접속 정책은 애플리케이션에서 관리합니다.

<span id="스트리밍-request-body-전송"></span>

<span id="스트리밍-요청-본문-전송"></span>

## 스트리밍 요청 본문 전송 {#send-streaming-request-bodies}

클라이언트에 있는 파일을 서버로 가져오는 예제입니다. 클라이언트는 데이터를
업로드하고, 서버는 읽은 항목 수를 응답합니다.
[전체 실행 예제 ZIP](/downloads/typescript-request-streaming-app.zip)에는 서로 별도로
실행하는 HTTP 서버와 클라이언트, [가져오기 명세](/examples/todo-import-stream.json)가 있습니다.
이 API는 시작하기의 Todo 명세와 별개입니다.

클라이언트의 `todos.ndjson` 파일에는 한 줄에 항목 하나가 들어 있습니다.

```text
{"title":"Write documentation"}
{"title":"Ship the SDK"}
```

**클라이언트:** 파일을 한 줄씩 읽어 하나의 `POST /todos/import` 요청으로 보냅니다.
`readTodos`는 예제에 포함된 애플리케이션 함수입니다. 파일에서 각 줄을 읽고
JSON 객체로 바꾸어 내보내며, SDK가 생성하는 함수가 아닙니다.
SDK는 그 객체를 `itemSchema`로 검증하고 NDJSON으로 인코딩합니다.

```ts
const result = await api.$operations.importTodos({
  body: readTodos("./todos.ndjson"),
});
console.log(`Imported ${result.imported} todos`);
```

**서버:** 별도 프로세스에서 요청 본문을 한 줄씩 읽어 처리합니다.
아래는 예제의 Node HTTP 서버에서 처리 부분만 꺼낸 코드입니다.
`request`와 `response`는 서버의 HTTP 요청·응답 객체이고,
`createInterface`는 `node:readline`의 함수입니다.

```ts
let imported = 0;
for await (const line of createInterface({ input: request })) {
  const todo = JSON.parse(line);
  console.log("Received:", todo.title);
  imported++;
}
response.writeHead(200, { "content-type": "application/json" });
response.end(JSON.stringify({ imported }));
```

서버 터미널에는 다음 값이 출력됩니다. 이 예제는 항목을 저장하지 않고 출력합니다.
실제 가져오기에서는 그 자리에 저장 처리를 넣습니다.

```text
Received: Write documentation
Received: Ship the SDK
```

파일 전송이 끝나면 서버는 HTTP `200`과 JSON `{"imported":2}`를 응답합니다.
클라이언트의 `result`는 이 응답을 변환한 `{ imported: 2 }`이고, 출력은 다음과 같습니다.

```text
Imported 2 todos
```

업로드한 항목을 돌려받는 것이 아닙니다. 서버가 파일을 처리한 결과를 받습니다.
SDK 생성기는 명세에서 호출 코드를 만들 뿐, 파일이나 이벤트를 만들거나 서버를 실행하지 않습니다.

전체 파일을 배열로 모으지 않고 항목을 순서대로 보낼 수 있습니다.
항목마다 요청을 보내는 방식은 아니며, 네트워크 청크 경계도 항목 경계와 같다고 보장하지 않습니다.
스트리밍 업로드를 지원하는 Fetch 환경이 필요합니다.
업로드 중 실패하면 서버가 앞선 항목을 이미 처리했을 수 있으므로 재시도·중복 처리 정책은 서버에서 정하세요.

순차형 미디어에 `schema`만 있으면 전체 데이터를 하나의 값으로 전달합니다.
`schema`와 `itemSchema`가 함께 있으면 전체 값과
[`StreamSource<T>`](/ko/typescript/reference/streaming#streaming-request-body) 중 필요한 방식을 선택할 수 있습니다.
