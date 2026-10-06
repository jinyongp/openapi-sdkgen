# 생성된 TypeScript 타입

생성된 SDK의 기본 진입점은 요청, 응답, 구성 요소, 열거형 타입을 제공합니다.

생성된 클라이언트의 호출 방법은
[생성된 클라이언트 API](./client-api.md)에서 확인할 수 있습니다.

<span id="컴파일러-지원"></span>

## 컴파일러 지원

생성된 클라이언트와 서버 소스에는 **TypeScript 5.7.3 이상**과 `ES2022`, `DOM`,
`DOM.Iterable` 라이브러리가 필요합니다. 애플리케이션의 기존 TypeScript
컴파일러와 번들러로 함께 빌드합니다.

지원 확인 버전: **5.7.3, 5.9.3, 6.0.3, 7.0.2**.

현재 개발 버전은 아래 모듈 설정에서 `strict`와 `isolatedDeclarations`를 각각
켜거나 끌 수 있습니다.

| 모듈 형식 | `module` / `moduleResolution` | 설정 |
| --- | --- | --- |
| Node.js용 ESM | `NodeNext` / `NodeNext` | `package.json`에 `"type": "module"` |
| 번들러용 ESM | `ESNext` / `Bundler` | 애플리케이션의 번들러로 빌드 |
| Node.js용 CommonJS | `NodeNext` / `NodeNext` | `"type": "commonjs"`와 `verbatimModuleSyntax: false` |

`strict: false`이면 `exactOptionalPropertyTypes: false`도 지정합니다.
`isolatedDeclarations: true`로 선언 파일을 생성할 때는 `declaration: true`가 필요합니다.
루트 SDK, API 선택, 클라이언트별 코드, 서버 코드, 선택적으로 포함하는 메타데이터에
같은 설정을 적용할 수 있습니다.

## 생성 소스 타입 검사 {#source-checking}

`--typecheck`를 지정하면 애플리케이션의 컴파일러로
생성된 SDK의 구현 코드까지 타입 검사할 수 있습니다.

```sh
openapi-sdkgen generate --input ./openapi.yaml --target typescript \
  --output ./src/generated/api --typecheck
```

반복해서 생성한다면 TOML 설정 파일에 같은 값을 지정하세요.

```toml
[typescript]
typecheck = true
```

기본값은 `false`이며 각 생성 파일에 `@ts-nocheck`를 넣습니다. `--typecheck`를
켜면 이 지시를 제거해 애플리케이션의 컴파일러가 생성 소스를 검사합니다.
`--typecheck=false`를 명시하면 TOML의 `true` 설정보다 우선합니다. 어느 값을 선택해도
API 타입과 런타임 동작은 같습니다. 루트 SDK, 클라이언트별 코드, 서버 코드,
메타데이터에 함께 적용되며, 기존 출력의 설정은 `--incremental`로 바꿀 수 있습니다.

현재 개발 버전은 위 모듈 설정에서 생성 소스와 선언 파일의
타입 검사를 통과합니다. `strict`, `exactOptionalPropertyTypes`,
`noUncheckedIndexedAccess`, `noPropertyAccessFromIndexSignature`,
`noUnusedLocals`, `noUnusedParameters`, `noImplicitReturns`,
`noImplicitOverride`, `noFallthroughCasesInSwitch`를 함께 지원합니다.
ESM 설정에서는 `verbatimModuleSyntax`도 켤 수 있습니다. 모든 설정에서
`isolatedModules`, 라이브러리 타입 검사, 도달할 수 없는 코드·미사용 레이블 검사를
적용할 수 있습니다.
측정한 SDK별 결과는 [호환성 검증 결과](./compatibility.md)에서 확인할 수 있습니다.

## 타입 기준 선택

