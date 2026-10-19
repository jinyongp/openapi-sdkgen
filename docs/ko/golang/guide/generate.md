# 생성과 검증

생성기는 애플리케이션 안에 Go 소스를 만듭니다. 모듈과 의존성 설정은
[첫 Go SDK](./getting-started.md)에서 시작하세요.

## 새 출력 만들기

```sh
openapi-sdkgen generate --input ./openapi.json --target golang \
  --go-import-path example.com/consumer/internal/api --output ./internal/api
```

출력에는 작업별 모델·클라이언트·공통 HTTP/스키마 소스와 요청한 확장이 포함됩니다.
`go.mod`와 `go.sum`은 사용자 모듈이 관리하며 생성기는 의존성을 설치하지 않습니다.

## 기존 SDK 재생성

```sh
openapi-sdkgen generate --input ./openapi.json --target golang \
  --go-import-path example.com/consumer/internal/api --output ./internal/api --incremental
```

<!--@include: ../../_shared/ko/managed-output.md -->

## 컴파일 전 검사

```sh
openapi-sdkgen generate --input ./openapi.json --target golang \
  --go-import-path example.com/consumer/internal/api --output ./internal/api --check
go test ./...
```

`--check`는 소스를 쓰지 않습니다. 출력 디렉터리를 지정하면 경로·내용·생성 조건·매니페스트의
차이를 검사하고, 생략하면 생성 가능 여부만 확인합니다. `--check`와 `--incremental`은 함께
쓸 수 없습니다. 성공하면 0, 실패하면 0이 아닌 종료 코드를 반환합니다. Go 컴파일이나 API 호출은 하지 않습니다.

## 설정 파일

```toml
source = "./openapi.json"
target = "golang"
output = "./internal/api"

[golang]
import_path = "example.com/consumer/internal/api"
package = "api"
```

```sh
openapi-sdkgen generate --config ./openapi-sdkgen.toml --incremental
```

`--go-package`로 루트 패키지 이름을 바꿀 수 있습니다. 웹훅·콜백에는 `--with server`,
런타임 메타데이터에는 `--with metadata`를 사용하세요. CLI 옵션이 대응하는 설정 필드보다 우선합니다.
[CLI](../reference/cli.md), [원격 입력](./remote-inputs.md), [선택 생성](./selective-client.md)에서 자세히 확인하세요.
