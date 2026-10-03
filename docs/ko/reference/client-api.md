# 생성된 클라이언트 API

TypeScript SDK는 용도에 따라 가져올 경로가 나뉩니다. 일반 API 호출은
`./generated/api`를 사용합니다.

| 경로 | 용도 |
| --- | --- |
| `./generated/api` | API 호출, 생성 타입, 오류, Link, 스트림 |
| `./generated/api/clients/<name>/index.js` | 설정한 클라이언트의 API와 타입. 다음 릴리스 |
| `./generated/api/metadata` | OpenAPI 버전과 선택적으로 포함한 원문 확인 |

웹훅·콜백을 수신하는 모듈의 경로는
[생성된 서버 API](./server-api.md)를 참고하세요.

::: details Node ESM으로 실행할 때

Node에서 컴파일된 파일을 실행한다면 `.js` 파일 경로를 명시하세요.

```ts
import { createClient } from "./generated/api/index.js";
```
:::

<span id="client"></span>

## 클라이언트

### `createClient`

```ts
import { createClient } from "./generated/api";

const api = createClient({
  baseURL: "https://api.example.test/v1",
});
```

실제 설정 흐름은 [인증·전송·스트림](../guide/transport.md)에서 확인하세요.

[이름 있는 클라이언트](../guide/named-clients.md)도 같은 `ClientOptions`를 받는
동기 `createClient(options)`를 제공합니다. 반환 타입과 메서드는 해당 클라이언트의
선택 목록을 반영합니다. URL·인증·요청 설정을 따로 쓰려면 별도 인스턴스를 만드세요.

### `ClientOptions`

