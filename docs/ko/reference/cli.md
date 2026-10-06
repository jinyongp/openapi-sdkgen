# CLI 레퍼런스

이 페이지는 명령 문법과 옵션 동작을 빠르게 찾는 레퍼런스입니다. 실제 작업
흐름은 [시작하기](../guide/getting-started.md)와
[SDK 생성과 검증](../guide/generate.md)에서 설명합니다.

## 도움말과 버전

```sh
openapi-sdkgen --help
openapi-sdkgen inspect --help
openapi-sdkgen generate --help
openapi-sdkgen --version
```

설치된 CLI가 지원하는 생성 대상, 추가 기능, 옵션은 `generate --help`에서 확인할 수
있습니다.

## `inspect` {#inspect}

```text
openapi-sdkgen inspect [options]
```

API 목록을 조회하고 필터링하거나, 선택 생성에 쓸 경로를 출력합니다.
사용 흐름은 [생성할 API 찾기](../guide/inspect.md)를 참고하세요.
v10.0.0부터 제공되는 명령입니다.

| 옵션 | 의미 |
| --- | --- |
| `--input <source>` | JSON/YAML 파일, `file://` 또는 HTTP(S) URL, 표준 입력을 뜻하는 `-` |
| `--config <path>` | 생성용 TOML 파일의 입력·참조 설정 재사용 |
| `--search <text>` | 경로·API 식별자·요약의 부분 문자열 검색. 대소문자 구분 없이 반복 가능 |
| `--method <method>` | HTTP 메서드. 표준 메서드는 대소문자 구분 없이, 사용자 정의 메서드는 정확히 일치. 반복 가능 |
| `--tag <tag>` | 정확히 일치하는 태그. 반복 가능 |
| `--operation <operationId>` | 정확히 일치하는 API 식별자. 반복 가능 |
| `--route <METHOD /path>` | 정확히 일치하는 메서드·OpenAPI 경로. 반복 가능 |
| `--deprecated true\|false` | 사용 중단 여부. 미선언 상태는 `false`로 조회 |
| `--format table\|json\|selection` | 기본 표, 버전이 있는 JSON, `[selection]` TOML 블록 |
| `--target typescript` | 문서 전체 클라이언트의 호출 경로를 분석한 뒤 조회 필터 적용 |
| `--schema-extension <path>` | 명시적인 TypeScript 분석에 쓸 스키마 어휘 매니페스트. 반복 가능 |
| `--diagnostics-format human\|json` | 표준 오류로 출력할 진단 형식 |
| `--diagnostic-mode fail-fast\|collect` | 진단 수집 방식 |

설정 파일에 `source`가 없다면 `--input`이 필수입니다. 입력·참조의 상대 경로와
CLI 덮어쓰기 규칙은 생성 명령과 같습니다. 설정 파일의 `selection`, `clients`, `target`,
`output`, `addons`는 생성에 적용됩니다. 분석은 CLI에 생성 대상을 지정해서 요청하며,
이때 설정 파일의 스키마 확장도 재사용합니다.

같은 필터의 여러 값은 그중 하나와 일치하면 되고, 서로 다른 필터는 모두 만족해야
합니다. API 식별자와 경로는 하나의 선택 그룹입니다. 존재하지 않는 정확한 ID나
경로를 지정하면 실패합니다. 일반 검색의 결과가 없으면 표·JSON은 정상적으로 반환하고,
`selection` 출력은 하나 이상의 API를 요구합니다.

