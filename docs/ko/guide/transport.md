# 인증, 전송, 스트림

생성된 클라이언트는 Fetch를 전송 경계로 사용합니다. 대부분의 애플리케이션은 base
URL과 인증 정보를 설정합니다. Cookie jar, 제한된 응답 header 접근, mutual TLS
같은 runtime-specific 기능에는 custom transport를 사용합니다.

## 일반 Bearer credential 전달

operation에 하나의 Bearer credential만 필요하다면 완성된 Authorization header
값을 전달합니다.

```ts
const api = createClient({
  baseURL: "https://api.example.test",
  authorization: "Bearer example-token",
});
```

로그인, token refresh, credential 저장은 애플리케이션에서 관리합니다.

## 여러 OpenAPI security 대안 중 선택

OpenAPI는 하나의 operation에 여러 Security Requirement Object를 선언할 수
있습니다. 적용 가능한 requirement가 여러 개라면 생성된 요청 옵션은
`securityRequirement`를 요구하며, 애플리케이션이 어느 대안을 충족할지
선택합니다.

Todo 수정 operation이 `userAuth`와 `serviceAuth` 중 하나를 허용한다면:

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

허용되는 requirement ID는 생성된 TypeScript union에 포함되며 autocomplete와
static checking으로 확인할 수 있습니다. 유효한 requirement가 하나면
SDK가 자동 선택합니다. 빈 requirement가 다른 대안과 함께 있으면 익명 접근을
뜻하며 ID는 `"anonymous"`입니다.

## `securityProvider`로 credential 로드

선택된 requirement에 맞춰 credential을 동적으로 가져와야 한다면
`securityProvider`를 사용합니다.

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

Provider는 resolved operation, 선택된 requirement, origin을 받습니다. 클라이언트는
반환된 credential 형태를 검사하고 OpenAPI에 선언된 security scheme 위치에
적용합니다.

API key, HTTP Basic/Bearer, OAuth2, OpenID Connect, mTLS를 지원합니다. OAuth
로그인 UX, token refresh, 영구 credential 저장은 호스트 애플리케이션의
책임입니다.

## Cookie 인증

브라우저가 관리하는 cookie 인증을 사용한다면 Fetch credentials를 설정합니다.

```ts
const api = createClient({
  baseURL: "https://api.example.test",
  credentials: "include",
});
```

전송되는 ambient cookie는 브라우저와 Fetch 정책이 결정합니다.

브라우저 밖에서 cookie jar가 필요하면 해당 기능을 제공하는 transport를
사용합니다.

## 선언된 요청 header 전달

OpenAPI parameter로 선언된 header는 `headerParams`에 생성됩니다.

```ts
await api.$operations.createTodo({
  headerParams: { "Idempotency-Key": requestID },
  body: {
    title: "문서 작성",
    callbackUrl: "https://app.example.test/todo-status",
  },
});
```

`Origin`, `Host`, `Cookie`, `Sec-*` 같은 header는 active Fetch 환경이 제어하며,
caller-provided 값을 적용할 수 있는지도 transport가 결정합니다.

## Custom transport 설정

Transport는 Fetch-compatible 함수와 지원하는 추가 capability를 제공합니다.

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

실행 환경에 특화된 요청 동작도 transport에 둘 수 있습니다.

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

## 요청 취소와 timeout

요청 옵션에는 `AbortSignal`과 timeout을 전달할 수 있습니다.

```ts
const controller = new AbortController();

const todos = await api.todos.list(
  { query: { completed: false } },
  { signal: controller.signal, timeoutMS: 5_000 },
);
```

같은 요청 옵션은 생성된 operation, route, resource, Link, stream 호출에서
사용할 수 있습니다.

## 스트리밍 동작

