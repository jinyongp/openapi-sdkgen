<span id="필요한-operation만-불러오기"></span>

# 필요한 API만 불러오기

생성된 SDK에는 기존 전체 클라이언트와 함께 선택형 진입점이 제공됩니다.
큰 API 중 일부만 사용하는 애플리케이션이라면 이 진입점으로 필요한 API와
그 실행에 필요한 코드만 준비할 수 있습니다. 기본 설정에서는 모든 API와 TypeScript
타입을 생성하므로 애플리케이션에서 선택을 바꿀 때 SDK를 다시 생성할 필요는 없습니다.
생성 파일 자체를 줄이려면 생성할 API를 설정에서 지정하세요.

## 필요한 API만 생성하기 {#generation}

기능마다 다른 API 목록을 별도 모듈 경로에 배정하려면
[기능별 클라이언트 생성](./named-clients.md)을 사용하세요. 아래 선택은 루트 SDK와
선택형 진입점에 적용됩니다.

API 식별자(`operationId`)와 경로는 [생성할 API 찾기](./inspect.md)에서 조회할 수 있습니다.
검색 결과를 생성 설정에 넣을 선택 목록으로 출력할 수도 있습니다.

큰 API 문서의 일부만 사용한다면 생성 설정에 필요한 API 식별자나 경로를 적습니다.

```toml
source = "./openapi.yaml"
target = "typescript"
output = "./src/generated/api"

[selection]
operations = ["listTasks"]
routes = ["GET /tasks/{task-id}"]
```

이 파일로 `openapi-sdkgen generate --config ./openapi-sdkgen.toml`을 실행하면
두 목록에 있는 API와 필요한 타입·실행 코드를 생성합니다. 일반 클라이언트와 선택형
진입점 모두 이 API 집합을 사용하며, `loadOperations`로 그 안의 API를 준비할 수 있습니다.

이름은 원문과 정확히 일치해야 합니다. `operations`에는 원래 `operationId`를,
`routes`에는 `GET /tasks/{task-id}`처럼 메서드와 OpenAPI 경로를 적습니다.
경로 인자는 원문에 있는 `{task-id}` 형태를 유지하고, 실제 호출할 때
`path: { "task-id": "one" }`으로 값을 전달합니다. API 식별자가 없는 API는
경로로 선택할 수 있으며, 같은 API를 여러 번 지정해도 한 번만 생성합니다.

`[selection]`과 이름 있는 클라이언트를 모두 생략하면 전체 API를 생성합니다.
이름 있는 클라이언트만 지정하면 루트는 각 선택의 합집합을 제공합니다.
빈 선택 목록, 존재하지 않는 이름,
숨겨진 API를 지정하면 오류가 발생합니다. 중복 API 식별자를 비롯한 문서 오류는
선택 생성 전에 수정해야 합니다.

선택한 API의 OpenAPI Link도 사용할 수 있습니다. Link 대상과 필요한 타입은 내부
의존 코드로 함께 생성됩니다. 대상을 직접 호출하거나 `loadOperations`로 준비하려면
선택 목록에도 추가하세요. Link 대상은 문서의 참조를 통해 이미 불러온 범위에 있어야
합니다. 자세한 조건은 [Link 지원 범위](../reference/capabilities.md)에서 확인하세요.

`--with server`를 함께 쓰면 선택한 API에 연결된 콜백과 최상위 웹훅을 생성합니다.
기존 SDK의 선택 목록을 바꿀 때는 `--incremental`을 사용하세요. 생성기가 관리하는
파일을 갱신하고 사용자 파일은 보존합니다. `--check --output`으로 기존 SDK가
현재 선택 목록에 맞는지도 확인할 수 있습니다.

