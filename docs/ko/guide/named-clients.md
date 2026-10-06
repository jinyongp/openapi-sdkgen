# 기능별 클라이언트 생성

페이지마다 서로 다른 API를 쓴다면 생성 설정에서 기능별 클라이언트를 지정할 수
있습니다. 각 클라이언트는 별도 모듈 경로를 가지며, 배정된 API의 메서드와 타입을
제공합니다. 실행 코드와 API 구현, 필요한 모델은 클라이언트 사이에서 공유하고,
같은 API를 여러 클라이언트에 넣어도 구현을 재사용합니다.

## 클라이언트에 API 배정하기

아래 예제는 `listOrders`, `createOrder`, `GET /products/{id}`가 있는 OpenAPI
문서를 사용합니다. 모든 클라이언트는 입력 문서 하나와 생성 설정을 공유합니다.
현재 클라이언트별 설정은 `selection`이며, API 식별자(`operationId`)와 경로를 함께 지정할 수
있습니다.

```toml
source = "./openapi.yaml"
target = "typescript"
output = "./src/generated/api"

[clients.orders.selection]
operations = ["listOrders", "createOrder"]

[clients.catalog.selection]
routes = ["GET /products/{id}"]
```

`openapi-sdkgen.toml`로 저장한 뒤 SDK를 생성합니다.

```sh
openapi-sdkgen generate --config ./openapi-sdkgen.toml
```

`src/generated/api/clients/orders/index.ts`와
`src/generated/api/clients/catalog/index.ts`에 진입점이 생깁니다.
ID와 경로는 [생성할 API 찾기](./inspect.md)에서 조회할 수 있습니다. 선택에는
`{id}`를 포함한 원문 이름을 그대로 적고, 실제 ID는 생성된 메서드를 호출할 때 전달합니다.

클라이언트 이름은 영문 소문자로 시작하고, 소문자·숫자·하이픈으로 구성한 64자 이내의
이름을 사용합니다. `con` 같은 Windows 장치 이름은 예약되어 있습니다. 각 클라이언트에는
비어 있지 않은 선택 목록이 필요합니다. 존재하지 않거나 숨겨진 API, 지원하지 않는
클라이언트 속성을 지정하면 오류가 발생합니다.

## 사용하는 페이지에서 가져오기

`GET /products/{id}`의 리소스 메서드가 `get`으로 생성되는 문서라면 상품 페이지에서
다음과 같이 호출할 수 있습니다.

```ts
import { createClient } from "./generated/api/clients/catalog/index.js";

const api = createClient({ baseURL: "https://api.example.test/v1" });
const product = await api.products("product-1").get();
```

`createClient`는 동기 함수입니다. 리소스 트리와 `$routes`, `$operations`에는
선택한 API가 들어갑니다. 리소스 메서드 이름은 문서의 이름 규칙을 따르므로
[호출 경로 조회](./inspect.md#typescript)로 확인할 수 있습니다. 각 인스턴스는 기존
[클라이언트 옵션](../reference/client-api.md#clientoptions)을 받으며, URL·인증·Fetch
구현·요청 상태를 독립적으로 가집니다.

페이지를 열 때 클라이언트 코드를 불러오려면 동적 `import()`를 사용하세요.

```ts
const { createClient } = await import("./generated/api/clients/catalog/index.js");
const api = createClient({ baseURL: "https://api.example.test/v1" });
const product = await api.$routes["GET /products/{id}"]({
  path: { id: "product-1" },
});
```

진입점을 가져오면 선택된 코드를 준비하고, 메서드를 호출하면 API 요청을 보냅니다.
애플리케이션 전체에서 모든 클라이언트를 사용해도 페이지마다 필요한 진입점을 나누어
불러올 수 있습니다. 공통 의존 코드는 공유됩니다. 각 클라이언트는 자신에게 필요한
모델과 요청·응답 표현을 가져오므로 다른 클라이언트에서만 쓰는 모델은 초기 의존성에
포함되지 않습니다. 다만 사용하는 모델 자체의 의존성이 크면 필요한 코드도 커질 수
있습니다. 번들러에서 여러 페이지를 하나의 청크로 합치면 의존 코드도 함께 들어갑니다.

## 루트 SDK와 생성 범위

루트 진입점과 `loadOperations`의 생성·배포 비용은
[선택 방식별 비용 비교](./selection-benchmarks.md)에서 확인할 수 있습니다.

루트 선택을 생략하면 루트 SDK는 이름 있는 클라이언트들의 선택을 합쳐 제공합니다.
API 선택은 각 클라이언트 설정에 한 번만 적으면 됩니다.

| 설정 | 결과 |
| --- | --- |
| `[clients.orders.selection]` | `clients/orders`에서 제공하는 API |
| `[selection]` | 루트 SDK와 선택형 진입점에서 제공하는 API |
| 이름 있는 클라이언트만 지정 | 루트 SDK에서 각 클라이언트가 선택한 API의 합집합 제공 |
| 이름 있는 클라이언트와 `[selection]` 모두 생략 | 루트 SDK에서 문서 전체 API 제공 |

루트나 각 클라이언트에서 선택한 API와 실행에 필요한 Link 의존성만 생성합니다.
공통 구현은 한 번만 생성하고 각 클라이언트는 자신의 선택 범위만 제공합니다.
이름 있는 클라이언트 없이 일반 `[selection]`만 사용해도 같은 생성 규칙이 적용됩니다.

루트 SDK에는 주문 목록만 필요하다면 `[selection]`에 `operations = ["listOrders"]`를
추가하세요. `catalog` 클라이언트에는 여전히 선택한 상품 API가 들어갑니다. CLI의
`--operation`, `--route` 덮어쓰기는 루트 선택에 적용됩니다.

생성 설정에 API 배정을 관리하려면 이름 있는 클라이언트를 사용하세요. 애플리케이션
실행 중에 API 목록을 선택하려면 [`loadOperations`](./selective-client.md#prepare)를
사용할 수 있습니다. 이름 있는 진입점은 미리 지정한 선택을 바로 준비합니다.

## Link, 타입, 설정 갱신

OpenAPI Link를 통한 후속 호출도 사용할 수 있습니다. Link 의존성으로만 들어간
대상은 후속 호출 시 불러오며, 출발 클라이언트의 설정으로 요청합니다. 그
클라이언트의 일반 메서드로도 쓰려면 대상을 선택 목록에 직접 넣으세요. 참조 조건은
[Link 지원 범위](../reference/capabilities.md)를 참고하세요.

`Client`, `ComponentInput`, `ComponentOutput` 타입은 사용하는 클라이언트의
진입점에서 가져오세요. [클라이언트별 타입](../reference/typescript-types.md#named-clients)에
예제가 있습니다. `addons = ["server"]`를 함께 쓰면 공용 서버에 루트와 모든
클라이언트가 직접 선택한 API, 해당 API의 콜백, 최상위 웹훅이 포함됩니다.

선택 목록이나 클라이언트 이름을 바꾼 뒤에는 `--incremental`로 다시 생성합니다.
생성기가 관리하는 진입점을 함께 갱신하고 사용자 파일은 보존합니다.
`--check --output`으로 SDK가 현재 설정과 일치하는지도 확인할 수 있습니다.
