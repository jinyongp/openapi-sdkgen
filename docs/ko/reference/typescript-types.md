# 생성된 TypeScript 타입

생성된 SDK의 기본 진입점은 요청, 응답, component, enum 타입을 제공합니다.

생성된 클라이언트의 호출 방법은
[생성된 클라이언트 API](./client-api.md)에서 확인할 수 있습니다.

## 컴파일러 지원

생성된 client와 server 소스는 **TypeScript 5.7.3 이상**과 `ES2022`, `DOM`,
`DOM.Iterable` 라이브러리를 사용합니다. Runtime의 generic typed array는
[TypeScript 5.7](https://www.typescriptlang.org/docs/handbook/release-notes/typescript-5-7.html)에서
도입됐습니다. 컴파일러와 번들러는 애플리케이션이 관리하며, 생성기가 출력
디렉터리에 TypeScript 컴파일러를 설치하지는 않습니다.

CI의 `just agent ts-compat`는 **5.7.3, 5.9.3, 6.0.3, 7.0.2**를 고정해 검증합니다.
각 버전에서 생성된 client·server 소스와 공개 타입 사용 예제의 `@ts-nocheck`를
제거한 뒤 `strict`, `noUncheckedIndexedAccess`, `verbatimModuleSyntax`,
`isolatedModules`, `skipLibCheck: false`로 검사합니다. 검증 입력은 3.2 normative,
webhook, nested Schema reference fixture입니다. 전체 runtime/conformance suite와
외부 compatibility benchmark는 저장소의 TypeScript 7.0.2를 사용합니다.

새 컴파일러 버전은 이 matrix에 추가하면서 검증합니다. 최소 버전 계약이 아직
나오지 않은 컴파일러의 변경까지 검증했다는 뜻은 아닙니다. TypeScript 5.6
이하는 지원 범위 밖입니다.

이 matrix는 `CHANGELOG.md`에 기록한 readable-stream iterator 수정까지 포함합니다.
v9.0.0으로 생성한 소스에는 이 수정이 없어서 `@ts-nocheck`를 제거하면 이전
컴파일러의 strict 검사에서 오류가 날 수 있습니다. 수정이 포함된 generator
release로 다시 생성하면 됩니다.

## 타입 기준 선택

| 기준 | Helper |
| --- | --- |
| 생성된 클라이언트 메서드 | `Operation*<typeof method>` |
| OpenAPI `operationId` | `Operation*<"operationId">` |
| `"METHOD /path"` route | `Route*<"METHOD /path">` |
| `components.schemas` 이름 | `ComponentInput<Name>` / `ComponentOutput<Name>` |

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
