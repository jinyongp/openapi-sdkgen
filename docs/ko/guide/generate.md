# SDK 생성과 검증

일반 생성은 OpenAPI 문서로 SDK 소스를 만듭니다. `--check`는 문서의 생성
가능 여부와 기존 생성 파일의 최신 상태를 확인합니다. CI, 편집기,
커밋 전 검사에서 사용할 수 있습니다.

아래 예시는 [`openapi-sdkgen`](../reference/cli.md)이 `PATH`에 있다고 가정합니다. 프로젝트 개발
의존성으로 설치했다면 명령 앞에 `pnpm exec`을 붙이세요.

시작하기에서 SDK를 이미 만들었다면 [기존 SDK 다시 생성](#regenerate-existing-sdk)으로
이동하세요. 새 출력 디렉터리를 만들 때만 일반 생성 명령을 사용합니다.

## 새 출력 디렉터리 생성

일반 생성 명령은 새 관리 디렉터리를 만듭니다.

```sh
openapi-sdkgen generate \
  --input ./openapi.yaml \
  --target typescript \
  --output ./src/generated/api
```

TypeScript 생성 대상은 클라이언트, 생성 타입, 실행 코드, OpenAPI 버전 정보를
출력합니다. 생성기가 관리하는 파일은 CLI로 다시 생성하며, 출력 디렉터리는
애플리케이션 저장소에서 일반 소스로 관리합니다.

생성 결과는 전체 작업이 성공한 뒤 한 번에 반영됩니다.

<span id="regenerate-existing-sdk"></span>

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

## 생성 결과 확인

CI, 편집기, 커밋 전 검사에서 OpenAPI 문서의 생성 가능 여부를 확인하려면
[`--check`](../reference/cli.md#fresh-incremental-and-check-modes)를 사용합니다.

[`--output`](../reference/cli.md#core-options)을 생략하면 OpenAPI 문서로 SDK를 생성할 수 있는지 확인합니다.

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

생성 내용이나 경로가 달라졌거나 입력·설정 식별값이 바뀌었거나, 소유 파일이
수정·삭제됐거나, 매니페스트가 잘못됐거나, 새 생성 경로가 사용자 파일과
충돌하면 실패합니다. 한 번의 실행에서는 [`--check`](../reference/cli.md#fresh-incremental-and-check-modes)와 [`--incremental`](../reference/cli.md#fresh-incremental-and-check-modes) 중 하나를
선택합니다.

## 생성 소스 관리 방식 선택

관리 SDK 디렉터리를 OpenAPI 문서와 함께 커밋할 수 있습니다. 생성기 버전을
고정하고 로컬에서는 `--incremental`로 갱신합니다. CI에서는 위의 출력 디렉터리를
지정한 `--check` 명령으로 최신 상태를 확인한 뒤 애플리케이션을 컴파일합니다.
생성 매니페스트도 파일과 함께 보관하세요.

빌드 전에 SDK를 생성하고 출력 디렉터리를 버전 관리에서 제외하는 방법도 있습니다.
OpenAPI 입력과 생성 설정을 커밋하고 생성기 버전을 고정한 뒤, 깨끗한 체크아웃에서도
TypeScript 컴파일 전에 생성 명령을 실행합니다. `--incremental`은 최초 빌드와
반복 빌드에 모두 사용할 수 있습니다. 출력 디렉터리를 생략한 `--check`는 입력의
생성 가능 여부를 검사하며, 아직 생성하지 않은 파일과 비교하지는 않습니다.

<span id="ci와-도구에서-diagnostics-사용"></span>

## CI 결과 확인

검증이 성공하면 종료 코드가 `0`, 생성 실패나 파일 차이가 있으면 `0`이 아닙니다.
`--check`는 TypeScript 컴파일이나 실제 API 호출을 대신하지 않습니다. 검증 후
애플리케이션의 타입 검사와 테스트를 실행하세요.

여러 문제를 한 번에 보려면 `--diagnostic-mode collect`를 사용합니다.
CI 도구에서 진단을 파싱해야 할 때만 `--diagnostics-format json`을 추가하세요.
형식과 오류 보고 조건은 [CLI 진단 레퍼런스](../reference/cli.md#diagnostics)에 있습니다.

## 다른 입력과 선택 기능

| 필요한 작업 | 다음 문서 |
| --- | --- |
| URL·표준 입력, 명세 인증, 원격 참조 잠금 | [원격 명세와 참조](./remote-inputs.md) |
| 웹훅·콜백 수신 코드 추가 | [웹훅과 콜백 수신](./server.md) |
| 원본 OpenAPI 문서 함께 내보내기 | [메타데이터 옵션](../reference/cli.md#metadata-addon) |
| 사용자 정의 JSON Schema 어휘 처리 | [스키마 어휘](./schema-vocabularies.md) |

SDK를 준비했다면 [클라이언트 사용](./client.md)에서 호출과 오류 처리를 이어서 확인하세요.