아래 표는 루트 SDK에서 타입을 추출하는 기준을 설명합니다. 설정한 클라이언트 진입점에서는
[클라이언트별 타입](#named-clients)을 사용하세요.

| 기준 | 타입 추출 도구 |
| --- | --- |
| 생성된 클라이언트 메서드 | `Operation*<typeof method>` |
| OpenAPI `operationId` | `Operation*<"operationId">` |
| `"METHOD /path"` 경로 | `Route*<"METHOD /path">` |
| `components.schemas` 이름 | `ComponentInput<Name>` / `ComponentOutput<Name>` |

## 클라이언트별 타입 {#named-clients}

각 클라이언트 진입점이 구체적인 `Client` 타입과 선택한 API·후속
호출에 필요한 구성 요소 타입을 제공합니다. `createClient`와 같은 진입점에서
타입을 가져오세요. 상품 API가 `Product` 구성 요소를 사용하는 경우:

```ts
import {
  createClient,
  type Client,
  type ComponentOutput,
} from "./generated/api/clients/catalog/index.js";

type Product = ComponentOutput<"Product">;
type GetProductInput = Parameters<Client["$routes"]["GET /products/{id}"]>[0];
type GetProductOutput = Awaited<ReturnType<Client["$routes"]["GET /products/{id}"]>>;
```

`ComponentInput`은 모델의 요청 표현, `ComponentOutput`은 응답 표현입니다. 다른
클라이언트에서만 쓰는 구성 요소 이름은 포함되지 않으며, 이 클라이언트의 API에서
쓰지 않는 방향의 표현은 `never`입니다. 해당 메서드의 정확한 호출 타입은
`Parameters`와 `ReturnType`으로 추출할 수 있습니다.

## 응답 본문 타입

디코딩한 응답 DTO는 중첩 객체, 배열, 튜플, 맵까지 수정 가능한 타입으로 생성합니다.
일반 반환값, `raw.data`, HTTP 오류 본문, 스트림·페이지네이션 항목, 서버 핸들러의
응답 본문에 같은 출력 계약을 적용합니다. 입력 타입은 읽기 전용 값도 계속 받습니다.
리터럴 제약과 OpenAPI의 `readOnly`·`writeOnly`에 따른 입출력 필드 구분도 유지하며,
응답 메타데이터와 열거형 목록에는 기존 읽기 전용 계약을 적용합니다.

기존 서버 핸들러나 모의 응답이 읽기 전용 배열을 반환했다면 수정 가능한 출력 계약에
맞는 배열을 반환해야 합니다. DTO를 편집하면 로컬 객체와 공유 참조의 값이 바뀌며
API에 자동으로 저장되지는 않습니다. 자세한 내용은
[응답 데이터 편집](../guide/client.md#변환된-응답-객체-사용)을 참고하세요.

`OperationHTTPError<typeof method>`는 해당 메서드에 선언된 HTTP 오류 타입을
추출합니다. 잡은 오류가 `unknown`이면
[`isOperationHTTPError`](./client-api.md#오류-처리)로 이 타입에 맞는지 확인합니다.

## 생성된 메서드에서 추출

생성된 메서드의 타입을 `Operation*` 타입 추출 도구에 전달합니다.

```ts
import {
  createClient,
  type OperationBody,
  type OperationInput,
  type OperationQuery,
} from "./generated/api";

const api = createClient({
  baseURL: "https://api.example.test/v1",
});

const listTodos = api.$operations.listTodos;
type TodoFilters = OperationQuery<typeof listTodos>;

const updateTodo = api.todos("todo-1").update;
type UpdateInput = OperationInput<typeof updateTodo>;
type UpdateBody = OperationBody<typeof updateTodo>;
```

`OperationInput`은 메서드에 전달하는 전체 인자입니다. `OperationBody`는 요청
본문입니다. 리소스 트리 메서드의 입력에는 선택자로 이미 바인딩된 값을 뺀
나머지 항목이 들어갑니다.

```ts
const filters = { completed: false } satisfies TodoFilters;

async function update(body: UpdateBody) {
  return updateTodo({ body });
}
```

<span id="operation-id-또는-route로-추출"></span>

## API 식별자 또는 경로로 추출

OpenAPI `operationId`에는 `Operation*` 타입 추출 도구를 사용하고 `"METHOD /path"`
문자열에는 `Route*` 타입 추출 도구를 사용합니다.

```ts
import type {
  OperationBody,
  OperationInput,
  OperationOutput,
  OperationQuery,
  RouteBody,
  RouteInput,
  RouteOutput,
  RouteParameter,
} from "./generated/api";

type ListInput = OperationInput<"listTodos">;
type ListOutput = OperationOutput<"listTodos">;
type ListQuery = OperationQuery<"listTodos">;
type CreateBody = OperationBody<"createTodo">;

type UpdateInput = RouteInput<"PATCH /todos/{todoID}">;
type UpdateOutput = RouteOutput<"PATCH /todos/{todoID}">;
type UpdateBody = RouteBody<"PATCH /todos/{todoID}">;
type TodoID = RouteParameter<
  "PATCH /todos/{todoID}",
  "path",
  "todoID"
>;
```

<span id="요청-및-응답-helper"></span>

## 요청 및 응답 타입 추출

| 값 | API 식별자·메서드 기준 | 경로 기준 |
| --- | --- | --- |
| 전체 호출 입력 | `OperationInput` | `RouteInput` |
| 성공 응답 | `OperationOutput` | `RouteOutput` |
| 스트림 항목 | `OperationStreamItem` | `RouteStreamItem` |
| 요청 본문 | `OperationBody` | `RouteBody` |
| 경로 파라미터 | `OperationPath` | `RoutePath` |
| 쿼리 파라미터 | `OperationQuery` | `RouteQuery` |
| 쿼리 문자열 파라미터 | `OperationQuerystring` | `RouteQuerystring` |
| 헤더 | `OperationHeaders` | `RouteHeaders` |
| 쿠키 | `OperationCookies` | `RouteCookies` |
| 파라미터 하나 | `OperationParameter` | `RouteParameter` |
| 전체 계약 | `OperationContract` | `RouteContract` |

파라미터 타입을 추출할 때는 위치와 파라미터 이름을 전달합니다.

```ts
type Limit = OperationParameter<"listTodos", "query", "limit">;
```

선택 속성과 스키마에 선언된 `null`은 보존됩니다. 전체 호출에서 요청 영역을
생략할 수 있는지는 `OperationInput` 또는 `RouteInput`으로 확인합니다.

<span id="component-타입"></span>

## 구성 요소 타입

`components.schemas`에 선언된 스키마에는 `ComponentInput`과 `ComponentOutput`을 사용합니다.

```ts
import type { ComponentInput, ComponentOutput } from "./generated/api";

type TodoInput = ComponentInput<"Todo">;
type TodoOutput = ComponentOutput<"Todo">;
```

`readOnly` 필드는 출력 타입에, `writeOnly` 필드는 입력 타입에 포함됩니다.

<span id="enum-값과-타입"></span>

## 열거형 값과 타입

구성 요소의 열거형은 TypeScript 타입과 런타임 값을 제공합니다.

```yaml
components:
  schemas:
    TodoStatus:
      type: string
      enum: [TODO, DONE]
```

```ts
import { Enums, isEnumValue, type EnumValue } from "./generated/api/enums";

type TodoStatus = EnumValue<"TodoStatus">;
// "TODO" | "DONE"

const defaultStatus: TodoStatus = Enums.TodoStatus.TODO;
const completedStatus = Enums.TodoStatus.DONE;

for (const status of Enums.TodoStatus) {
  console.log(status);
}

const options = Array.from(Enums.TodoStatus);

declare const input: unknown;

if (isEnumValue(Enums.TodoStatus, input)) {
  input satisfies TodoStatus;
}
```

기본 `./generated/api` 진입점의 기존 `Enums`, `EnumValue`, `isEnumValue` 가져오기도
그대로 유효합니다. 열거형 런타임 값과 타입을 사용하는 모듈에서는 전용
`./generated/api/enums` 진입점도 사용할 수 있습니다.

`Enums`에는 구성 요소 스키마로 선언된 열거형이 포함됩니다. 인라인 열거형과 중첩
열거형은 생성된 요청, 응답, 구성 요소 타입에서 사용할 수 있습니다.

## 추가 제공 타입

생성된 진입점은 원본 응답 호출, 리소스 트리 호출, 페이지 조회, Link, 스트림 타입도
제공합니다.

| 타입 | 용도 |
| --- | --- |
| `OperationMethod<Route>` | 생성된 API 호출 |
| `OperationRawCall<Route>` | 원본 응답을 반환하는 API 호출 |
| `ResourceCall<Route>` | 리소스 트리 호출 |
| `RawCall<Route>` | 원본 응답을 반환하는 리소스 호출 |
| `PaginateCall<Route>` | 페이지 조회 호출 |
| `LinkCalls<Route>` | OpenAPI Link 호출 |
| `StreamCall<Route>` | 스트림 호출 |
| `RouteStreamItem<Route>` | 정확한 경로 스트림의 항목 타입 |
| `OperationStreamItem<Source>` | API 식별자나 생성된 메서드에서 추출한 스트림 항목 타입 |
| `OperationStream<T>` | 필요할 때 시작되는 단일 소비자 스트리밍 응답 객체 |
| `StreamSource<T>` | 요청 데이터를 제공하는 `AsyncIterable<T>` 또는 `ReadableStream<T>` |
| `ServerSentEvent` | 표준 SSE 프레임 값 |
| `StreamProtocol<Frame>` | 사용자 정의 순차형 미디어 프레임 분리 |
| `StreamAdapter` | 스트림 프로토콜 위에 적용하는 애플리케이션 변환 |
| `StreamCodec` | 선택적인 프로토콜/어댑터 조합 |
| `StreamResponseMetadata` | 열린 스트림의 상태 코드, 헤더, 콘텐츠 타입, 요청 메타데이터 |
| `CursorPaginationInput` | 커서 기반 페이지 조회 입력 |
| `OffsetPaginationInput` | 오프셋 기반 페이지 조회 입력 |
| `BothPaginationInput` | 커서 또는 오프셋 기반 페이지 조회 입력 |
| `SortDirection` | `"asc" | "desc"` 타입과 런타임 상수 |

스트림의 시작·종료 동작, 요청 데이터 공급, SSE 프레임, 프로토콜/어댑터 계약과 코덱
설정은 [스트리밍 API](./streaming.md)에 정리되어 있습니다.

`Components`, `Operations`, `Routes`는 생성된 타입을 이름별로 모은 매핑을 제공합니다.
