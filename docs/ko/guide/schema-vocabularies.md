# 사용자 정의 JSON Schema vocabulary

OpenAPI 3.1 또는 3.2 스키마가 필수 사용자 정의 JSON Schema vocabulary를
선언하면 `--schema-extension`으로 해당 vocabulary compiler를 등록합니다.

Schema extension은 사용자 정의 JSON Schema vocabulary를 일반 JSON Schema로
변환합니다. OpenAPI `x-*` 필드는 pagination, visibility 같은 SDK 편의 기능을
설정합니다.

## 언제 필요한가

스키마가 사용자 정의 vocabulary를 필수로 선언할 수 있습니다.

```yaml
components:
  schemas:
    TodoTitle:
      $vocabulary:
        https://schemas.example.test/todo-v1: true
      x-todo-title-policy: concise
      type: string
```

필수 custom vocabulary에는 등록된 extension이 필요하며, 의미를 해석할 수 없으면
생성이 중단됩니다.

Schema extension은 SDK 생성 중에 실행되는 신뢰된 로컬 코드입니다. 사용자 정의
스키마를 일반 JSON Schema object 또는 boolean schema로 변환하고, 생성된
애플리케이션 소스에는 변환된 스키마 의미가 반영됩니다.

## 매니페스트 작성

버전 1 JSON 매니페스트에 처리할 vocabulary와 실행 파일을 등록합니다.

```json
{
  "version": 1,
  "extensions": [
    {
      "vocabularies": ["https://schemas.example.test/todo-v1"],
      "command": "./bin/todo-schema-extension",
      "args": [],
      "sha256": "<sha256-of-the-executable>"
    }
  ]
}
```

`command`는 매니페스트 기준 상대 경로를 사용할 수 있습니다. 해석된 경로는 실행
가능한 일반 파일이어야 합니다. `sha256`은 실행 파일 내용과 일치해야 하며,
프로그램이 바뀌면 새 digest를 다시 신뢰해야 합니다.

실행 파일은 버전이 있는 JSON-RPC schema-extension 프로토콜을 사용합니다. 자신이
처리하는 vocabulary를 선언하고, 해당 스키마를 TypeScript 생성기가 이해할 수
있는 표준 JSON Schema로 낮춥니다.

## extension digest 기록

로컬 OpenAPI 파일을 사용하면 integrity lock의 기본 경로는
`<input>.openapi-sdkgen.lock`입니다. 최초 성공 실행에서 extension digest를
잠금 파일에 기록합니다.

```sh
openapi-sdkgen generate \
  --input ./openapi.yaml \
  --target typescript \
  --schema-extension ./todo-schema-extension.json \
  --update-ref-lock \
  --output ./src/generated/api
```

이후 실행은 기존 integrity lock을 사용합니다.

```sh
openapi-sdkgen generate \
  --input ./openapi.yaml \
  --target typescript \
  --schema-extension ./todo-schema-extension.json \
  --output ./src/generated/api \
  --incremental
```

실행 파일을 변경하면 매니페스트의 digest를 갱신하고 `--update-ref-lock`으로 새
digest를 integrity lock에 기록합니다.

루트 OpenAPI 문서를 HTTP(S) URL이나 stdin에서 읽으면 잠금 파일 이름을 유도할
로컬 입력 파일이 없습니다. 이 경우 `--ref-lock`을 지정합니다.

```sh
openapi-sdkgen generate \
  --input https://api.example.test/openapi.yaml \
  --ref-lock ./openapi-sdkgen.lock \
  --schema-extension ./todo-schema-extension.json \
  --update-ref-lock \
  --target typescript \
  --output ./src/generated/api
```

## 보안 경계

생성을 수행하는 머신에서 신뢰할 수 있는 프로그램만 등록하세요. 매니페스트와
integrity lock은 실행 파일 identity를 고정하며, extension은 생성 프로세스와 같은
권한으로 실행됩니다.

Generation diagnostics에는 schema-extension 프로토콜이 정의한 결과만 반영됩니다.
생성된 SDK에는 변환된 스키마 의미가 포함됩니다.

`x-pagination`, `x-sdk-visibility` 같은 일반 SDK `x-*` 기능은
[OpenAPI x-* 확장](../reference/extensions.md)을 참고하세요. CLI 옵션은
[CLI 레퍼런스](../reference/cli.md)에서 확인할 수 있습니다.
