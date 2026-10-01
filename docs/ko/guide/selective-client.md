# 필요한 operation만 불러오기

생성된 SDK에는 기존 전체 클라이언트와 함께 선택형 진입점이 제공됩니다.
큰 API 중 일부만 사용하는 애플리케이션이라면 이 진입점으로 필요한 operation과
그 실행에 필요한 코드만 준비할 수 있습니다. 생성기는 모든 operation과 TypeScript
타입을 그대로 출력합니다. 애플리케이션에서 선택을 바꿀 때마다 SDK를 다시 생성할
필요는 없습니다.

아래 예제는 `GET /tasks`에 `operationId: listTasks`가 있고, `GET /health`에는
operation ID가 없는 문서를 기준으로 합니다. 먼저 [SDK 생성과 검증](./generate.md)에
따라 SDK를 생성하세요. 브라우저에 직접 제공할 때는 생성된 TypeScript를 ESM
JavaScript로 컴파일해야 합니다.

## 코드를 준비한 뒤 클라이언트 구성하기 {#prepare}

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
await api.$routes["GET /health"]();
```

`operations`는 OpenAPI의 정확한 operation ID를 키로 사용합니다. `routes`의 키는
HTTP method와 OpenAPI path template을 합친 값이며, ID가 없는 operation도 선택할
수 있습니다. 같은 operation을 두 이름으로 선택해도 한 번만 등록됩니다. 지역 변수나
export 이름을 바꿔도 생성된 클라이언트의 메서드 이름은 바뀌지 않습니다.

`loadOperations`는 코드를 준비할 뿐 API를 호출하지 않습니다. `createClient`는
동기 함수이며, 준비된 코드에 base URL, Fetch 구현, 인증 등의
[클라이언트 옵션](../reference/client-api.md#clientoptions)을 연결합니다. 같은 준비
결과를 서로 다른 설정의 클라이언트에서 재사용해도 자격 증명은 공유되지 않습니다.
빈 선택형 클라이언트가 필요하면 빈 배열을 준비하면 됩니다.

`createClient`도 선택형 진입점에서 가져와야 합니다. 기존 루트 진입점의 생성 함수는
전체 클라이언트를 만들며, 이후 애플리케이션이 호출하는 메서드만 보고 선택을 추론하지
않습니다.

## 이름을 중복 작성하지 않고 feature 합성하기 {#features}

feature에서 사용하는 선택 목록을 해당 코드와 함께 둘 수 있습니다.

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
```

중첩 배열과 객체는 값으로 합성하므로 feature 배열에 spread를 쓸 필요가 없습니다.
named export로 내보낸 배열도 사용할 수 있고, export 값이 모두 선택 값인 모듈
namespace도 전달할 수 있습니다. 그룹에서는 자체 소유한 문자열 키의 속성을,
배열에서는 index의 값을 읽습니다. getter도 일반 속성처럼 읽고, 얻은 값을 그 준비
과정에서 재사용합니다. getter의 부작용은 애플리케이션 코드의 동작이며, 던진 오류는
그대로 전달됩니다.

선택 목록에는 operation 참조, 배열 또는 선택 값으로 구성한 그룹을 넣습니다.
모듈 전체를 선택 목록으로 전달한다면 무관한 metadata나 함수를 함께 export하지
않도록 구분하세요. 비동기 설정은 애플리케이션에서 먼저 await한 뒤 결과를 전달합니다.
로더가 함수를 대신 실행하거나 Promise를 await하거나 임의 iterable을 소비하지는
않습니다. 순환하는 컨테이너는 오류지만, 같은 feature를 여러 그룹에서 재사용하는
것은 허용됩니다.

## 확정된 선택과 계산된 선택의 타입 {#types}

직접 작성한 tuple이나 `as const`로 선언한 feature 배열은 어떤 operation이 반드시
포함되는지 보존합니다. 해당 메서드는 필수 속성이며 원래의 입력, 출력, 오류,
`.raw()`, pagination, stream 타입을 유지합니다. resource 메서드도 해당 operation을
선택했을 때만 포함되고, 이름과 충돌 처리 규칙은 전체 클라이언트와 같습니다.

실행 중 filter로 계산한 배열은 특정 operation이 포함된다고 보장할 수 없습니다.
이때 후보 메서드는 선택적 속성이므로 존재를 확인한 뒤 호출합니다.

```ts
const candidates = [operations.listTasks, routes["GET /health"]];
const selection = candidates.filter(() => Math.random() > 0.5);
const dynamic = createClient({
  operations: await loadOperations(selection),
});

await dynamic.$operations.listTasks?.({ query: { limit: 20 } });
```

고정된 목록이라면 선언할 때 tuple 타입을 유지하세요. 이미 일반 배열로 넓어진 타입은
나중에 함수에 전달한다고 원래 정보가 복원되지 않습니다. 타입은 getter의 내부 동작이나
사용자가 작성한 타입 단언의 진실성까지 증명하지는 않습니다.

## 전체 route를 열거해서 선택하기 {#enumeration}

