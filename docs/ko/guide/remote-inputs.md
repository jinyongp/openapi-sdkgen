# 원격 명세와 참조 읽기

로컬 파일만 사용한다면 [SDK 생성과 검증](./generate.md)의 명령으로 충분합니다.
이 가이드는 URL이나 표준 입력을 읽거나, 인증이 필요한 명세와 원격 `$ref`를
사용할 때 필요한 설정을 설명합니다.

입력 명세를 받는 인증과 생성된 SDK가 API를 호출할 때의 인증은 별개입니다.
SDK 호출 인증은 [인증·전송·스트림](./transport.md)을 참고하세요.
아래 URL은 예시이므로 실제 명세와 참조 주소로 바꾸세요.

## 입력 소스 선택

[`--input`](../reference/cli.md#input-source-options)은 로컬 JSON/YAML 파일, `file://` URL, HTTP(S) URL, 표준 입력을
뜻하는 `-`를 받을 수 있습니다.

```sh
# Local file
openapi-sdkgen generate --input ./openapi.yaml --target typescript --check

# Development server
openapi-sdkgen generate \
  --input http://localhost:4010/openapi.json \
  --target typescript \
  --check

# Standard input
curl https://api.example.test/openapi.yaml | \
  openapi-sdkgen generate \
    --input - \
    --input-base https://api.example.test/openapi.yaml \
    --target typescript \
    --check
```

[`--input-base`](../reference/cli.md#input-source-options)는 표준 입력에서 읽은 문서의 상대 참조를 해석할 기준 위치를
지정합니다. 파일과 URL 입력은 각 입력 위치를 기준 위치로 사용합니다.

## 인증이 필요한 OpenAPI URL 읽기

입력 문서의 인증 정보는 환경 변수로 전달합니다. [`--http-header-env`](../reference/cli.md#authenticated-http-input)는 요청
헤더를 환경 변수 이름에 연결하고 생성기가 그 값을 내부에서 읽으므로,
비밀 값은 환경 변수에 유지됩니다.

```sh
export OPENAPI_TOKEN='Bearer example-token'

openapi-sdkgen generate \
  --input https://api.internal.example/openapi.yaml \
  --http-header-env Authorization=OPENAPI_TOKEN \
  --target typescript \
  --check
```

`Authorization=OPENAPI_TOKEN`은 `OPENAPI_TOKEN` 환경 변수의 값을 읽으라는
뜻입니다. `$OPENAPI_TOKEN`과 `${OPENAPI_TOKEN}`은 셸 확장 문법이고,
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

환경 변수에는 인증 방식을 포함한 완성된 헤더 값을 넣습니다. 헤더와 환경 변수의
연결은 여러 번 지정할 수 있으며 `https://` 루트 입력에서만 허용됩니다.
`http://` 입력은 요청을 보내기 전에 오류로 거부합니다. `Host`, `Cookie`,
`Proxy-Authorization` 같은 전송 계층이 관리하는 헤더도 CLI가 거부합니다.

mTLS 또는 사설 CA가 필요하면 [`--tls-client-cert`, `--tls-client-key`, `--tls-ca-file`](../reference/cli.md#authenticated-http-input)을 사용합니다. 클라이언트 인증서와 키는 함께 지정해야 합니다.
루트 OpenAPI URL은 사용자가 지정한 정확한 출처(프로토콜, 호스트, 포트) 안에서만
신뢰됩니다. 리디렉션도 그 출처를 벗어날 수 없으므로 다른 출처로의 리디렉션에
의존하지 말고 최종 URL을 직접 사용합니다. 인증 정보와 헤더
설정도 같은 경계에 묶이며 같은 출처의 요청에만 적용됩니다.

## 원격 `$ref`를 재현 가능하게 사용

루트 OpenAPI URL과 다른 출처의 `$ref`는 각각 별도의 입력 정책을 사용합니다.
다른 출처의 원격 참조는 허용할 HTTPS 출처를 명시합니다.

최초 실행에서는 [`--allow-remote-ref`](../reference/cli.md#remote-ref-options)로 정확한 HTTPS 출처를 허용하고 [`--update-ref-lock`](../reference/cli.md#remote-ref-options)으로 무결성 잠금 파일을 갱신합니다.

```sh
openapi-sdkgen generate \
  --input ./openapi.yaml \
  --target typescript \
  --allow-remote-ref https://schemas.example.test \
  --update-ref-lock \
  --output ./src/generated/api
```

로컬 루트 파일의 기본 잠금 파일 경로는 `<input>.openapi-sdkgen.lock`입니다. 이후
실행에서는 [`--update-ref-lock`](../reference/cli.md#remote-ref-options)을 생략하며, 잠금 파일에 기록된 내용과 실제
참조가 일치해야 생성이 계속됩니다.

[`--offline`](../reference/cli.md#remote-ref-options)은 잠긴 로컬 캐시만 사용해 원격 참조를 해석합니다.
URL/표준 입력 흐름처럼 로컬 입력 파일 이름에서 잠금 파일 경로를 만들 수 없거나 별도
위치를 사용하려면 [`--ref-lock <path>`](../reference/cli.md#remote-ref-options)를 지정합니다.

루트 OpenAPI URL에 설정한 인증 정보는 같은 출처에만 적용됩니다.

<span id="webhook과-callback-수신-코드-생성"></span>
