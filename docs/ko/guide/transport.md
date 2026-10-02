# 인증, 전송, 스트림

생성된 클라이언트는 Fetch로 요청을 보냅니다. 대부분의 애플리케이션은 기본 URL과
인증 정보를 설정하면 됩니다. 쿠키 저장소, 제한된 응답 헤더 접근, 상호 TLS 인증처럼
실행 환경에 따른 기능이 필요하면 사용자 정의 전송 구현을 사용합니다.

<span id="일반-bearer-credential-전달"></span>

## 일반 Bearer 인증 정보 전달

API에 하나의 Bearer 인증 정보만 필요하다면 완성된 `Authorization` 헤더
값을 전달합니다.

```ts
const api = createClient({
  baseURL: "https://api.example.test",
  authorization: "Bearer example-token",
});
```

로그인, 토큰 갱신, 인증 정보 저장은 애플리케이션에서 관리합니다.

<span id="여러-openapi-security-대안-중-선택"></span>

## 여러 OpenAPI 인증 대안 중 선택

OpenAPI는 하나의 API에 여러 인증 요구 사항 객체를 선언할 수
있습니다. 적용 가능한 인증 요구 사항이 여러 개라면 생성된 요청 옵션은
[`securityRequirement`](../reference/client-api.md#security-requirement)를 요구하며, 애플리케이션이 어느 대안을 충족할지
선택합니다.

Todo 수정 API가 `userAuth`와 `serviceAuth` 중 하나를 허용한다면:

```ts
await api.$operations.updateTodo(
  {
    path: { todoID: "todo-1" },
    body: { completed: true },
  },
  {
    securityRequirement: "userAuth",
    authorization: "Bearer example-token",
  },
);
```

허용되는 인증 요구 사항의 식별자는 생성된 TypeScript 유니온 타입에 포함되며 자동 완성과
정적 타입 검사로 확인할 수 있습니다. 유효한 인증 요구 사항이 하나면
SDK가 자동 선택합니다. 빈 인증 요구 사항이 다른 대안과 함께 있으면 익명 접근을
뜻하며 ID는 `"anonymous"`입니다.

<span id="securityprovider로-credential-로드"></span>

## `securityProvider`로 인증 정보 로드

선택된 인증 요구 사항에 맞춰 인증 정보를 동적으로 가져와야 한다면
[`securityProvider`](../reference/client-api.md#clientoptions)를 사용합니다.

```ts
const api = createClient({
  baseURL: "https://api.example.test",
  securityProvider: async ({ operation, requirement, origin }) => {
    if (requirement.id === "serviceAuth") {
      return {
        serviceAuth: {
          kind: "api-key",
          value: await getTodoServiceToken(operation, origin),
        },
      };
    }

    return {
      userAuth: {
        kind: "http-bearer",
        token: await getTodoUserToken(operation, origin),
      },
    };
  },
});
```

`securityProvider`는 호출할 API 정보인 `operation`, 선택된 인증 요구 사항인
`requirement`, 요청할 출처인 `origin`을 받습니다. 클라이언트는 반환된 인증 정보의
형태를 검사하고 OpenAPI에 선언된 위치에 적용합니다.

API 키, HTTP Basic/Bearer, OAuth2, OpenID Connect, mTLS를 지원합니다. OAuth
로그인 화면, 토큰 갱신, 영구 인증 정보 저장은 호스트 애플리케이션의
책임입니다.

<span id="cookie-인증"></span>

## 쿠키 인증

브라우저가 관리하는 쿠키 인증을 사용한다면
[`ClientOptions.credentials`](../reference/client-api.md#clientoptions)를 설정합니다.

```ts
const api = createClient({
  baseURL: "https://api.example.test",
  credentials: "include",
});
```

자동으로 전송되는 쿠키는 브라우저와 Fetch 정책이 결정합니다.

브라우저 밖에서 쿠키 저장소가 필요하면 해당 기능을 제공하는 전송 구현을
사용합니다.

<span id="request-headers"></span>

<span id="선언된-요청-header-전달"></span>

## 선언된 요청 헤더 전달

OpenAPI 매개변수로 선언된 헤더는 [`headerParams`](../reference/client-api.md#request-headers)에 생성됩니다.

```ts
await api.$operations.createTodo({
  headerParams: { "Idempotency-Key": requestID },
  body: {
    title: "문서 작성",
    callbackUrl: "https://app.example.test/todo-status",
  },
});
```

`Origin`, `Host`, `Cookie`, `Sec-*` 같은 헤더는 실행 중인 Fetch 환경이 제어하며,
호출자가 지정한 값을 적용할 수 있는지도 전송 구현이 결정합니다.

<span id="custom-transport-설정"></span>

## 사용자 정의 전송 구현 설정

[`ClientOptions.transport`](../reference/client-api.md#clientoptions)는 Fetch와 호환되는 함수와 지원하는 추가 기능을 제공합니다.

```ts
const api = createClient({
  baseURL: "https://api.example.test",
  transport: {
    fetch: undiciFetch,
    capabilities: {
      cookieJar: true,
      readableResponseHeaders: ["set-cookie"],
      mutualTLS: true,
    },
  },
});
```

실행 환경에 특화된 요청 동작도 전송 구현에 둘 수 있습니다.

```ts
const api = createClient({
  baseURL: "https://api.example.test",
  transport: {
    async fetch(input, init = {}) {
      const headers = new Headers(init.headers);
      headers.set("Origin", trustedOrigin);
      return fetch(input, { ...init, headers });
    },
  },
});
```

<span id="요청-취소와-timeout"></span>

## 요청 취소와 시간 제한

[요청 옵션](../reference/client-api.md#request-options)에는 `AbortSignal`과 시간 제한을 전달할 수 있습니다.

```ts
const controller = new AbortController();

const todos = await api.todos.list(
  { query: { completed: false } },
  { signal: controller.signal, timeoutMS: 5_000 },
);
```

같은 요청 옵션은 생성된 API, 경로, 리소스, Link, 스트림 호출에서
사용할 수 있습니다.

## 스트리밍 동작

openapi-sdkgen은 OpenAPI 3.0.x, 3.1.x, 3.2.x를 모두 지원합니다. 지원하는 순차형
콘텐츠 타입은 모든 버전에서 `schema`로 전체 데이터를 모은 값을 표현할 수 있습니다.
OpenAPI 3.2에서는 `itemSchema`로 각 항목의 타입을 지정해 순차적으로 보내거나
받을 수 있습니다.

정확한 버전 및 미디어 타입 계약은
[스트리밍 API](../reference/streaming.md#openapi-version-support)를 참고하세요.

`itemSchema`가 있는 API에는 `.stream()`이 추가되고
[`OperationStream<T>`](../reference/streaming.md#operationstream)을 반환합니다.
항목별로 보내는 요청 본문은
[`StreamSource<T>`](../reference/streaming.md#streaming-request-body)를
사용하므로 `AsyncIterable<T>`와 웹의 `ReadableStream<T>`을 모두 전달할 수
있습니다. 순차형 미디어 타입 객체에 `schema`와 `itemSchema`가 함께
있으면 생성된 요청 타입이 전체 값과 항목별 스트림을 모두 받습니다.

[`maxStreamFrameBytes`](../reference/streaming.md#maxstreamframebytes)는
애플리케이션 값으로 변환하기 전의 전송 프레임, 레코드, 멀티파트의 각 부분 하나의 크기를
제한합니다.

<span id="기본-protocol에-adapter-적용"></span>

### 기본 프로토콜에 어댑터 적용

기본 SSE는 이벤트 객체를 반환하고 `data`를 문자열로 보존합니다.
JSON 애플리케이션 데이터를 파싱하고 인코딩할 때 문자열로 변환하려면
[`StreamAdapter<Frame, Item>`](../reference/streaming.md#streamadapter)를
지정합니다. 아래 어댑터는 기본 SSE 파서를 재사용하면서 이름이 `todo`인 이벤트도
선택합니다.

```ts
import type { ServerSentEvent, StreamAdapter } from "./generated/api";

const todoAdapter: StreamAdapter<ServerSentEvent, TodoEvent> = {
  async *decode(events) {
    for await (const event of events) {
      if (event.event !== "todo") continue;
      yield JSON.parse(event.data) as TodoEvent;
    }
  },
  async *encode(items) {
    for await (const item of items) {
      yield { event: "todo", data: JSON.stringify(item) };
    }
  },
};

const api = createClient({
  baseURL,
  streamCodecs: {
    "text/event-stream": { adapter: todoAdapter },
  },
});
```

한 번의 요청에서 클라이언트 미디어 타입 기본값을 바꾸려면
[`streamCodec`](../reference/streaming.md#requestoptions-streamcodec)을
사용합니다. 어댑터 결과는 API의 `itemSchema`로 검증한 뒤 생성된 타입에 맞게
변환합니다.

<span id="사용자-정의-framing"></span>

### 사용자 정의 프레임 처리

사용자 정의 순차형 미디어의 프레임 구분 규칙을 구현하려면
[`StreamProtocol<Frame>`](../reference/streaming.md#streamprotocol)을
사용합니다. [`StreamCodec`](../reference/streaming.md#streamcodec)은 사용자 정의
프로토콜과 선택적인 어댑터를 함께 구성할 수 있습니다.

프로토콜에는 크기가 제한된 `StreamReader`와 `StreamContext.maxFrameBytes`가
전달되므로 기본 프로토콜과 같은 취소/프레임 크기 계약을 따릅니다.

순회를 끝내거나 `abort()`를 호출하거나 `toReadableStream()`을 취소하면
본문을 해제합니다. 외부 `AbortSignal`과 시간 제한도 같은 시작·종료 동작을
사용합니다. 생성된 클라이언트는 Server-Sent Events 재접속·재전송을 자동
수행하지 않습니다. 스트림의 시작·종료 동작과 설정은
[스트리밍 API](../reference/streaming.md)를 참고하세요.

<span id="integration-예제"></span>

### 연동 예제

외부 서비스나 프레임워크와 연결하는 방법은 예제에서 다룹니다.
AI SDK를 사용하는 서버와 생성된 클라이언트를 사용하는 애플리케이션을 나누는 방법은
[생성된 클라이언트로 AI 스트리밍 API 사용](../examples/ai-streaming.md)을
참고하세요.