CLI에서는 `--operation`과 `--route`를 반복해서 지정할 수 있습니다.
문법과 설정 덮어쓰기 규칙은 [CLI 레퍼런스](../reference/cli.md#api-selection)를 참고하세요.

아래 예제는 `GET /tasks`에 `operationId: listTasks`가 있고, `GET /health`에는
API 식별자가 없는 문서를 기준으로 합니다. 먼저 [SDK 생성과 검증](./generate.md)에
따라 SDK를 생성하세요. 브라우저에 직접 제공할 때는 생성된 TypeScript를 ESM
JavaScript로 컴파일해야 합니다.

## 코드를 준비한 뒤 클라이언트 구성하기 {#prepare}

생성 소스, 배포 파일, 압축 용량, 준비 시간은
[선택 방식별 비용 비교](./selection-benchmarks.md)에서 확인할 수 있습니다.
정적 operation import와 동적 lookup은 배포 비용이 다릅니다.

```ts
import {
  createClient,
  loadOperations,
  operations,
  routes,
} from "./generated/api/selective/index.js";

const prepared = await loadOperations([
  operations.listTasks,
  routes["GET /health"],
]);

const api = createClient({
  baseURL: "https://api.example.test/v1",
  operations: prepared,
});

const tasks = await api.$operations.listTasks({ query: { limit: 20 } });
console.log(tasks);
await api.$routes["GET /health"]();
```

`operations`는 OpenAPI에 선언된 API 식별자를 키로 사용합니다. `routes`의 키는
HTTP 메서드와 OpenAPI 경로 템플릿을 합친 값이며, 식별자가 없는 API도 선택할
수 있습니다. 같은 API를 두 이름으로 선택해도 한 번만 등록됩니다. 지역 변수나
내보내는 이름을 바꿔도 생성된 클라이언트의 메서드 이름은 바뀌지 않습니다.

`loadOperations`는 코드를 준비할 뿐 API를 호출하지 않습니다. `createClient`는
동기 함수이며, 준비된 코드에 기본 URL, Fetch 구현, 인증 등의
[클라이언트 옵션](../reference/client-api.md#clientoptions)을 연결합니다. 같은 준비
결과를 서로 다른 설정의 클라이언트에서 재사용해도 자격 증명은 공유되지 않습니다.
빈 선택형 클라이언트가 필요하면 빈 배열을 준비하면 됩니다.

`createClient`도 선택형 진입점에서 가져와야 합니다. 기존 루트 진입점의 생성 함수는
전체 클라이언트를 만들며, 이후 애플리케이션이 호출하는 메서드만 보고 선택을 추론하지
않습니다.

## 기능별 선택 목록 합치기 {#features}

각 기능에서 사용하는 API 목록을 해당 기능의 코드와 함께 두고 합칠 수 있습니다.

```ts
// tasks.operations.ts
import { operations } from "./generated/api/selective/index.js";

export default [operations.listTasks] as const;
```

```ts
import {
  createClient,
  loadOperations,
  routes,
} from "./generated/api/selective/index.js";
import tasks from "./tasks.operations.js";

const api = createClient({
  operations: await loadOperations([tasks, routes["GET /health"]]),
});
await api.$operations.listTasks({ query: { limit: 20 } });
```

중첩된 배열과 객체도 그대로 전달할 수 있으므로, 기능별 배열을 전개 문법(`...`)으로
풀어 쓸 필요가 없습니다. 위 예제의 기본 내보내기 대신 이름을 지정해 내보낸 배열도
사용할 수 있습니다. 모듈에서 내보낸 값이 모두 API 선택 값이라면 모듈 네임스페이스
객체도 전달할 수 있습니다.

선택 목록에는 API 참조, 배열, 선택 값으로 구성한 객체를 넣습니다. 객체에서는 직접
소유한 문자열 키의 속성을, 배열에서는 각 인덱스의 값을 읽습니다. 접근자도 실행되며,
그 결과는 한 번의 준비 과정에서 재사용하고 발생한 예외는 호출자에게 전달합니다.
같은 목록을 여러 그룹에서 재사용할 수 있지만, 자기 자신을 참조하는 등 순환 구조가
있으면 오류가 발생합니다.

모듈 전체를 전달할 때는 선택 목록만 내보내도록 구성하세요. 비동기로 구한 목록은
애플리케이션에서 먼저 `await`한 뒤 전달합니다. `loadOperations`가 받는 값은 API
참조와 배열·객체이며, 함수 실행이나 `Promise` 처리, 임의의 반복 가능한 객체 순회는
애플리케이션에서 수행합니다.

## 확정된 선택과 계산된 선택의 타입 {#types}

직접 작성한 튜플이나 `as const`로 선언한 기능별 배열은 어떤 API가 반드시
포함되는지 보존합니다. 해당 메서드는 필수 속성이며 원래의 입력, 출력, 오류,
`.raw()`, 페이지 조회, 스트리밍 타입을 유지합니다. 리소스 메서드도 해당 API를
선택했을 때만 포함되고, 이름과 충돌 처리 규칙은 전체 클라이언트와 같습니다.

실행 중 필터링으로 계산한 배열은 특정 API가 포함된다고 보장할 수 없습니다.
이때 후보 메서드는 선택적 속성이므로 존재를 확인한 뒤 호출합니다.

```ts
const candidates = [operations.listTasks, routes["GET /health"]];
const selection = candidates.filter(() => Math.random() > 0.5);
const dynamic = createClient({
  operations: await loadOperations(selection),
});

await dynamic.$operations.listTasks?.({ query: { limit: 20 } });
```

고정된 목록이라면 선언할 때 튜플 타입을 유지하세요. 이미 일반 배열로 넓어진 타입은
나중에 함수에 전달한다고 원래 정보가 복원되지 않습니다. 타입은 접근자의 내부 동작이나
사용자가 작성한 타입 단언의 진실성까지 증명하지는 않습니다.

## 전체 경로를 열거해서 선택하기 {#enumeration}

기본 참조 객체에는 모든 API 이름의 실행 시점 목록이 들어 있지 않습니다.
열거하거나 필터링해야 한다면 이름만 제공하는 별도 진입점을 사용합니다.

```ts
import { routes as allRoutes } from "./generated/api/selective/all.js";
import { createClient, loadOperations } from "./generated/api/selective/index.js";

const taskRoutes = Object.entries(allRoutes)
  .filter(([route]) => route.startsWith("GET /tasks"))
  .map(([, operation]) => operation);

const preparedTasks = await loadOperations(taskRoutes);
const api = createClient({ operations: preparedTasks });
await api.$operations.listTasks?.({ query: { limit: 20 } });
```

`all.js`를 사용하면 이름 목록의 전송 비용이 추가되지만 모든 API 구현을 가져오지는
않습니다. 이렇게 계산한 선택의 후보 메서드는 여전히 선택적입니다. 작은 기본 참조
객체를 열거하거나 그 객체에서 전체 목록의 포함 여부를 확인하는 대신 이 진입점을
사용하세요.

## 번들러에서 정적 참조 사용하기 {#bundlers}

애플리케이션을 번들링한다면 생성된 API 모듈을 직접 가져와 번들러가 실행
의존성을 확인할 수 있게 합니다.

```ts
// tasks.operations.ts
import { operation } from "./generated/api/selective/operations/tasks/get.js";

export default [operation] as const;
```

이 배열을 같은 `loadOperations`와 `createClient`에 전달하면 됩니다. 모듈에서 내보내는 참조의
이름은 `operation`이며, 클라이언트의 메서드 이름은 여전히 원본 API 식별자로
결정됩니다. OpenAPI 경로에 특수문자나 이름 충돌이 있다면 실제 생성된 파일 경로를
확인하세요.

정적으로 모듈을 가져오면 Vite가 선택한 생성 모듈을 애플리케이션 번들에 포함합니다.

기본 네임스페이스 참조는 실행 중 모듈 조회 URL을 계산합니다. 번들러가 이 경로를 보고
필요한 파일을 자동으로 모두 수집한다고 가정해서는 안 됩니다. 정적 참조를 사용하거나,
생성된 ESM 모듈을 외부 자산으로 그대로 제공하세요. 번들러의 청크 분리와
미리 불러오기 설정에 따라서도 다운로드 시점이 달라지므로 최종 애플리케이션 산출물을
확인해야 합니다.

## 후속 호출과 스트리밍 사용하기 {#helpers}

준비한 API는 기존 보조 기능을 유지합니다. ESM 모듈을 직접 제공하는 환경에서는 OpenAPI Link의
대상 코드를 후속 호출 시 불러옵니다.
따라서 첫 호출에는 모듈 다운로드 시간이나 모듈 로딩 오류가 추가될 수 있습니다.
요청에는 같은 클라이언트의 설정을 사용합니다. Link 내부에서만 필요한 대상이 공개
API·경로 목록에 자동으로 추가되지는 않습니다.

이미 준비한 대상은 재사용합니다. 선택한 API의 동기 `.stream()`은 준비 완료 후
바로 사용할 수 있고,
[`OperationStream`](../reference/streaming.md#operationstream)을 반환합니다.
[요청 취소와 시간 제한](./transport.md)은 요청에 적용됩니다. 브라우저의 모듈
다운로드까지 취소한다고 보장하는 것은 아닙니다.

## 생성된 트리를 일관되게 배포하기 {#deployment}

ESM 모듈을 직접 제공할 때는 컴파일된 생성 디렉터리 전체를 배포하고 상대 경로와 `.js`
확장자를 유지합니다. 코드의 기준 경로는 생성된 모듈의 위치이며 API의 `baseURL`과는
다릅니다. 네임스페이스를 통한 모듈 조회는 Web Crypto를 사용하므로 운영 환경에서는 HTTPS를
사용하세요. JavaScript에 맞는 MIME 타입을 제공하고 스크립트 정책에서 해당 출처를
허용해야 하며, 다른 출처의 모듈을 불러올 때는 CORS 응답도 필요합니다.

생성 버전마다 다른 자산 URL로 배포하고, 기존 페이지가 사용할 수 있는 동안은 이전
자산을 유지합니다. 일부 파일만 새 생성 버전으로 덮어쓰지 마세요. 로더가 생성 버전과
프로토콜 호환성을 확인하지만, 이 검사는 인증이나 신뢰할 수 있는 스크립트 호스팅을
대신하지 않습니다.

선택형 진입점의 `OperationPreparationError`에는 `stage`가 있습니다. 값은 `INPUT`,
`MODULE_LOAD`, `IDENTITY`, `BINDING`이며 가능한 경우 원래 `cause`도 보존합니다.
모듈 로딩 실패만으로 HTTP 상태나 CSP·오프라인·파일 누락을 항상 구별할 수는
없으므로 브라우저의 네트워크 진단을 함께 확인하세요. 애플리케이션 접근자의 예외를
안전성 판정 오류로 바꾸지는 않습니다.

첫 준비, 추가 기능 준비, 첫 후속 호출, 재방문을 나눠 측정하세요. 공통 코드는
재사용되지만 대부분의 API를 선택하면 모듈 조회와 로딩 비용 때문에 전체
클라이언트보다 커질 수도 있습니다. 실제 사용 패턴에서 더 적합하다면 기존 전체
진입점을 사용하는 것이 좋습니다.
