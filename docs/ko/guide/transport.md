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

지원되는 streaming 응답은 `$streams` 아래에서 `AsyncIterable`로 노출됩니다.
Iterator가 필요한 만큼 읽으면서 Fetch backpressure를 유지합니다.

순회를 일찍 끝내거나 요청을 abort하면 underlying reader도 취소됩니다. Decode
오류는 iteration 과정에서 발생합니다.

Server-Sent Events의 인증 갱신, replay cursor, 중복 event 처리, reconnect 정책은
애플리케이션에서 관리합니다.

Todo stream 예시는
[생성된 클라이언트 사용](./client.md#스트리밍-응답-읽기)에서 확인하세요.
