# 이름 있는 클라이언트

[이름 있는 클라이언트 명세](/examples/named-clients.json)를 `openapi.yaml`로 저장하세요.
각 클라이언트는 독립된 import 패키지에서 선택한 작업 API를 제공합니다.

```toml
source = "./openapi.yaml"
target = "golang"
output = "./internal/api"

[golang]
import_path = "example.com/consumer/internal/api"

[clients.orders.selection]
operations = ["listOrders", "createOrder"]

[clients.catalog.selection]
routes = ["GET /products/{id}"]
```

```sh
openapi-sdkgen generate --config ./openapi-sdkgen.toml
```

`example.com/consumer/internal/api/clients/orders` 또는
`example.com/consumer/internal/api/clients/catalog`를 import하고 해당 패키지의
`ClientOptions`로 `NewClient`를 구성하세요. 루트처럼 `context.Context`로 호출하며 의존성은 사용자 모듈이 관리합니다.

이름은 소문자로 시작하고 ASCII 소문자·숫자·하이픈을 사용하며 최대 64자입니다.
Windows 예약 장치 이름은 사용할 수 없습니다. 선택 목록은 비어 있으면 안 됩니다.
알 수 없는 설정 필드·작업이나 숨긴 API를 지정하면 생성 오류입니다.

## 루트 선택

| 설정 | 루트 SDK |
| --- | --- |
| 루트 선택 없이 이름 있는 클라이언트 사용 | 각 선택의 합집합 |
| 명시적 `[selection]` | 루트 선택만 |
| 선택과 이름 있는 클라이언트 모두 없음 | 전체 명세 |

CLI `--operation`·`--route`는 루트 선택을 덮어씁니다. 생성 범위에는 루트·이름 있는 클라이언트의
선택과 필요한 Link 의존성이 포함됩니다. Link만을 위한 대상은 내부에 남습니다.
클라이언트별 URL·인증·요청 상태는 독립적이며 공통 소스 패키지는 공유합니다.
Go는 정적 import를 사용합니다. 실행 중 dynamic import나 JavaScript 청크 설정은 필요하지 않습니다.
[선택 비용](./selection-cost.md)과 [생성 모드](./generate.md)를 확인하세요.
