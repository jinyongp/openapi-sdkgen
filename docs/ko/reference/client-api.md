# 생성된 클라이언트 API

이 페이지의 코드는 API 형태를 설명하는 예시입니다. API 이름·매개변수·미디어·인증은
사용하는 명세에서 생성됩니다. [시작하기](../guide/getting-started.md)의 작은 Todo 명세에는
아래 확장 기능이 모두 포함되어 있지 않습니다.

TypeScript SDK는 용도에 따라 가져올 경로가 나뉩니다. 일반 API 호출은
`./generated/api`를 사용합니다.

| 경로 | 용도 |
| --- | --- |
| `./generated/api` | API 호출, 생성 타입, 오류, Link, 스트림 |
| `./generated/api/clients/<name>/index.js` | 설정한 클라이언트의 API와 타입 |
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
  body: { title: "Write documentation" },
});
```

이름이나 경로 선택자 타입의 충돌로 리소스 메서드를 만들 수 없으면
`SDKGEN-W513`을 보고합니다. 해당 API는 `$operations`나 `$routes`로 호출하세요.

### `$routes`

HTTP 메서드와 OpenAPI 경로를 기준으로 호출합니다.

```ts
const todos = await api.$routes["GET /todos"]();
```

### `$operations`

OpenAPI에 선언된 `operationId`로 호출합니다.

```ts
const todos = await api.$operations["listTodos"]();
```

### `.raw()`

모든 생성된 API 호출에는 `.raw()`가 있습니다. 해석된 본문과 함께
상태 코드, 응답 헤더, 요청 메타데이터, 선택된 콘텐츠 타입, 원본 Fetch
`Response`를 반환합니다.

```ts
const result = await api.$operations.createTodo.raw({
  body: { title: "Write documentation" },
});

result.status;
result.headers;
result.response;
```

일반 호출에서는 응답 본문이 이미 소비됩니다. 선언된 스트리밍
응답에서 소비되지 않은 본문이 필요하면 별도의 `.raw()` 요청을 사용합니다.

#### 성공 응답의 상태와 미디어 타입

raw 응답의 상태 타입은 성공 범위인 `200`~`299`로 제한됩니다. 같은 미디어에
`200: Item`, `default: Problem`이 선언되어 있으면 `status === 200`일 때
본문이 `Item`으로 좁혀집니다. `202`에서는 여전히 `Problem`을 반환할 수 있습니다.
같은 미디어의 `2XX` 선언이 있으면 모든 성공 상태를 담당하므로 `default` 본문은
HTTP 오류 계약에 남고 일반 호출·raw·스트림·페이지 조회의 결과 타입에서는 빠집니다.

미디어가 다르거나 와일드카드 또는 본문 없는 응답이 선언된 경우에는 실제로 선택될
수 있는 분기를 유지합니다. raw의 `contentType`은 정규화된 실제 응답 헤더 값입니다.
와일드카드 선언에는 문자열이나 템플릿 리터럴 타입을 사용합니다.

<span id="security-requirement"></span>

## 인증 요구 사항

API에 OpenAPI 인증 대안이 여러 개라면 생성된 요청 옵션이
`securityRequirement`를 요구합니다. 인증 요구 사항이 하나이면 자동 선택되며, 빈
인증 요구 사항이 다른 대안과 함께 있으면 `"anonymous"`로 표현됩니다.

[인증 가이드](../guide/transport.md)의 `userAuth`와 `serviceAuth` 스키마를
`createTodo`에 추가했다면:

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

유효한 인증 요구 사항 ID는 생성된 TypeScript 타입에 포함됩니다. 인증 정보를
동적으로 가져와야 한다면 `securityProvider`를 사용합니다. 전체 인증
모델과 예시는 [인증, 전송, 스트림](../guide/transport.md)을 참고하세요.

<span id="request-headers"></span>

## 요청 헤더

선언된 헤더는 `headerParams`에 생성됩니다. Fetch가 제어하는 헤더는 호출자
입력에서 선택 사항이며 전송 여부는 실행 중인 Fetch가 결정합니다. 자세한
사용법은 [요청 헤더](../guide/transport.md#request-headers)에서 확인할 수 있습니다.

## Link

`$links`는 OpenAPI Link에서 생성한 후속 호출입니다. 원래 호출의 `.raw()` 결과를
전달하면 응답 값을 다음 요청에 사용할 수 있습니다.
예제는 [OpenAPI Link로 후속 호출하기](../guide/files-links-streams.md#openapi-links)를 참고하세요.

외부 `operationRef`의 대상은 `$ref`로 불러오고 원래 API 경로에 연결해야 합니다.
Link만 선언하면 외부 문서를 불러오지 않습니다. 예를 들어
`operationRef: ./target.json#/paths/~1items/get`은
`$ref: ./target.json#/paths/~1items`로 Path Item을 `/items`에 연결했을 때 해석할 수 있습니다.
대상을 찾을 수 없거나 여러 대상이 겹치면 `SDKGEN-W509`로 알리고 해당 도우미를
생략합니다. 요청 주소는 대상의 서버 설정 또는 Link에 지정한 서버를 사용합니다.

## 스트리밍

생성된 순차형 미디어 API, 시작·종료 동작, 사용자 정의 프로토콜·어댑터,
요청 데이터 공급, 프레임 크기 제한은 전용 [스트리밍 API](./streaming.md) 레퍼런스에
정리되어 있습니다.

<span id="errors"></span>

## 오류 처리

```ts
import {
  isAPIError,
  isErrorCategory,
  isErrorCode,
  isOperationHTTPError,
  TransportErrorCode,
} from "./generated/api";
```

- `isAPIError(error)`: 생성된 API 오류인지 확인
- `isErrorCode(error, code)`: 정확한 오류 코드 확인
- `isErrorCategory(error, category)`: 오류 범주 확인
- `isOperationHTTPError(error, method)`: 해당 생성 메서드에서 발생한 HTTP 오류인지
  확인하고 선언된 상태·본문·미디어 타입으로 좁히기
- `TransportErrorCode`: 전송 과정에서 발생할 수 있는 오류 코드

인증 요구 사항 선택 오류는 `SECURITY_REQUIREMENT_REQUIRED`와
`SECURITY_REQUIREMENT_INVALID`를 사용합니다. 인증 정보 획득 및 적용 오류는
`SECURITY_CREDENTIALS_REQUIRED`와 `SECURITY_CREDENTIALS_INVALID`를 사용합니다.

`isOperationHTTPError`는 지정한 API에서 발생한 선언된 HTTP 오류를 확인합니다.
raw·스트리밍 호출에도 사용할 수 있으며 전송 오류에는 일치하지 않습니다.
`OperationHTTPError<typeof method>`로 같은 오류 타입을 추출할 수 있습니다.

### 선언된 HTTP 오류 좁히기 {#declared-http-errors}

아래 예제에는 `todoID` 경로 매개변수와 JSON `404` 응답의 문자열 `message`를
선언한 `getTodo`가 필요합니다. 시작하기 명세에는 없는 API이므로 추가하고 다시
생성한 뒤 사용하세요.
[확장 Todo 예제 명세](/examples/todo-types.json)에 해당 선언이 들어 있습니다.

```ts
import { isOperationHTTPError } from "./generated/api";

try {
  await api.$operations.getTodo({ path: { todoID: "todo-1" } });
} catch (error: unknown) {
  if (!isOperationHTTPError(error, api.$operations.getTodo)) throw error;
  if (error.status === 404) console.log(error.data.message);
}
```
<span id="openapi-메타데이터"></span>

<span id="metadata-migration"></span>

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
