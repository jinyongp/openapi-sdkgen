# 생성할 API 찾기

`inspect`로 OpenAPI 문서의 API 목록을 살펴보고, 필요한 API 식별자(`operationId`)와 경로를
찾아 생성 설정에 옮길 수 있습니다.

## 목록 조회와 필터링 {#browse}

JSON이나 YAML 문서의 경로를 지정하면 API 목록을 표로 보여줍니다.

```sh
openapi-sdkgen inspect --input ./openapi.yaml
```

표에는 HTTP 메서드, 경로, API 식별자, 태그, 사용 중단 여부, 요약이 나옵니다.
아래의 숫자는 문서 전체 API 중 검색 조건에 맞는 API 수입니다. API 식별자가
없는 API는 `—`로 표시하며, `GET /tasks/{task-id}`처럼 경로로 지정할 수 있습니다.

경로·API 식별자·요약에서 검색한 뒤 메서드로 범위를 좁혀보세요.

```sh
openapi-sdkgen inspect --input ./openapi.yaml --search tasks --method GET
```

검색어는 대소문자를 구분하지 않습니다. 같은 필터를 여러 번 지정하면 그중 하나에
맞는 API를 찾고, 서로 다른 필터는 모두 만족해야 합니다. 예를 들어
`--tag Tasks --tag Users --method GET`은 태그가 `Tasks` 또는 `Users`인 `GET` API를
찾습니다. 태그와 API 식별자는 원문의 표기와 정확히 일치해야 합니다.
`--deprecated false`를 쓰면 사용 중단 상태가 `false`이거나 선언되지 않은 API를
찾을 수 있습니다.

이름을 알고 있다면 `--operation getTask`나 `--route 'GET /tasks/{task-id}'`로
지정하세요. 두 옵션을 함께 쓰면 어느 쪽이든 일치하는 API를 포함합니다.
경로 인자는 원문에 있는 `{task-id}` 표기를 유지하고, 실제 값은 생성된 클라이언트를
호출할 때 전달합니다. 존재하지 않는 ID나 경로를 지정하면 오류가 나고, 일반 검색의
결과가 0개이면 빈 표나 JSON을 정상적으로 반환합니다.

조회 범위는 입력 문서의 `paths`이며, 경로 항목 참조로 연결된 API도 해당 경로에
포함됩니다. 파일, URL, 표준 입력, 인증과 참조 설정은 생성 명령과 같은
[입력 옵션](../reference/cli.md#input-source-options)을 사용합니다.

## 검색 결과를 생성 설정에 옮기기 {#selection}

원하는 API를 찾았다면 출력 형식을 `selection`으로 바꿔보세요.

```sh
openapi-sdkgen inspect --input ./openapi.yaml --search tasks --method GET --format selection
```

문서에 `GET /tasks/{task-id}`가 있다면 다음 TOML 블록을 출력합니다.

```toml
[selection]
routes = ['GET /tasks/{task-id}']
```

기존 설정 파일의 `[selection]` 블록을 이 결과로 바꾸세요. 입력 문서, 생성 대상,
출력 경로는 그대로 함께 지정합니다.

```toml
source = "./openapi.yaml"
target = "typescript"
output = "./src/generated/api"

[selection]
routes = ["GET /tasks/{task-id}"]
```

선택한 API의 생성 가능 여부를 확인한 뒤 SDK를 생성합니다.

```sh
openapi-sdkgen generate --config ./openapi-sdkgen.toml --check
openapi-sdkgen generate --config ./openapi-sdkgen.toml
```

`selection` 출력에는 검색 결과가 하나 이상 있어야 합니다. API 식별자가 없는
API도 경로로 선택할 수 있습니다. Link 대상과 콜백, 기존 SDK 갱신 방법은
[필요한 API만 생성하기](./selective-client.md#generation)에서 확인하세요.

같은 설정 파일로 목록을 조회할 수도 있습니다.

```sh
openapi-sdkgen inspect --config ./openapi-sdkgen.toml --search tasks
```

`inspect`는 입력·참조 설정을 재사용해 문서 전체를 검색합니다. 설정 파일의
생성용 선택 목록은 `generate`를 실행할 때 적용됩니다. TypeScript 분석이 필요하면
명령에 `--target typescript`를 추가하세요.

## 생성된 클라이언트의 호출 경로 확인하기 {#typescript}

TypeScript 클라이언트에서 API를 어떻게 호출하는지 보려면 생성 대상을 지정합니다.

```sh
openapi-sdkgen inspect --input ./openapi.yaml --target typescript --search tasks
```

추가된 열에는 `api.users.list()` 같은 리소스 호출식과 사용할 수 있는
`resource`, `routes`, `operations`가 표시됩니다. `resource`는 중첩된 클라이언트
경로, `routes`는 `api.$routes["GET /users"]`를 통한 호출, `operations`는 원래 ID를
사용하는 `api.$operations.listUsers`를 뜻합니다.

`taskID`, `body`, `options` 같은 인자는 실제로 전달할 값을 나타냅니다.
여러 보안 방식 중 하나를 선택해야 하는 API에는 `options`를 표시하고,
스트리밍 전용 API에는 `.stream()` 호출식을 보여줍니다.

호출식은 문서 전체 API를 포함한 클라이언트를 기준으로 합니다. 필터는 분석 후
표시할 행을 좁힙니다. 일부 API만 생성하면 같은 리소스 이름을 놓고 충돌하는
API의 구성에 따라 호출 이름이 달라질 수 있습니다. 숨겨진 API와 생성에서 빠진
API는 별도로 표시합니다. 리소스 호출을 제공할 수 없는 경우에는 이유와 함께
사용 가능한 경로·API 식별자를 보여줍니다.

대형 문서에서 API를 찾을 때는 기본 명령부터 사용하세요. 생성 대상 분석은 전체
클라이언트를 살펴보므로, 검색 결과가 적어도 기본 조회보다 시간과 메모리가 더 듭니다.

## 스크립트에서 JSON 사용하기 {#json}

```sh
openapi-sdkgen inspect --input ./openapi.yaml --method GET --format json
```

결과에는 `schemaVersion`, 문서 정보, 전체 API 수인 `total`, 검색 결과 수인
`matched`, 경로와 메서드 순으로 정렬된 `operations` 배열이 들어갑니다.
각 항목은 원래 경로, ID, 태그, 요약, 사용 중단 여부, 원문 위치를 포함합니다.
API 식별자가 선언되지 않았다면 `null`입니다. TypeScript 분석을 요청하면
각 항목에 호출 정보를 담은 `typescript` 객체가 추가되고, 결과에
`analysisScope: "full-document-client"`가 기록됩니다.

표준 출력에는 선택한 형식의 결과가, 표준 오류에는 진단이 나옵니다. 진단도 JSON으로
처리하려면 `--diagnostics-format json`을 지정하세요. 전체 옵션과 JSON 필드는
[CLI 레퍼런스](../reference/cli.md#inspect)에서 확인할 수 있습니다.