openapi-sdkgen은 OpenAPI 3.0.x, 3.1.x, 3.2.x를 모두 지원합니다. 알려진
sequential content type에 일반 `schema`를 선언하면 세 버전 모두 complete
buffered value로 처리할 수 있습니다. OpenAPI 3.2의 Media Type Object에는
[`itemSchema`](https://spec.openapis.org/oas/v3.2.0.html#media-type-object)가
추가되며, 이를 사용하면 [`.stream()`](../reference/client-api.md#link와-스트림)
기반 typed incremental 호출을 생성합니다.

`itemSchema`가 있는 operation의 `.stream()`은
[`OperationStream<T>`](../reference/typescript-types.md#스트림-타입)을
반환합니다. SSE, NDJSON/JSON Lines, JSON Sequence, streaming multipart
framing은 기본으로 처리합니다. 버전별 차이는
[OpenAPI 지원 범위](../reference/capabilities.md#지원-openapi-버전)에 정리되어
있습니다.

Incremental request body는
[`StreamSource<T>`](../reference/typescript-types.md#스트림-타입)를
사용하므로 `AsyncIterable<T>`와 Web `ReadableStream<T>`을 모두 전달할 수
있습니다. Sequential media에 `schema`가 있으면 complete application value를
사용할 수 있고, `schema`와 `itemSchema`가 함께 있으면 complete와 incremental
입력을 모두 지원합니다.

[`maxStreamFrameBytes`](../reference/client-api.md#link와-스트림)는 application
adapter 적용 전의 wire frame, record, multipart part 크기를 제한합니다. client
기본값과 개별 요청 옵션에서 설정할 수 있습니다.

### 기본 protocol에 adapter 적용

[`StreamAdapter<Frame, Item>`](../reference/typescript-types.md#스트림-타입)는
표준 framing 위에 application semantics를 적용합니다. 예를 들어 Todo SSE의
`data`를 application event로 변환하면서 SSE parser는 그대로 재사용할 수
있습니다.

```ts
import type {
  ServerSentEvent,
  StreamAdapter,
} from "./generated/api";

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

한 번의 요청에서 client 기본값을 바꾸려면 `streamCodec`을 지정합니다.
Adapter가 만든 값은 operation의 `itemSchema` 검증과 property projection을
거칩니다.

### AI event stream을 AI SDK UI로 연결

AI API는 text delta나 tool-input delta 같은 application event를 SSE 위에
전송하는 경우가 많습니다. 이런 provider/application protocol은
[`StreamAdapter`](../reference/typescript-types.md#스트림-타입)에 두고,
생성된 typed stream을 애플리케이션 경계에서 AI SDK UI stream으로 연결할 수
있습니다.

아래 예시는 `generate` operation의 `itemSchema`로 생성된 타입에 JSON-in-SSE를
매핑한 뒤 text delta를 AI SDK UI message stream으로 전달합니다.

```ts
import {
  createUIMessageStream,
  createUIMessageStreamResponse,
} from "ai";
import {
  createClient,
  type OperationStreamItem,
  type ServerSentEvent,
  type StreamAdapter,
} from "./generated/api";

type AiEvent = OperationStreamItem<"generate">;

const aiAdapter: StreamAdapter<ServerSentEvent, AiEvent> = {
  async *decode(events) {
    for await (const event of events) {
      if (event.data === "[DONE]") return;
      yield JSON.parse(event.data) as AiEvent;
    }
  },
  async *encode(items) {
    for await (const item of items) {
      yield { data: JSON.stringify(item) };
    }
  },
};

const api = createClient({
  baseURL,
  streamCodecs: {
    "text/event-stream": { adapter: aiAdapter },
  },
});

export async function POST() {
  const upstream = api.$operations.generate.stream({
    body: { prompt: "릴리스 노트를 요약해줘." },
  });

  const stream = createUIMessageStream({
    async execute({ writer }) {
      const id = "answer";
      writer.write({ type: "text-start", id });

      for await (const event of upstream) {
        if (event.type === "text-delta") {
          writer.write({ type: "text-delta", id, delta: event.text });
        }
      }

      writer.write({ type: "text-end", id });
    },
    onError: () => "Upstream generation failed",
  });

  return createUIMessageStreamResponse({ stream });
}
```

Tool call, reasoning, source, custom data를 어떤 AI SDK UI part로 변환할지는
애플리케이션이 결정합니다. openapi-sdkgen은 HTTP framing, adapter composition,
생성 타입, validation, lifecycle을 담당합니다. AI SDK 쪽 API는
[`createUIMessageStream`](https://ai-sdk.dev/docs/reference/ai-sdk-ui/create-ui-message-stream)과
[`createUIMessageStreamResponse`](https://ai-sdk.dev/docs/reference/ai-sdk-ui/create-ui-message-stream-response)
레퍼런스를 참고하세요.

Playground에도 **AI event stream** OpenAPI 3.2 예제를 추가했습니다.
[Playground → AI event stream](../playground.md?example=ai-event-stream)에서 바로
열 수 있습니다.

### 사용자 정의 framing

사용자 정의 sequential media의 byte framing은
[`StreamProtocol<Frame>`](../reference/typescript-types.md#스트림-타입)로
정의합니다. Protocol에는 bounded `StreamReader`와
`StreamContext.maxFrameBytes`가 전달되며 built-in protocol과 같은 취소·크기
제한을 적용받습니다.
[`StreamCodec`](../reference/typescript-types.md#스트림-타입)은 protocol과
선택적인 adapter를 함께 구성합니다.

순회를 끝내거나 `abort()`를 호출하거나 `toReadableStream()`을 cancel하면
underlying body를 해제합니다. 외부 `AbortSignal`과 timeout도 같은 lifecycle을
사용합니다. Server-Sent Events의 reconnect와 replay는 애플리케이션에서
관리합니다.

Todo stream 예시는
[생성된 클라이언트 사용](./client.md#스트리밍-응답-읽기)에서 확인하세요.
