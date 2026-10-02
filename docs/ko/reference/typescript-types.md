# 생성된 TypeScript 타입

생성된 SDK의 기본 진입점은 요청, 응답, component, enum 타입을 제공합니다.

생성된 클라이언트의 호출 방법은
[생성된 클라이언트 API](./client-api.md)에서 확인할 수 있습니다.

## 컴파일러 지원

생성된 클라이언트와 서버 소스에는 **TypeScript 5.7.3 이상**과 `ES2022`, `DOM`,
`DOM.Iterable` 라이브러리가 필요합니다. 애플리케이션의 기존 TypeScript
컴파일러와 번들러로 함께 빌드합니다.

지원 확인 버전: **5.7.3, 5.9.3, 6.0.3, 7.0.2**.

## 타입 기준 선택

아래 표는 루트 SDK의 타입 helper를 설명합니다. 설정한 클라이언트 진입점에서는
[클라이언트별 타입](#named-clients)을 사용하세요.

| 기준 | Helper |
| --- | --- |
| 생성된 클라이언트 메서드 | `Operation*<typeof method>` |
| OpenAPI `operationId` | `Operation*<"operationId">` |
| `"METHOD /path"` route | `Route*<"METHOD /path">` |
| `components.schemas` 이름 | `ComponentInput<Name>` / `ComponentOutput<Name>` |

## 클라이언트별 타입 {#named-clients}

다음 릴리스에서는 각 클라이언트 진입점이 구체적인 `Client` 타입과 선택한 API·Link
helper에 필요한 component 타입을 제공합니다. `createClient`와 같은 진입점에서
타입을 가져오세요. 상품 API가 `Product` component를 사용하는 경우:

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
클라이언트에서만 쓰는 component 이름은 포함되지 않으며, 이 클라이언트의 API에서
쓰지 않는 방향의 표현은 `never`입니다. 해당 메서드의 정확한 호출 타입은
`Parameters`와 `ReturnType`으로 추출할 수 있습니다.

## 생성된 메서드에서 추출

생성된 메서드의 타입을 `Operation*` helper에 전달합니다.

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

`OperationInput`은 메서드에 전달하는 전체 인자입니다. `OperationBody`는 request
body입니다. Resource tree 메서드의 입력에는 selector로 이미 바인딩된 값을 뺀
나머지 항목이 들어갑니다.

```ts
const filters = { completed: false } satisfies TodoFilters;

async function update(body: UpdateBody) {
  return updateTodo({ body });
}
```

## Operation ID 또는 route로 추출

OpenAPI `operationId`에는 `Operation*` helper를 사용하고 `"METHOD /path"`
문자열에는 `Route*` helper를 사용합니다.

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

## 요청 및 응답 helper

| 값 | Operation helper | Route helper |
| --- | --- | --- |
| 전체 호출 입력 | `OperationInput` | `RouteInput` |
| 성공 응답 | `OperationOutput` | `RouteOutput` |
| stream item | `OperationStreamItem` | `RouteStreamItem` |
| request body | `OperationBody` | `RouteBody` |
| path 파라미터 | `OperationPath` | `RoutePath` |
| query 파라미터 | `OperationQuery` | `RouteQuery` |
| query-string 파라미터 | `OperationQuerystring` | `RouteQuerystring` |
| header | `OperationHeaders` | `RouteHeaders` |
| cookie | `OperationCookies` | `RouteCookies` |
| 파라미터 하나 | `OperationParameter` | `RouteParameter` |
| 전체 계약 | `OperationContract` | `RouteContract` |

파라미터 helper에는 location과 파라미터 이름을 전달합니다.

```ts
type Limit = OperationParameter<"listTodos", "query", "limit">;
```

선택 속성과 스키마에 선언된 `null`은 보존됩니다. 전체 호출에서 요청 영역을
생략할 수 있는지는 `OperationInput` 또는 `RouteInput`으로 확인합니다.

## Component 타입

`components.schemas`에 선언된 스키마에는 component helper를 사용합니다.

```ts
import type { ComponentInput, ComponentOutput } from "./generated/api";

type TodoInput = ComponentInput<"Todo">;
type TodoOutput = ComponentOutput<"Todo">;
```

`readOnly` 필드는 출력 타입에, `writeOnly` 필드는 입력 타입에 포함됩니다.

## Enum 값과 타입

Component enum은 TypeScript 타입과 런타임 값을 제공합니다.

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

기본 `./generated/api` 진입점의 기존 `Enums`, `EnumValue`, `isEnumValue` import도
그대로 유효합니다. Enum 런타임 값과 타입을 사용하는 모듈에서는 전용
`./generated/api/enums` 진입점도 사용할 수 있습니다.

`Enums`에는 component 스키마로 선언된 enum이 포함됩니다. Inline enum과 중첩
enum은 생성된 요청, 응답, component 타입에서 사용할 수 있습니다.

## 추가 제공 타입

생성된 진입점은 raw 호출, resource tree 호출, pagination, Link, stream 타입도
제공합니다.

| 타입 | 용도 |
| --- | --- |
| `OperationMethod<Route>` | 생성된 operation 호출 |
| `OperationRawCall<Route>` | raw operation 호출 |
| `ResourceCall<Route>` | resource tree 호출 |
| `RawCall<Route>` | raw resource tree 호출 |
| `PaginateCall<Route>` | pagination 호출 |
| `LinkCalls<Route>` | OpenAPI Link 호출 |
| `StreamCall<Route>` | stream 호출 |
| `RouteStreamItem<Route>` | exact route stream의 item 타입 |
| `OperationStreamItem<Source>` | operation ID나 생성된 메서드에서 추출한 stream item 타입 |
| `OperationStream<T>` | lazy 단일 소비자 streaming response handle |
| `StreamSource<T>` | 요청에 사용하는 `AsyncIterable<T>` 또는 `ReadableStream<T>` source |
| `ServerSentEvent` | 표준 SSE frame 값 |
| `StreamProtocol<Frame>` | 사용자 정의 sequential media byte framing |
| `StreamAdapter` | stream protocol 위에 적용하는 application 변환 |
| `StreamCodec` | 선택적인 protocol/adapter 조합 |
| `StreamResponseMetadata` | 열린 stream의 status, header, content type, request metadata |
| `CursorPaginationInput` | cursor pagination 입력 |
| `OffsetPaginationInput` | offset pagination 입력 |
| `BothPaginationInput` | cursor 또는 offset pagination 입력 |
| `SortDirection` | `"asc" | "desc"` 타입과 런타임 상수 |

Stream lifecycle, request source, SSE frame, protocol/adapter 계약과 codec
설정은 [스트리밍 API](./streaming.md)에 정리되어 있습니다.

`Components`, `Operations`, `Routes`는 생성된 전체 타입 map을 제공합니다.