| 옵션 | 용도 |
| --- | --- |
| `baseURL` | API의 절대 기본 URL |
| `origin` | OpenAPI의 상대 서버 URL을 해석할 기준 출처 |
| `server` | 생성된 OpenAPI 서버 선택 |
| `transport` | 지원 기능을 명시한 사용자 정의 전송 구현 |
| `fetch` | Fetch 구현 또는 이를 감싸는 함수 |
| `headers` | 기본 요청 헤더 |
| `authorization` | 기본 `Authorization` 헤더의 완성된 값 |
| `credentials` | 기본 Fetch 인증 정보 전송 모드 |
| `securityProvider` | 선택된 인증 요구 사항에 맞춰 인증 정보를 가져오는 함수 |
| `timeoutMS` | 기본 요청 시간 제한 |
| `codecs` | 선언된 사용자 정의 미디어 타입의 전체 값을 처리하는 코덱 |
| `streamCodecs` | 순차형 프로토콜/어댑터의 미디어 타입 기본값. [스트리밍 API](./streaming.md#clientoptions-streamcodecs) 참고 |
| `maxStreamFrameBytes` | 기본 순차형 미디어의 프레임 크기 제한. [스트리밍 API](./streaming.md#maxstreamframebytes) 참고 |

<span id="request-options"></span>

## 요청 옵션

생성된 호출은 적용 가능한 경우 다음 요청별 옵션을 받습니다.

| 옵션 | 용도 |
| --- | --- |
| `baseURL` | 한 호출의 API 기본 URL 덮어쓰기 |
| `accept` | 선언된 응답 미디어 타입 선택 |
| `headers` | 호출자가 추가하는 명세에 선언되지 않은 헤더 |
| `authorization` | 클라이언트 기본값을 대신할 `Authorization` 헤더 |
| `credentials` | 한 호출의 Fetch 인증 정보 전송 모드 |
| `csrfToken` | 생성된 `X-CSRF-Token` 헤더 값 |
| `requestID` | 생성된 `X-Request-Id` 헤더 값 |
| `signal` | 호출자가 관리하는 취소 신호 |
| `timeoutMS` | 클라이언트 기본값을 대신할 요청 시간 제한 |
| `multipartHeaders` | 선언된 멀티파트의 각 부분 추가 헤더 |
| `multipartContentTypes` | 멀티파트의 각 부분 미디어 타입 선택 |
| `streamCodec` | 한 호출의 순차형 프로토콜/어댑터 덮어쓰기. [스트리밍 API](./streaming.md#requestoptions-streamcodec) 참고 |
| `maxStreamFrameBytes` | 한 호출의 순차형 미디어의 프레임 크기 제한 |

`path`, `query`, `headerParams`, `body` 같은 API별 입력 영역은
OpenAPI 문서의 API에서 생성됩니다. `RequestOptions`는 호출 단위 동작을 설정합니다.

## TypeScript 타입

생성된 SDK는 구성 요소, 경로, API, 요청 영역, 파라미터를 기준으로 타입을
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

여러 API가 하나의 경로 선택자를 공유하더라도 공개 선택자 입력 타입이
같으면 리소스 메서드를 유지합니다. 스키마 제약과 경로 직렬화는 각
API에 그대로 남으므로 리소스 값을 바인딩한다고 계약을 합치거나 느슨하게
만들지 않습니다. 선택자 타입 자체가 호환되지 않으면 리소스 호출 방식만 생략하고
정확한 `$operations` / `$routes` 호출은 유지하며 `SDKGEN-W513`을 보고합니다.

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

모든 생성된 API 호출에는 `.raw()`가 있습니다. 해석된 본문과 함께
상태 코드, 응답 헤더, 요청 메타데이터, 선택된 콘텐츠 타입, 원본 Fetch
`Response`를 반환합니다.

```ts
const result = await api.$operations.getTodo.raw({
  path: { todoID: "todo-1" },
});

result.status;
result.headers;
result.response;
```

일반 호출에서는 응답 본문이 이미 소비됩니다. 선언된 스트리밍
응답에서 소비되지 않은 본문이 필요하면 별도의 `.raw()` 요청을 사용합니다.

<span id="security-requirement"></span>

## 인증 요구 사항

API에 OpenAPI 인증 대안이 여러 개라면 생성된 요청 옵션이
`securityRequirement`를 요구합니다. 인증 요구 사항이 하나이면 자동 선택되며, 빈
인증 요구 사항이 다른 대안과 함께 있으면 `"anonymous"`로 표현됩니다.

Todo API가 `userAuth`와 `serviceAuth` 중 하나를 허용한다면:

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

유효한 인증 요구 사항 ID는 생성된 TypeScript 타입에 포함됩니다. 인증 정보를
동적으로 가져와야 한다면 `securityProvider`를 사용합니다. 전체 인증
모델과 예시는 [인증, 전송, 스트림](../guide/transport.md)을 참고하세요.

<span id="request-headers"></span>

## 요청 헤더

선언된 헤더는 `headerParams`에 생성됩니다. Fetch가 제어하는 헤더는 호출자
입력에서 선택 사항이며 전송 여부는 실행 중인 Fetch가 결정합니다. 자세한
사용법은 [요청 헤더](../guide/transport.md#request-headers)에서 확인할 수 있습니다.

## Link

`$links`는 OpenAPI Link 객체에 따라 다음 API를 호출하는 함수를 제공합니다.
원본 응답 정보를 사용해 런타임 표현식을 해석하고, 후속 호출의 입력 타입도 검사합니다.

다른 문서를 가리키는 `operationRef`는 대상 API를 `$ref`로 이미 불러온 경우에
지원합니다. 원래 경로를 유지해야 하며, 대상의 원문 위치가 하나로 정해져야 합니다.
예를 들어 `operationRef: ./target.json#/paths/~1items/get`을 사용하려면
`$ref: ./target.json#/paths/~1items`로 `GET /items`를 `/items`에 포함하세요.
대상이 없거나 경로가 달라졌거나 같은 원문이 여러 경로에 연결되어 모호하면
`SDKGEN-W509`를 보고하고 해당 후속 호출 함수를 생략합니다.

Link 해석에는 입력 문서의 참조 허용 목록·잠금 파일·오프라인 캐시 정책이
적용됩니다. Link만 `target.json`을 가리키는 경우에는 추가 문서 요청 없이
`SDKGEN-W509`를 보고합니다. 이 조건은 허용된 참조를 통해 읽은 문서 범위에서
후속 호출을 생성하기 위한 경계입니다.

후속 호출은 대상 API·경로·문서의 서버 설정 또는 Link에 명시된 서버를 기본 URL로
사용합니다. 상속한 상대 서버 URL은 대상 문서의 HTTP URL을 기준으로 해석합니다.

전체 예시는 [OpenAPI Link 따라가기](../guide/client.md#openapi-links)를
참고하세요.

## 스트리밍

생성된 순차형 미디어 API, 시작·종료 동작, 사용자 정의 프로토콜·어댑터,
요청 데이터 공급, 프레임 크기 제한은 전용 [스트리밍 API](./streaming.md) 레퍼런스에
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

인증 요구 사항 선택 오류는 `SECURITY_REQUIREMENT_REQUIRED`와
`SECURITY_REQUIREMENT_INVALID`를 사용합니다. 인증 정보 획득 및 적용 오류는
`SECURITY_CREDENTIALS_REQUIRED`와 `SECURITY_CREDENTIALS_INVALID`를 사용합니다.

<span id="openapi-메타데이터"></span>

## OpenAPI 메타데이터

모든 SDK에서 입력 문서의 OpenAPI 버전을 확인할 수 있습니다. API 설명과
생성 타입은 기본 설정에서도 제공됩니다.

```ts
import { openapi } from "./generated/api/metadata";

openapi.version;
openapi.versionLine;
```

SDK에서 원문을 읽으려면 [`--with metadata`](./cli.md#metadata-addon)를 지정하세요.
원문의 설명이나 확장 필드를 읽는 문서 도구·스크립트에서 사용할 수 있습니다.

```ts
import { openapi } from "./generated/api/metadata";

console.log(openapi.document.info.title);
```

`openapi.document`에는 입력 JSON·YAML 문서 전체를 디코딩한 값이 들어갑니다.
API를 선택해 생성해도 제외한 API의 원문은 함께 포함됩니다. 외부 `$ref`는
원래 경로를 유지하며, YAML의 주석과 서식은 원본 파일에서 확인할 수 있습니다.

루트 API를 선택해 생성하면 메타데이터 모듈에서 공개 경로와 내부 Link 의존성을
담은 `generationSelection`을 제공합니다. 이름 있는 클라이언트를 설정하면
클라이언트별 API 구성을 담은 `generationClients`도 제공합니다. 두 값 모두
`--with metadata`를 지정했을 때만 포함됩니다.

### SDK 재생성 시 이전 방법 {#metadata-migration}

다음 메이저 버전부터는 `--with metadata`를 지정하면 SDK에 원문이 포함됩니다.
`openapi.document`, `generationSelection`, `generationClients`를 사용하는 코드는
재생성 전에 이 옵션을 추가하거나 설정
파일에 `addons = ["metadata"]`를 넣으세요. 이미 생성한 SDK가 제공하는 API는 유지됩니다.
