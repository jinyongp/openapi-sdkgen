# 생성된 클라이언트 API

TypeScript SDK는 용도에 따라 가져올 경로가 나뉩니다. 일반 API 호출은
`./generated/api`를 사용합니다.

| 경로 | 용도 |
| --- | --- |
| `./generated/api` | API 호출, 생성 타입, 오류, Link, 스트림 |
| `./generated/api/metadata` | 원본 OpenAPI 파일과 버전 확인 |

Inbound Webhook/Callback import는
[생성된 서버 API](./server-api.md)를 참고하세요.

::: details Node ESM으로 실행할 때

Node에서 컴파일된 파일을 실행한다면 `.js` 파일 경로를 명시하세요.

```ts
import { createClient } from "./generated/api/index.js";
```
:::

<span id="client"></span>

## 클라이언트

### createClient

```ts
import { createClient } from "./generated/api";

const api = createClient({
  baseURL: "https://api.example.test/v1",
});
```

실제 설정 흐름은 [인증·전송·스트림](../guide/transport.md)에서 확인하세요.

### ClientOptions

| 옵션 | 용도 |
| --- | --- |
| `baseURL` | 명시적인 absolute API base URL |
| `origin` | relative OpenAPI Server URL을 해석할 origin |
| `server` | generated OpenAPI Server 선택 |
| `codecs` | 선언된 custom media type의 complete-value codec |
| `streamCodecs` | sequential protocol/adapter의 media-type 기본값. [스트리밍 API](./streaming.md#clientoptions-streamcodecs) 참고 |
| `transport` | 명시적인 capability를 가진 host transport |
| `fetch` | Fetch 구현 또는 wrapper |
| `headers` | 기본 request header |
| `authorization` | 기본 complete Authorization header 값 |
| `credentials` | 기본 Fetch credentials mode |
| `securityProvider` | 선택된 OpenAPI security requirement의 dynamic credential 획득 |
| `timeoutMS` | 기본 request timeout |
| `maxStreamFrameBytes` | 기본 sequential frame limit. [스트리밍 API](./streaming.md#maxstreamframebytes) 참고 |

<span id="request-options"></span>

## 요청 옵션

Generated call은 적용 가능한 경우 다음 per-request option을 받습니다.

| 옵션 | 용도 |
| --- | --- |
| `baseURL` | 한 호출의 API base URL override |
| `signal` | caller-owned cancellation signal |
| `timeoutMS` | client 기본값을 override하는 request timeout |
| `headers` | 호출자가 추가하는 non-contract header |
| `authorization` | client 기본값을 override하는 Authorization header |
| `accept` | 선언된 response media type 선택 |
| `streamCodec` | 한 호출의 sequential protocol/adapter override. [스트리밍 API](./streaming.md#requestoptions-streamcodec) 참고 |
| `csrfToken` | generated `X-CSRF-Token` header 값 |
| `requestID` | generated `X-Request-Id` header 값 |
| `credentials` | 한 호출의 Fetch credentials mode |
| `multipartHeaders` | 선언된 multipart part 추가 header |
| `multipartContentTypes` | multipart part media type 선택 |
| `maxStreamFrameBytes` | 한 호출의 sequential frame limit |

`path`, `query`, `headerParams`, `body` 같은 operation-specific input section은
OpenAPI operation에서 생성됩니다. `RequestOptions`는 호출 단위 동작을 설정합니다.

## TypeScript 타입

생성된 SDK는 component, route, operation, 요청 영역, 파라미터를 기준으로 타입을
제공합니다. 전체 타입 API와 예제는
[생성된 TypeScript 타입](./typescript-types.md)에서 확인하세요.

## API 호출

<span id="resource-methods"></span>

### 리소스 메서드

일반적인 애플리케이션 코드에서는 경로를 바탕으로 생성된 리소스 메서드를
사용합니다.

```ts
const todo = await api.todos.create({
  body: { title: "문서 작성" },
});
```

### `$routes`

HTTP 메서드와 OpenAPI 경로를 기준으로 호출합니다.

```ts
const todos = await api.$routes["GET /todos"]({
  query: { limit: 20 },
});
```

### `$operations`

OpenAPI에 선언된 `operationId`로 호출합니다.

```ts
const todos = await api.$operations["listTodos"]({
  query: { limit: 20 },
});
```

### `.raw()`

모든 generated operation call에는 `.raw()`가 있습니다. Decoded body와 함께
status, response header, request metadata, 선택된 content type, 원본 Fetch
`Response`를 반환합니다.

```ts
const result = await api.$operations.getTodo.raw({
  path: { todoID: "todo-1" },
});

result.status;
result.headers;
result.response;
```

일반 decoded call에서는 Fetch body가 이미 소비됩니다. 선언된 streaming
response에서 소비되지 않은 body가 필요하면 별도의 `.raw()` 요청을 사용합니다.

## Security Requirement

Operation에 OpenAPI security 대안이 여러 개라면 생성된 요청 옵션이
`securityRequirement`를 요구합니다. Requirement가 하나이면 자동 선택되며, 빈
requirement가 다른 대안과 함께 있으면 `"anonymous"`로 표현됩니다.

Todo operation이 `userAuth`와 `serviceAuth` 중 하나를 허용한다면:

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

유효한 requirement ID는 생성된 TypeScript 타입에 포함됩니다. Credential을
동적으로 가져와야 한다면 `securityProvider`를 사용합니다. 전체 security
모델과 예시는 [인증, 전송, 스트림](../guide/transport.md)을 참고하세요.

<span id="request-headers"></span>

## 요청 헤더

선언된 헤더는 `headerParams`에 생성됩니다. Fetch가 제어하는 헤더는 호출자
입력에서 선택 사항이며 전송 여부는 실행 중인 Fetch가 결정합니다. 자세한
사용법은 [요청 헤더](../guide/transport.md#request-headers)에서 확인할 수 있습니다.

## Link

`$links`에는 OpenAPI Link Object에서 생성된 타입 안전 후속 호출이 있습니다.
각 helper는 Link runtime expression을 해석하는 데 필요한 원본 response context를
전달합니다.

전체 예시는 [OpenAPI Link 따라가기](../guide/client.md#openapi-links)를
참고하세요.

## 스트리밍

Generated sequential-media API, lifecycle, protocol/adapter extension point,
request source, frame limit는 전용 [스트리밍 API](./streaming.md) 레퍼런스에
정리되어 있습니다.

## 오류 처리

```ts
import {
  isAPIError,
  isErrorCategory,
  isErrorCode,
  TransportErrorCode,
} from "./generated/api";
```

- `isAPIError(error)`: 생성된 API 오류인지 확인
- `isErrorCode(error, code)`: 정확한 오류 코드 확인
- `isErrorCategory(error, category)`: 오류 범주 확인
- `TransportErrorCode`: 전송 과정에서 발생할 수 있는 오류 코드

Security Requirement 선택 오류는 `SECURITY_REQUIREMENT_REQUIRED`와
`SECURITY_REQUIREMENT_INVALID`를 사용합니다. 인증 정보 획득 및 적용 오류는
`SECURITY_CREDENTIALS_REQUIRED`와 `SECURITY_CREDENTIALS_INVALID`를 사용합니다.

## OpenAPI 메타데이터

```ts
import { openapi } from "./generated/api/metadata";

openapi.document;
openapi.version;
openapi.versionLine;
```

`openapi.document`에서 SDK 생성에 사용한 OpenAPI 파일의 내용을 확인할 수
있습니다.
