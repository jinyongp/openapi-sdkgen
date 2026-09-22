# CLI 레퍼런스

이 페이지는 명령 문법과 flag 동작을 빠르게 찾는 레퍼런스입니다. 실제 작업
흐름은 [시작하기](../guide/getting-started.md)와
[SDK 생성과 검증](../guide/generate.md)에서 설명합니다.

## 도움말과 버전

```sh
openapi-sdkgen --help
openapi-sdkgen generate --help
openapi-sdkgen --version
```

설치된 CLI가 지원하는 target, add-on, flag는 `generate --help`에서 확인할 수
있습니다.

## `generate`

```text
openapi-sdkgen generate [options]
```

`--input`과 `--target`은 필수입니다. 일반 생성은 `--output`을 요구하고,
`--check`는 `--output` 없이도 실행할 수 있습니다.

<span id="core-options"></span>

### 옵션

| 옵션 | 의미 |
| --- | --- |
| `--input <source>` | OpenAPI 3.0.x, 3.1.x, 3.2.x JSON/YAML 입력. 로컬 경로, `file://` URL, HTTP(S) URL, stdin을 뜻하는 `-` |
| `--target typescript` | TypeScript target 생성 |
| `--output <directory>` | 생성 디렉터리. `--check`와 함께 쓰면 기존 managed output 검증 |
| `--check` | compile/prepare를 실행하고 출력은 유지. `--output`이 있으면 managed-output drift도 검증 |
| `--incremental` | 기존 manifest-owned output 갱신 |
| `--with <addon>` | target-specific artifact 추가. 현재 `server`, 반복 가능 |
| `--diagnostics-format human|json` | 사람이 읽는 진단 또는 버전이 있는 JSON 진단 선택 |

한 번의 실행에서는 `--check`와 `--incremental` 중 하나를 선택합니다.
`--output`은 디렉터리 경로를 받으며 stdout 출력 모드는 제공하지 않습니다.

<span id="fresh-incremental-and-check-modes"></span>

## Fresh, incremental, check 모드

Fresh generation은 새 출력 디렉터리를 만듭니다.

```sh
openapi-sdkgen generate \
  --input ./openapi.yaml \
  --target typescript \
  --output ./src/generated/api
```

최초 성공 생성 이후 같은 관리 디렉터리를 갱신하려면 `--incremental`을
사용합니다. 출력 매니페스트가 생성기가 교체하거나 삭제할 수 있는 파일을
관리하며, 매니페스트 밖의 사용자 파일은 보존합니다.

`--output` 없이 `--check`를 사용하면 compiler/target preflight를 수행하고 기존
출력 디렉터리는 그대로 유지됩니다.

```sh
openapi-sdkgen generate \
  --input ./openapi.yaml \
  --target typescript \
  --check
```

저장소에 생성 소스를 함께 커밋한다면 기존 managed output을 지정해 drift를
확인할 수 있습니다.

```sh
openapi-sdkgen generate \
  --input ./openapi.yaml \
  --target typescript \
  --check \
  --output ./src/generated/api
```

생성 내용·경로 drift, generation fingerprint 변경, 수정되거나 사라진 소유 파일,
잘못된 매니페스트, unmanaged path 충돌이 있으면 check가 실패합니다.

권장 CI 및 재생성 흐름은
[SDK 생성과 검증](../guide/generate.md)을 참고하세요.

## Diagnostics

기본 형식은 `human`입니다. 다른 도구가 구조화된 결과를 읽어야 한다면 JSON을
선택합니다.

```sh
openapi-sdkgen generate \
  --input ./openapi.yaml \
  --target typescript \
  --check \
  --diagnostics-format json 2> diagnostics.json
```

JSON envelope에는 version, severity 개수, diagnostics, 실행하지 못한 phase가
포함됩니다. Diagnostic report는 stderr에 기록되며, 생성 artifact는 요청한
경우에만 output 디렉터리에 기록됩니다.

<span id="input-source-options"></span>

## 입력 소스 옵션

### `--input <source>`

다음 입력을 사용할 수 있습니다.

```sh
# 로컬 파일
openapi-sdkgen generate --input ./openapi.yaml --target typescript --check

# file URL
openapi-sdkgen generate --input file:///workspace/openapi.yaml --target typescript --check

# HTTP(S) URL
openapi-sdkgen generate --input https://api.example.test/openapi.yaml --target typescript --check

# stdin
cat ./openapi.yaml | openapi-sdkgen generate --input - --target typescript --check
```

### `--input-base <source>`

stdin으로 읽은 문서의 상대 참조에 기준 위치가 필요할 때 사용합니다.

```sh
curl https://api.example.test/openapi.yaml | \
  openapi-sdkgen generate \
    --input - \
    --input-base https://api.example.test/openapi.yaml \
    --target typescript \
    --check
```

파일과 URL 입력은 각 입력 위치를 base로 사용합니다.

<span id="authenticated-http-input"></span>

## 인증이 필요한 HTTP(S) 입력

### `--http-header-env <header=env>`

HTTP header 이름을 환경 변수 이름에 연결합니다. 그 환경 변수의 값 전체가 header 값이 됩니다.

```sh
export OPENAPI_TOKEN='Bearer example-token'

openapi-sdkgen generate \
  --input https://api.internal.example/openapi.yaml \
  --http-header-env Authorization=OPENAPI_TOKEN \
  --target typescript \
  --check
```

