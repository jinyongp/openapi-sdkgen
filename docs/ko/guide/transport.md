# 인증, 전송, 스트림

생성된 클라이언트는 Fetch로 요청을 보냅니다. 대부분의 애플리케이션은 기본 URL과
인증 정보를 설정하면 됩니다. 쿠키 저장소, 제한된 응답 헤더 접근, 상호 TLS 인증처럼
실행 환경에 따른 기능이 필요하면 사용자 정의 전송 구현을 사용합니다.

<span id="일반-bearer-credential-전달"></span>

아래 일반 인증·쿠키·취소 예제는 [시작하기](./getting-started.md)의 명세를
사용합니다. 코드에서 `createClient`는 `./generated/api/index.js`에서 가져오세요.
인증 대안과 선언된 헤더 예제는 해당 절의 명세 확장이 필요합니다.

각 절의 명세 확장은 원래 시작 명세에 각각 적용하는 독립 예시입니다.
실제 요청에서는 토큰과 서버 주소를 애플리케이션의 값으로 바꾸세요.


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

`authorization`과 `headers.Authorization`은 일반적인 요청 기본값입니다.
`security: []`가 선언된 API에도 전송하며, 기본 URL의 출처로 전송 범위를
제한하지 않습니다. API별 서버가 다른 출처를 사용할 수도 있습니다. 선언된 인증
요구 사항이나 최종 출처에 따라 인증 정보를 선택하려면 클라이언트를 나누거나
`securityProvider`를 사용하세요.

<span id="여러-openapi-security-대안-중-선택"></span>

## 여러 OpenAPI 인증 대안 중 선택

OpenAPI는 하나의 API에 여러 인증 요구 사항 객체를 선언할 수
있습니다. 적용 가능한 인증 요구 사항이 여러 개라면 생성된 요청 옵션은
[`securityRequirement`](../reference/client-api.md#security-requirement)를 요구하며, 애플리케이션이 어느 대안을 충족할지
선택합니다.

시작 명세의 `createTodo`에 아래 필드를 추가하고 `--incremental`로 다시 생성합니다.
기존 요청·응답 스키마는 유지하세요.

```yaml
# Merge securitySchemes into components, and security into /todos POST.
components:
  securitySchemes:
    userAuth:
      type: http
      scheme: bearer
    serviceAuth:
      type: apiKey
      in: header
      name: x-service-token
paths:
  /todos:
    post:
      security:
        - userAuth: []
        - serviceAuth: []
```

```ts
await api.$operations.createTodo(
  {
    body: { title: "Write documentation" },
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

`getTodoServiceToken`과 `getTodoUserToken`은 애플리케이션이 구현하는 토큰 조회
함수입니다. 아래 예제는 앞 절의 두 인증 스키마를 사용하며 이 함수를 별도로 제공해야 합니다.

```ts
const api = createClient({
  baseURL: "https://api.example.test",
  securityProvider: async ({ operation, requirement, origin }) => {
    if (origin !== "https://api.example.test") {
      throw new Error("Untrusted API origin");
    }
    if (requirement.id === "serviceAuth") {
      return {
        [requirement.id]: {
          kind: "api-key",
          value: await getTodoServiceToken(operation, origin),
        },
      };
    }

    return {
      [requirement.id]: {
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

선택된 인증 요구 사항이 비어 있지 않고 기존 인증 정보로 충족되지 않을 때 제공자를
호출합니다. 익명 API나 클라이언트·요청의 인증 정보로 이미 충족된 요구 사항에서는
호출하지 않습니다. 제공자의 인증 정보를 보내도 되는 출처는 제공자에서 판단하세요.
제공자의 출처 검사는 다른 옵션이나 헤더로 전달한 인증 정보에는 적용되지 않습니다.

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

시작 명세의 `/todos` → `post`에 아래 `parameters`를 추가하고 다시 생성하세요.

```yaml
parameters:
  - name: Idempotency-Key
    in: header
    required: true
    schema:
      type: string
```

```ts
await api.$operations.createTodo({
  headerParams: { "Idempotency-Key": "request-1" },
  body: {
    title: "Write documentation",
  },
});
```

`Origin`, `Host`, `Cookie`, `Sec-*` 같은 헤더는 실행 중인 Fetch 환경이 제어하며,
호출자가 지정한 값을 적용할 수 있는지도 전송 구현이 결정합니다.

<span id="custom-transport-설정"></span>

## 사용자 정의 전송 구현 설정

[`ClientOptions.transport`](../reference/client-api.md#clientoptions)는 Fetch와 호환되는 함수와 지원하는 추가 기능을 제공합니다.

```ts
import { createClient } from "./generated/api/index.js";

async function loggingFetch(input: RequestInfo | URL, init?: RequestInit) {
  const response = await fetch(input, init);
  console.log(response.status);
  return response;
}

const api = createClient({
  baseURL: "https://api.example.test",
  transport: { fetch: loggingFetch },
});
```

`capabilities.cookieJar`, `readableResponseHeaders`, `mutualTLS`은 전송 구현이
이미 제공하는 기능을 알리는 설정입니다. 값을 지정하는 것만으로 쿠키 저장소나
클라이언트 인증서가 구성되지는 않습니다. 해당 기능은 호스트 전송 구현에서 먼저 준비하세요.

<span id="요청-취소와-timeout"></span>

## 요청 취소와 시간 제한

[요청 옵션](../reference/client-api.md#request-options)에는 `AbortSignal`과 시간 제한을 전달할 수 있습니다.

```ts
const controller = new AbortController();

const todos = await api.todos.list(
  {},
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

기본 SSE의 `data`는 문자열입니다. JSON 값 변환이나 이름별 이벤트 선택이 필요하면
[스트림 어댑터 예제](../reference/streaming.md#streamadapter)를 사용하세요.
한 요청의 설정을 바꾸려면 `streamCodec`, 클라이언트 전체에는 `streamCodecs`를 지정합니다.

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