인증, TLS, 표준 입력 기준 경로, 참조 허용 목록·잠금 파일·오프라인은
[생성 입력 옵션](#input-source-options)을 공유합니다. 설정 파일, 참조 잠금 파일과 SDK 출력은
유지하며, 참조 캐시는 기존 입력 정책에 따라 사용합니다.

### JSON 결과

| 필드 | 값 |
| --- | --- |
| `schemaVersion` | `1` |
| `document` | `title`, 문서의 `version`, `openapiVersion` |
| `total`, `matched` | 필터 적용 전 전체 API 수, 검색 결과 수 |
| `operations` | 경로·메서드 순으로 정렬된 항목 |
| `documentsRead` | 목록 조회에서 측정한 디코딩 문서 수. 측정된 경우에 포함 |
| `target`, `analysisScope` | 명시적인 분석에 포함: `typescript`, `full-document-client` |

각 항목에는 `method`, `path`, `route`, 문자열 또는 `null`인 `operationId`,
`tags`, `summary`, `deprecated`, `source`, 입력 문서에 연결된 JSON `pointer`가
들어갑니다. TypeScript 분석 항목의 `typescript` 객체에는 `exposed`, `hidden`,
`omitted` 중 하나인 `status`, 호출식 또는 `null`인 `resourceCall`, `$routes`와
`$operations`로 공개 호출할 수 있는지를 나타내는 `routes`, `operations`가 있습니다.
공개 API의 리소스 호출식이 없으면 `resourceOmission`에 이유를 담습니다.

표준 출력에는 선택한 형식의 결과만, 표준 오류에는 경고와 오류를 출력합니다.
입력·API 선언·생성 대상 분석·출력 오류는 0이 아닌 종료 코드를 반환합니다.
SDK 생성 가능 여부는 `generate --check`로 확인하세요.

## `generate`

```text
openapi-sdkgen generate [options]
```

`--input`과 `--target`은 `--config`에서 제공하지 않는 한 필수입니다. 일반
생성은 `--output`을 요구하고, `--check`는 `--output` 없이도 실행할 수 있습니다.

<span id="core-options"></span>

### 옵션

| 옵션 | 의미 |
| --- | --- |
| `--config <path>` | 반복 생성 설정을 명시적인 TOML 파일 하나에서 읽음. 같은 설정의 CLI 옵션이 우선 |
| `--input <source>` | OpenAPI 3.0.x, 3.1.x, 3.2.x JSON/YAML 입력. 로컬 경로, `file://` URL, HTTP(S) URL, 표준 입력을 뜻하는 `-` |
| `--target typescript` | TypeScript SDK 생성 |
| `--output <directory>` | 생성 디렉터리. `--check`와 함께 쓰면 기존 생성 파일 검증 |
| `--check` | SDK 생성 가능 여부 확인. `--output`을 지정하면 기존 생성 파일이 최신인지도 비교. 기존 파일은 유지 |
| `--incremental` | 기존 매니페스트에 기록된 출력 갱신 |
| `--typecheck` | 생성 소스의 `@ts-nocheck`를 제거해 애플리케이션에서 타입 검사. 기본값은 `false` |
| `--with <addon>` | `metadata` 또는 `server` 추가. 반복 가능 |
| `--operation <operationId>` | 정확한 API 식별자로 생성할 API 지정. 반복 가능 |
| `--route <METHOD /path>` | 정확한 메서드·경로로 생성할 API 지정. 반복 가능 |
| `--diagnostics-format human|json` | 사람이 읽는 진단 또는 버전이 있는 JSON 진단 선택 |
| `--diagnostic-mode fail-fast|collect` | `fail-fast`는 생성을 막는 오류에서 중단. `collect`는 확인 가능한 나머지 문제도 수집 |
| `--fail-on-resource-omission` | 생성된 API의 리소스 메서드가 생략되면 경고 대신 실패 |

한 번의 실행에서는 `--check`와 `--incremental` 중 하나를 선택합니다.
`--output`에는 생성 파일을 저장할 디렉터리를 지정합니다.

<span id="project-configuration"></span>

## 프로젝트 설정 파일

같은 생성 설정을 프로젝트에서 반복해서 사용한다면 `--config <path>`로 TOML
파일을 명시적으로 읽을 수 있습니다.

```toml
source = "./openapi.yaml"
target = "typescript"
output = "./src/generated/api"
addons = ["server"]
incremental = true
diagnostics_format = "human"
diagnostic_mode = "fail-fast"
fail_on_resource_omission = true

[input]
tls_ca_file = "./certs/internal-ca.pem"

[input.headers_from_env]
Authorization = "OPENAPI_TOKEN"

[references]
allow = ["https://schemas.example.com"]
lock = "./openapi.refs.lock"

[schema]
extensions = ["./schema-extensions/example.json"]
```

```sh
openapi-sdkgen generate --config ./openapi-sdkgen.toml
```

설정 파일은 반복 생성에 필요한 다음 키를 지원합니다.

| 설정 키 | 대응 CLI |
| --- | --- |
| `source` | `--input` |
| `target` | `--target` |
| `output` | `--output` |
| `addons` | 반복 가능한 `--with` |
| `selection.operations` | 반복 가능한 `--operation` |
| `selection.routes` | 반복 가능한 `--route` |
| `clients.<name>.selection.operations` | 설정 파일 전용 클라이언트 API 식별자 목록 |
| `clients.<name>.selection.routes` | 설정 파일 전용 클라이언트 경로 목록 |
| `incremental` | `--incremental` |
| `typescript.typecheck` | `--typecheck`. 생성된 모든 TypeScript 파일에 적용 |
| `diagnostics_format` | `--diagnostics-format` |
| `diagnostic_mode` | `--diagnostic-mode` |
| `fail_on_resource_omission` | `--fail-on-resource-omission` |
| `input.base` | `--input-base` |
| `input.headers_from_env` | 반복 가능한 `--http-header-env` |
| `input.tls_client_cert` | `--tls-client-cert` |
| `input.tls_client_key` | `--tls-client-key` |
| `input.tls_ca_file` | `--tls-ca-file` |
| `references.allow` | 반복 가능한 `--allow-remote-ref` |
| `references.lock` | `--ref-lock` |
| `references.offline` | `--offline` |
| `schema.extensions` | 반복 가능한 `--schema-extension` |

설정 파일은 자동 탐색하지 않습니다. 현재 디렉터리, 상위 디렉터리, 홈
디렉터리, 전역 설정 위치를 검색하지 않으며 반드시 `--config`로 지정해야 합니다.
설정 파일 없이 CLI 옵션만 지정해 실행할 수도 있습니다.

설정 파일 안의 상대 로컬 경로는 프로세스의 현재 디렉터리가 아니라 **설정 파일이
있는 디렉터리**를 기준으로 해석합니다. 로컬 OpenAPI 입력, 출력, `input.base`,
TLS 인증서·키·CA, 참조 잠금 파일, 스키마 확장 매니페스트가 이 규칙을
따릅니다. HTTP(S) URL, `file://` URL, 표준 입력 `-`은 기존 입력 의미를
그대로 사용합니다.

같은 설정이 CLI에도 있으면 CLI가 우선합니다. 반복 가능한 옵션은 CLI에서 한 번이라도
명시하면 설정 값에 추가하는 대신 **설정 목록 전체를 대체**합니다. 선택할 API 식별자와 경로, 추가 기능,
원격 참조 출처, 스키마 확장, HTTP 헤더와 환경 변수의 연결에 같은
규칙을 적용합니다. `--offline=false`처럼 명시한 불리언 값도 설정 값을
대체합니다.

HTTP 헤더 인증 정보는 비밀 값이 아니라 **환경 변수 이름만** 설정에
기록합니다.

```toml
[input.headers_from_env]
Authorization = "OPENAPI_TOKEN"
```

실제 토큰은 환경 변수에 보관합니다. `--config`는 설정 파일 자체를 선택하므로
설정 키가 아닙니다. `--check`, `--update-ref-lock`, `--help`는 실행 시점의
동작을 바꾸는 옵션이므로 CLI에서만 사용합니다. `--fail-on-resource-omission` 같은
지속적인 생성 정책은 대응 TOML 키를 제공합니다. 알 수 없는 TOML 키는
오류로 처리해 오타가 조용히 무시되지 않게 합니다.

## 생성할 API 선택 {#api-selection}

API 식별자와 경로를 함께 지정해 필요한 API만 생성할 수 있습니다.

```sh
openapi-sdkgen generate --input ./openapi.yaml --target typescript \
  --output ./src/generated/api --operation listTasks --route 'GET /tasks/{task-id}'
```

두 목록 중 하나에 포함되면 생성 대상이 됩니다. 경로 인자는 원문 이름과 중괄호를
유지하고, 표준 메서드는 `GET`, `POST`처럼 대문자로 적습니다. 이름은 대소문자를
구분하며 정확히 일치해야 합니다. 선택 설정을 생략하면 전체 API를 생성하고,
명시한 목록이 비어 있거나 이름을 찾을 수 없으면 오류가 발생합니다.

TOML에서는 `[selection]`의 `operations`, `routes` 배열을 사용합니다.
CLI의 `--operation` 목록은 `selection.operations`만 대체하고 설정 파일의 경로는
유지합니다. `--route` 목록은 `selection.routes`만 대체합니다.

Link 의존 코드, 서버 지원, 기존 SDK 갱신 방법은
[필요한 API만 생성하기](../guide/selective-client.md#generation)에서 설명합니다.

### 이름 있는 클라이언트 {#named-clients}

`[clients.<name>.selection]`으로 `clients/<name>/index.ts`에 API를 배정합니다.

```toml
[clients.orders.selection]
operations = ["listOrders", "createOrder"]
```

각 클라이언트에는 비어 있지 않은 선택 목록이 필요하며, 개별 설정은 `selection`만
지원합니다. 입력 문서·생성 대상·출력·추가 기능은 공유합니다. 루트 `[selection]`과
CLI 선택 덮어쓰기는 별도로 적용합니다. 루트 `[selection]`을 생략하면 이름 있는
클라이언트들의 선택을 합쳐 루트에 제공합니다. 선택한 API와 필요한 Link 의존성만
생성하고 공통 구현은 공유합니다. 이름은 영문 소문자로 시작하고 소문자·숫자·하이픈으로 구성한 64자 이내의
이름을 사용합니다. Windows 장치 이름은 예약되어 있습니다. 모듈 사용, 후속 호출, 갱신 예제는
[기능별 클라이언트 생성](../guide/named-clients.md)을 참고하세요.

<span id="fresh-incremental-and-check-modes"></span>

<span id="fresh-incremental-check-모드"></span>

## 최초 생성, 증분 생성, 검증

최초 생성은 새 출력 디렉터리를 만듭니다.

```sh
openapi-sdkgen generate \
  --input ./openapi.yaml \
  --target typescript \
  --output ./src/generated/api
```

최초 성공 생성 이후 같은 관리 디렉터리를 갱신하려면 `--incremental`을
사용합니다. 출력 매니페스트가 생성기가 교체하거나 삭제할 수 있는 파일을
관리하며, 매니페스트 밖의 사용자 파일은 보존합니다.

`--output` 없이 `--check`를 사용하면 문서의 SDK 생성 가능 여부를 확인합니다.
기존 출력 디렉터리는 그대로 유지됩니다.

```sh
openapi-sdkgen generate \
  --input ./openapi.yaml \
  --target typescript \
  --check
```

저장소에 생성 소스를 함께 커밋한다면 기존 출력 디렉터리를 지정해 생성 결과 차이를
확인할 수 있습니다.

```sh
openapi-sdkgen generate \
  --input ./openapi.yaml \
  --target typescript \
  --check \
  --output ./src/generated/api
```

생성 내용·경로의 차이, 입력·설정 식별값 변경, 수정되거나 사라진 생성 파일,
잘못된 매니페스트, 사용자 파일과의 경로 충돌이 있으면 `--check`가 실패합니다.

권장 CI 및 재생성 흐름은
[SDK 생성과 검증](../guide/generate.md)을 참고하세요.

<span id="diagnostics"></span>

## 진단

기본 형식은 `human`입니다. 다른 도구가 구조화된 결과를 읽어야 한다면 JSON을
선택합니다.

```sh
openapi-sdkgen generate \
  --input ./openapi.yaml \
  --target typescript \
  --check \
  --diagnostics-format json 2> diagnostics.json
```

JSON 보고서에는 오류·경고 수, 문제별 진단, 검사 완료 범위를 나타내는
`coverage`가 담깁니다. 현재 형식은 `schemaVersion: 4`입니다. 각 진단에는
고정된 식별자 `id`, 위치, 메시지가 있습니다. JSON을 파싱할 때는
`schemaVersion`에 맞춰 처리하세요.

한 번에 여러 문제를 확인하려면 `--diagnostic-mode collect`를 사용합니다.
확인 가능한 나머지 검사도 계속하며, 생성을 막는 오류가 있으면 실패 종료하고
SDK 생성을 중단합니다. 기본값은 `fail-fast`입니다.

진단의 영향을 확인하려면 `severity`, `scope`, `effect`를 함께 읽으세요.
`scope: document`와 `effect: block`은 SDK 생성을 중단합니다.
`scope: operation`과 `effect: omit-operation`은 해당 API 작업을 생략하며,
`scope: capability`와 `effect: omit-capability`는 해당 보조 기능을 생략합니다.

진단 보고서는 표준 오류에 기록되며, 생성 파일은 요청한 경우에만 출력
디렉터리에 기록됩니다.

<span id="input-source-options"></span>

## 입력 소스 옵션

### `--input <source>`

다음 입력을 사용할 수 있습니다.

```sh
# 로컬 파일
openapi-sdkgen generate --input ./openapi.yaml --target typescript --check

# 파일 URL
openapi-sdkgen generate --input file:///workspace/openapi.yaml --target typescript --check

# HTTP(S) URL
openapi-sdkgen generate --input https://api.example.test/openapi.yaml --target typescript --check

# 표준 입력
cat ./openapi.yaml | openapi-sdkgen generate --input - --target typescript --check
```

### `--input-base <source>`

표준 입력으로 읽은 문서의 상대 참조에 기준 위치가 필요할 때 사용합니다.

```sh
curl https://api.example.test/openapi.yaml | \
  openapi-sdkgen generate \
    --input - \
    --input-base https://api.example.test/openapi.yaml \
    --target typescript \
    --check
```

파일과 URL 입력은 각 입력 위치를 기준 위치로 사용합니다.

<span id="authenticated-http-input"></span>

## 인증이 필요한 HTTP(S) 입력

### `--http-header-env <header=env>`

HTTP 헤더 이름을 환경 변수 이름에 연결합니다. 그 환경 변수의 값 전체가 헤더 값이 됩니다.

```sh
export OPENAPI_TOKEN='Bearer example-token'

openapi-sdkgen generate \
  --input https://api.internal.example/openapi.yaml \
  --http-header-env Authorization=OPENAPI_TOKEN \
  --target typescript \
  --check
```

`Authorization=OPENAPI_TOKEN`처럼 환경 변수 이름 자체를 전달합니다.
`$OPENAPI_TOKEN`과 `${OPENAPI_TOKEN}`은 셸 확장 문법이라 비밀 값이
명령행 인자로 전달됩니다. 이 옵션에는 `Header-Name=ENV_VAR` 형태로 환경 변수 이름을 적으세요.

옵션은 반복할 수 있습니다. 환경 변수는 존재해야 하고 비어 있지 않은 유효한
헤더 값을 가져야 합니다. 같은 헤더를 중복 지정하면 오류입니다. `Host`,
`Cookie`, 연결 관리 헤더, 프록시 인증 헤더 등 전송 계층이 관리하는
헤더는 설정할 수 없습니다.

연결된 요청 헤더는 `https://` 루트 입력에서만 사용할 수 있습니다.
`http://` 입력에 `--http-header-env`를 지정하면 요청을 보내기 전에 오류로 거부합니다.

### `--tls-client-cert <path>`, `--tls-client-key <path>`

HTTPS 입력에 사용할 PEM 클라이언트 인증서와 개인 키를 함께 지정합니다.

### `--tls-ca-file <path>`

HTTPS 입력에 추가로 신뢰할 PEM CA를 지정합니다. 기존 인증서 검증은 그대로
적용됩니다.

연결된 헤더, 클라이언트 인증서, 사설 CA는 루트 OpenAPI 출처에 묶인
인증 설정입니다. 루트 리디렉션은 정확히 같은 출처(프로토콜, 호스트, 포트)
안에서만 허용되며 인증 설정도 같은 출처의 요청에만 적용됩니다.
다른 출처의 원격 참조는 별도의 `--allow-remote-ref` 정책을 사용합니다.

## OpenAPI 원문 포함 {#metadata-addon}

문서 도구나 스크립트에서 원문을 읽으려면 생성할 때 `--with metadata`를 추가하세요.

```sh
openapi-sdkgen generate --input ./openapi.yaml --target typescript \
  --with metadata --output ./src/generated/api
```

설정 파일에서는 `addons = ["metadata"]`로 지정합니다. 서버 핸들러도 함께
생성하려면 `addons = ["server", "metadata"]`를 사용하거나 두 옵션을 각각
`--with`로 지정하세요. 생성되는 값과 다음 메이저 버전의 이전 방법은
[OpenAPI 메타데이터](./client-api.md#openapi-메타데이터)를 참고하세요.

<span id="typescript-server-add-on"></span>

## TypeScript 서버 확장

### `--with server`

OpenAPI 웹훅과 콜백을 받기 위한 Fetch 기반 수신 처리 함수/라우터
생성 파일을 추가합니다.

```sh
openapi-sdkgen generate \
  --input ./openapi.yaml \
  --target typescript \
  --with server \
  --output ./src/generated/api
```

Fetch 기반 수신 명세를 생성하며, HTTP 서버 리스너와 프레임워크 연결은
애플리케이션에서 구성합니다. 문서에 선언된 웹훅/콜백을 받는 경우
사용하세요.

자세한 사용법은 [웹훅과 콜백 수신](../guide/server.md)을 참고하세요.

<span id="remote-ref-options"></span>

## 원격 `$ref` 옵션

원격 참조는 허용 목록과 무결성 잠금 파일로 관리합니다. 허용한 출처의 문서만
읽으며, 저장된 해시와 실제 내용이 일치해야 같은 입력으로 다시 생성할 수 있습니다.

| 옵션 | 의미 |
| --- | --- |
| `--allow-remote-ref <origin>` | 정확한 HTTPS 출처 하나 허용. 반복 가능 |
| `--ref-lock <path>` | 원격 참조/스키마 확장 무결성 잠금 파일 경로 지정 |
| `--update-ref-lock` | 컴파일 성공 후 참조 문서·확장 실행 파일의 해시 생성 또는 갱신 |
| `--offline` | 네트워크 없이 이미 잠긴 캐시의 원격 참조만 사용 |

로컬 입력 파일의 기본 잠금 파일 경로는 `<input>.openapi-sdkgen.lock`입니다.

다른 출처의 참조를 최초로 사용할 때:

```sh
openapi-sdkgen generate \
  --input ./openapi.yaml \
  --target typescript \
  --allow-remote-ref https://schemas.example.test \
  --update-ref-lock \
  --output ./src/generated/api
```

이후 실행에서는 `--update-ref-lock`을 생략하며 원격 문서 내용이 잠금 파일과
일치해야 합니다.

HTTP(S) 루트 입력은 같은 출처의 상대 참조를 사용할 수 있습니다. URL/표준 입력
루트에서 원격 `$ref`를 가져오려면 기본 잠금 파일 경로를 만들 로컬 파일 이름이
없으므로 `--ref-lock`을 지정해야 합니다. 다른 출처는 추가로
`--allow-remote-ref`가 필요합니다. 루트 출처 인증 정보는 같은 출처의 요청에만
적용됩니다.

<span id="schema-extension"></span>

## 스키마 확장

### `--schema-extension <manifest>`

필수 사용자 정의 JSON Schema 어휘를 처리할 신뢰된 로컬 컴파일러를
등록합니다. 여러 매니페스트가 필요하면 옵션을 반복합니다.

스키마 확장은 필수 사용자 정의 JSON Schema 어휘를 처리하고,
[OpenAPI `x-*` 확장](./extensions.md)은 SDK 편의 기능을 설정합니다.

스키마 확장 매니페스트는 버전, 어휘 URI, 실행 파일과 인자,
실행 파일 SHA-256을 선언합니다. 확장 실행 파일의 해시는 원격 참조와 같은
무결성 잠금 파일을 사용하므로 최초로 신뢰할 때 `--update-ref-lock`이 필요합니다.

로컬 OpenAPI 파일은 기본 잠금 파일 경로를 자동으로 만들 수 있습니다. URL이나 표준 입력
루트 입력에서 스키마 확장을 사용하려면 `--ref-lock`을 지정해야
합니다.

실행 파일은 생성 중에 사용자 정의 어휘를 표준 JSON Schema로 변환합니다.
신뢰된 로컬 코드로서 생성 프로세스와 같은 권한으로 실행되며, 생성된
소스에는 변환된 스키마 의미가 반영됩니다.

매니페스트 형식, 신뢰 범위, 최초/이후 실행 흐름은
[사용자 정의 JSON Schema 어휘](../guide/schema-vocabularies.md)를
참고하세요.
