# SDK 생성과 검증

생성 명령에는 일반 생성과 check 모드가 있습니다. 일반 생성은 애플리케이션
소스를 갱신하고, check 모드는 같은 컴파일·target 준비 과정을 실행해 CI, 편집기,
pre-commit에서 결과를 검증합니다.

아래 예시는 [`openapi-sdkgen`](../reference/cli.md)이 PATH에 있다고 가정합니다. 프로젝트 개발
의존성으로 설치했다면 명령 앞에 `pnpm exec`을 붙이세요.

## 새 출력 디렉터리 생성

일반 생성 명령은 새 관리 디렉터리를 만듭니다.

```sh
openapi-sdkgen generate \
  --input ./openapi.yaml \
  --target typescript \
  --output ./src/generated/api
```

TypeScript target은 클라이언트, 생성 타입, 소스 runtime, OpenAPI 메타데이터를
출력합니다. 생성기가 관리하는 파일은 CLI로 다시 생성하며, 출력 디렉터리는
애플리케이션 저장소에서 일반 소스로 관리합니다.

생성 결과는 전체 작업이 성공한 뒤 한 번에 반영됩니다.

## 기존 SDK 다시 생성

최초 생성이 성공한 뒤 같은 관리 디렉터리를 갱신하려면 [`--incremental`](../reference/cli.md#fresh-incremental-and-check-modes)을
사용합니다.

```sh
openapi-sdkgen generate \
  --input ./openapi.yaml \
  --target typescript \
  --output ./src/generated/api \
  --incremental
```

출력 디렉터리의 매니페스트에는 생성기가 소유한 파일이 기록됩니다. 증분 생성은:

- 내용이 같은 생성 파일을 그대로 유지하고,
- 바뀐 파일만 원자적으로 교체하며,
- 이전 매니페스트가 소유한 오래된 파일만 삭제하고,
- 사용자가 매니페스트 밖에 추가한 파일은 보존합니다.

소유한 생성 파일이 수정됐거나 매니페스트가 손상됐거나, 새 생성 경로가
사용자 파일과 충돌하거나, 다른 프로세스가 같은 출력을 갱신 중이면 출력 변경
없이 중단합니다.

자체 완결된 로컬 입력과 생성 fingerprint가 이전 실행과 같다면 변경 없는 증분
생성은 컴파일과 소스 생성을 건너뛸 수도 있습니다.

## 생성 결과 확인

CI, 편집기, pre-commit 작업에서 OpenAPI 문서의 생성 가능 여부를 확인하려면
[`--check`](../reference/cli.md#fresh-incremental-and-check-modes)를 사용합니다.

[`--output`](../reference/cli.md#core-options)을 생략하면 입력 로딩, 컴파일, target 준비까지 실행하며 기존 출력은
그대로 유지됩니다.

```sh
openapi-sdkgen generate \
  --input ./openapi.yaml \
  --target typescript \
  --check
```

생성 코드를 저장소에 함께 커밋한다면 기존 관리 출력 디렉터리도 지정할 수
있습니다. 현재 생성기가 만들 결과와 디렉터리가 일치하는지 비교하며 출력은
그대로 유지합니다.

```sh
openapi-sdkgen generate \
  --input ./openapi.yaml \
  --target typescript \
  --check \
  --output ./src/generated/api
```

생성 내용이나 경로가 달라졌거나 generation fingerprint가 바뀌었거나, 소유 파일이
수정·삭제됐거나, 매니페스트가 잘못됐거나, 새 생성 경로가 사용자 파일과
충돌하면 실패합니다. 한 번의 실행에서는 [`--check`](../reference/cli.md#fresh-incremental-and-check-modes)와 [`--incremental`](../reference/cli.md#fresh-incremental-and-check-modes) 중 하나를
선택합니다.

## CI와 도구에서 diagnostics 사용

기본 출력은 사람이 읽기 좋은 형식입니다. CI나 편집기 같은 도구에서는
[`--diagnostics-format`](../reference/cli.md#diagnostics)으로 버전이 있는 JSON report를 선택할 수 있습니다.

```sh
openapi-sdkgen generate \
  --input ./openapi.yaml \
  --target typescript \
  --check \
  --diagnostics-format json 2> diagnostics.json
```

JSON report에는 `schemaVersion`, severity 개수, diagnostics, 실행하지 못한
pipeline phase가 포함됩니다. URL credential, query, fragment 같은 민감한 source
정보는 렌더링 전에 정리됩니다.

종료 코드는 그대로 검증 결과입니다. 0은 성공을 뜻하며, generation diagnostic,
drift 또는 실행 환경 오류가 있으면 0이 아닌 코드로 종료합니다.

## 입력 소스 선택

[`--input`](../reference/cli.md#input-source-options)은 로컬 JSON/YAML 파일, `file://` URL, HTTP(S) URL, 표준 입력을
뜻하는 `-`를 받을 수 있습니다.

```sh
# 로컬 파일
openapi-sdkgen generate --input ./openapi.yaml --target typescript --check

# 개발 서버
openapi-sdkgen generate \
  --input http://localhost:4010/openapi.json \
  --target typescript \
  --check

# 표준 입력
curl https://api.example.test/openapi.yaml | \
  openapi-sdkgen generate \
    --input - \
    --input-base https://api.example.test/openapi.yaml \
    --target typescript \
    --check
```

[`--input-base`](../reference/cli.md#input-source-options)는 stdin에서 읽은 문서의 상대 참조를 해석할 기준 위치를
지정합니다. 파일과 URL 입력은 각 입력 위치를 base로 사용합니다.

## 인증이 필요한 OpenAPI URL 읽기

보호된 입력 credential은 환경 변수로 전달합니다. [`--http-header-env`](../reference/cli.md#authenticated-http-input)는 요청
header를 환경 변수 이름에 연결하고 생성기가 그 값을 내부에서 읽으므로,
secret 값은 환경 변수에 유지됩니다.

```sh
export OPENAPI_TOKEN='Bearer example-token'

openapi-sdkgen generate \
  --input https://api.internal.example/openapi.yaml \
  --http-header-env Authorization=OPENAPI_TOKEN \
  --target typescript \
  --check
```

`Authorization=OPENAPI_TOKEN`은 `OPENAPI_TOKEN` 환경 변수의 값을 읽으라는
뜻입니다. `$OPENAPI_TOKEN`과 `${OPENAPI_TOKEN}`은 shell expansion 문법이고,
이 옵션에는 환경 변수 이름 자체를 전달합니다.

환경 변수를 현재 명령에만 전달할 수도 있습니다.

```sh
OPENAPI_TOKEN='Bearer example-token' \
  openapi-sdkgen generate \
    --input https://api.internal.example/openapi.yaml \
    --http-header-env Authorization=OPENAPI_TOKEN \
    --target typescript \
    --check
```

환경 변수에는 인증 scheme을 포함한 완성된 header 값을 넣습니다. Header
mapping은 여러 번 지정할 수 있으며 `https://` 루트 입력에서만 허용됩니다.
`http://` 입력은 요청을 보내기 전에 오류로 거부합니다. `Host`, `Cookie`,
`Proxy-Authorization` 같은 transport 관리 header도 CLI가 거부합니다.

mTLS 또는 사설 CA가 필요하면 [`--tls-client-cert`, `--tls-client-key`, `--tls-ca-file`](../reference/cli.md#authenticated-http-input)을 사용합니다. Client certificate와 key는 함께 지정해야 합니다.
루트 OpenAPI URL은 사용자가 지정한 정확한 origin(scheme, host, port) 안에서만
신뢰됩니다. Redirect도 그 origin을 벗어날 수 없으므로 cross-origin redirect에
의존하지 말고 최종 canonical URL을 직접 사용합니다. Credential과 header
mapping도 같은 경계에 묶이며 same-origin 요청에만 적용됩니다.

## 원격 `$ref`를 재현 가능하게 사용

루트 OpenAPI URL과 cross-origin `$ref`는 각각 별도의 입력 정책을 사용합니다.
Cross-origin 원격 참조는 허용할 HTTPS origin을 명시합니다.

최초 실행에서는 [`--allow-remote-ref`](../reference/cli.md#remote-ref-options)로 정확한 HTTPS origin을 허용하고 [`--update-ref-lock`](../reference/cli.md#remote-ref-options)으로 integrity lock을 갱신합니다.

```sh
openapi-sdkgen generate \
  --input ./openapi.yaml \
  --target typescript \
  --allow-remote-ref https://schemas.example.test \
  --update-ref-lock \
  --output ./src/generated/api
```

로컬 루트 파일의 기본 lock 경로는 `<input>.openapi-sdkgen.lock`입니다. 이후
실행에서는 [`--update-ref-lock`](../reference/cli.md#remote-ref-options)을 생략하며, 잠금 파일에 기록된 내용과 실제
참조가 일치해야 생성이 계속됩니다.

[`--offline`](../reference/cli.md#remote-ref-options)은 잠긴 local cache만 사용해 원격 참조를 해석합니다.
URL/stdin 흐름처럼 로컬 입력 파일 이름에서 lock 경로를 만들 수 없거나 별도
위치를 사용하려면 [`--ref-lock <path>`](../reference/cli.md#remote-ref-options)를 지정합니다.

루트 OpenAPI URL에 설정한 인증 정보는 같은 origin에만 적용됩니다.

## Webhook과 Callback 수신 코드 생성

기본 TypeScript target은 외부 API를 호출하는 outbound client입니다. OpenAPI
문서가 애플리케이션이 받아야 할 Webhook이나 Callback도 정의한다면 선택적인
server artifact set을 추가합니다.

```sh
openapi-sdkgen generate \
  --input ./openapi.yaml \
  --target typescript \
  --with server \
  --output ./src/generated/api
```

[`--with server`](../reference/cli.md#typescript-server-add-on)는 Fetch 기반 handler/router 진입점을 추가합니다. HTTP listener,
framework, 실제 경로, 배포 환경 연결은 애플리케이션에서 구성합니다. 자세한
사용법은 [Webhook과 Callback 수신](./server.md)을 참고하세요.

Outbound operation으로 구성된 문서는 기본 client artifact set을 사용합니다.

## 필수 사용자 정의 JSON Schema vocabulary

필수 custom JSON Schema vocabulary를 선언한 문서는 신뢰한 로컬
[`--schema-extension`](../reference/cli.md#schema-extension)이 필요합니다. Schema extension은 custom JSON Schema 의미를
처리하고, OpenAPI `x-*` 필드는 SDK 편의 기능을 설정합니다.

매니페스트, SHA-256, JSON-RPC lowering, integrity lock 흐름과 보안 경계는
[사용자 정의 JSON Schema vocabulary](./schema-vocabularies.md)에서 설명합니다.

전체 flag를 빠르게 찾으려면 [CLI 레퍼런스](../reference/cli.md)를 참고하세요.