`Authorization=OPENAPI_TOKEN`처럼 환경 변수 이름 자체를 전달합니다.
`$OPENAPI_TOKEN`과 `${OPENAPI_TOKEN}`은 shell expansion 문법이라 secret 값이
argv에 들어가고 `Header-Name=ENV_VAR` 계약에도 맞지 않습니다.

옵션은 반복할 수 있습니다. 환경 변수는 존재해야 하고 비어 있지 않은 유효한
header 값을 가져야 합니다. 같은 header를 중복 지정하면 오류입니다. `Host`,
`Cookie`, connection 관리 header, proxy authorization 등 transport가 관리하는
header는 설정할 수 없습니다.

Mapping된 request header는 `https://` 루트 입력에서만 사용할 수 있습니다.
`http://` 입력에 `--http-header-env`를 지정하면 요청을 보내기 전에 오류로 거부합니다.

### `--tls-client-cert <path>`, `--tls-client-key <path>`

HTTPS 입력에 사용할 PEM client certificate와 private key를 함께 지정합니다.

### `--tls-ca-file <path>`

HTTPS 입력에 추가로 신뢰할 PEM CA를 지정합니다. 기존 certificate 검증은 그대로
적용됩니다.

Mapping된 header, client certificate, private CA는 루트 OpenAPI origin에 묶인
보호 credential입니다. 루트 redirect는 정확히 같은 origin(scheme, host, port)
안에서만 허용되며 보호 transport 설정도 same-origin 요청에만 적용됩니다.
Cross-origin 원격 참조는 별도의 `--allow-remote-ref` 정책을 사용합니다.

## TypeScript server add-on

### `--with server`

OpenAPI Webhook과 Callback을 받기 위한 Fetch 기반 inbound handler/router
artifact를 추가합니다.

```sh
openapi-sdkgen generate \
  --input ./openapi.yaml \
  --target typescript \
  --with server \
  --output ./src/generated/api
```

Fetch 기반 inbound contract를 생성하며, HTTP listener와 framework 연결은
애플리케이션에서 구성합니다. 문서에 선언된 Webhook/Callback을 받는 경우
사용하세요.

자세한 사용법은 [Webhook과 Callback 수신](../guide/server.md)을 참고하세요.

<span id="remote-ref-options"></span>

## 원격 `$ref` 옵션

원격 참조는 allowlist와 integrity lock을 사용해 fail-closed, reproducible
방식으로 처리합니다.

| 옵션 | 의미 |
| --- | --- |
| `--allow-remote-ref <origin>` | 정확한 HTTPS origin 하나 허용. 반복 가능 |
| `--ref-lock <path>` | remote-reference/schema-extension integrity lock 경로 지정 |
| `--update-ref-lock` | 성공한 compile 후 reference/extension digest 생성 또는 갱신 |
| `--offline` | 네트워크 없이 이미 잠긴 cache의 remote reference만 사용 |

로컬 입력 파일의 기본 lock 경로는 `<input>.openapi-sdkgen.lock`입니다.

Cross-origin 참조를 최초로 사용할 때:

```sh
openapi-sdkgen generate \
  --input ./openapi.yaml \
  --target typescript \
  --allow-remote-ref https://schemas.example.test \
  --update-ref-lock \
  --output ./src/generated/api
```

이후 실행에서는 `--update-ref-lock`을 생략하며 remote content가 lock과
일치해야 합니다.

HTTP(S) 루트 입력은 same-origin 상대 참조를 사용할 수 있습니다. URL/stdin
루트에서 remote `$ref`를 가져오려면 기본 lock 경로를 만들 로컬 파일 이름이
없으므로 `--ref-lock`을 지정해야 합니다. 다른 origin은 추가로
`--allow-remote-ref`가 필요합니다. 루트 origin credential은 same-origin 요청에만
적용됩니다.

## Schema extension

### `--schema-extension <manifest>`

필수 사용자 정의 JSON Schema vocabulary를 처리할 신뢰된 로컬 compiler를
등록합니다. 여러 매니페스트가 필요하면 옵션을 반복합니다.

Schema extension은 required custom JSON Schema vocabulary를 처리하고,
[OpenAPI x-* 확장](./extensions.md)은 SDK 편의 기능을 설정합니다.

Schema-extension 매니페스트는 version, vocabulary URI, 실행 파일과 인자,
실행 파일 SHA-256을 선언합니다. Extension digest는 remote reference와 같은
integrity lock을 사용하므로 최초로 신뢰할 때 `--update-ref-lock`이 필요합니다.

로컬 OpenAPI 파일은 기본 lock 경로를 자동으로 만들 수 있습니다. URL이나 stdin
루트 입력에서 schema extension을 사용하려면 `--ref-lock`을 지정해야
합니다.

실행 파일은 generation 중에 custom vocabulary를 일반 JSON Schema로 낮춥니다.
신뢰된 로컬 코드로서 generation process와 같은 권한으로 실행되며, generated
source에는 변환된 schema 의미가 반영됩니다.

매니페스트 형식, trust model, 최초/이후 실행 흐름은
[사용자 정의 JSON Schema vocabulary](../guide/schema-vocabularies.md)를
참고하세요.