기본 참조 객체에는 모든 operation 이름의 실행 시점 목록이 들어 있지 않습니다.
열거하거나 필터링해야 한다면 이름만 제공하는 별도 진입점을 사용합니다.

```ts
import { routes as allRoutes } from "./generated/api/selective/all.js";

const taskRoutes = Object.entries(allRoutes)
  .filter(([route]) => route.startsWith("GET /tasks"))
  .map(([, operation]) => operation);

const preparedTasks = await loadOperations(taskRoutes);
```

`all.js`를 사용하면 이름 목록의 전송 비용이 추가되지만 모든 operation 구현을 가져오지는
않습니다. 이렇게 계산한 선택의 후보 메서드는 여전히 선택적입니다. 작은 기본 참조
객체를 열거하거나 그 객체에서 전체 목록의 포함 여부를 확인하는 대신 이 진입점을
사용하세요.

## 번들러에서 정적 참조 사용하기 {#bundlers}

애플리케이션을 번들링한다면 생성된 operation 모듈을 직접 import해 번들러가 실행
의존성을 확인할 수 있게 합니다.

```ts
// tasks.operations.ts
import { operation } from "./generated/api/selective/operations/tasks/get.js";

export default [operation] as const;
```

이 배열을 같은 `loadOperations`와 `createClient`에 전달하면 됩니다. 모듈의 참조 export
이름은 `operation`이며, 클라이언트의 메서드 이름은 여전히 원본 operation ID로
결정됩니다. OpenAPI 경로에 특수문자나 이름 충돌이 있다면 실제 생성된 파일 경로를
확인하세요.

정적 import를 사용하면 Vite가 선택한 생성 모듈을 애플리케이션 번들에 포함합니다.

기본 namespace 참조는 실행 중 lookup URL을 계산합니다. 번들러가 이 경로를 보고
필요한 파일을 자동으로 모두 수집한다고 가정해서는 안 됩니다. 정적 참조를 사용하거나,
생성된 native ESM 트리를 외부 자산으로 그대로 제공하세요. 번들러의 chunk 분리와
preload 설정에 따라서도 다운로드 시점이 달라지므로 최종 애플리케이션 산출물을
확인해야 합니다.

## Link와 stream 사용하기 {#helpers}

준비한 operation은 기존 helper API를 유지합니다. native ESM에서는 OpenAPI Link의
대상 코드가 소스 operation을 준비할 때가 아니라 helper를 호출할 때 로드됩니다.
따라서 첫 호출에는 모듈 다운로드 시간이나 모듈 로딩 오류가 추가될 수 있습니다.
요청에는 같은 클라이언트의 설정을 사용합니다. Link 내부에서만 필요한 대상이 공개
operation·route 목록에 자동으로 추가되지는 않습니다.

이미 준비한 대상은 재사용합니다. 선택한 operation의 동기 `.stream()`은 준비 완료 후
바로 사용할 수 있고, 스트림의 Promise가 아닌
[OperationStream](../reference/streaming.md#operationstream)을 반환합니다.
[요청 취소와 timeout](./transport.md)은 요청에 적용됩니다. 브라우저의 native 모듈
다운로드까지 취소한다고 보장하는 것은 아닙니다.

## 생성된 트리를 일관되게 배포하기 {#deployment}

native ESM으로 제공할 때는 컴파일된 생성 트리 전체를 배포하고 상대 경로와 `.js`
확장자를 유지합니다. 코드의 기준 경로는 생성된 모듈의 위치이며 API의 `baseURL`과는
다릅니다. namespace lookup은 Web Crypto를 사용하므로 운영 환경에서는 HTTPS를
사용하세요. JavaScript에 맞는 MIME type을 제공하고 스크립트 정책에서 해당 출처를
허용해야 하며, 다른 출처의 모듈을 불러올 때는 CORS 응답도 필요합니다.

생성 버전마다 다른 자산 URL로 배포하고, 기존 페이지가 사용할 수 있는 동안은 이전
자산을 유지합니다. 일부 파일만 새 생성 버전으로 덮어쓰지 마세요. 로더가 generation과
프로토콜 호환성을 확인하지만, 이 검사는 인증이나 신뢰할 수 있는 스크립트 호스팅을
대신하지 않습니다.

선택형 진입점의 `OperationPreparationError`에는 `stage`가 있습니다. 값은 `INPUT`,
`MODULE_LOAD`, `IDENTITY`, `BINDING`이며 가능한 경우 원래 cause도 보존합니다.
native import 실패만으로 HTTP status나 CSP·오프라인·파일 누락을 항상 구별할 수는
없으므로 브라우저의 네트워크 진단을 함께 확인하세요. 애플리케이션 getter의 예외를
안전성 판정 오류로 바꾸지는 않습니다.

첫 준비, 추가 feature 준비, 첫 Link 호출, 재방문을 나눠 측정하세요. 공통 코드는
재사용되지만 대부분의 operation을 선택하면 lookup과 모듈 오버헤드 때문에 전체
클라이언트보다 커질 수도 있습니다. 실제 사용 패턴에서 더 적합하다면 기존 전체
진입점을 사용하는 것이 좋습니다.
