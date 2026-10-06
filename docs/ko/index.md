---
layout: home

hero:
  name: openapi-sdkgen
  text: OpenAPI에서 SDK 소스 생성
  tagline: OpenAPI 3.0, 3.1, 3.2 문서로 애플리케이션에서 사용할 SDK 소스를 생성합니다. 현재 버전은 TypeScript 코드 생성을 지원합니다.
  actions:
    - theme: brand
      text: 시작하기
      link: /ko/guide/getting-started
    - theme: alt
      text: 플레이그라운드
      link: /ko/playground
    - theme: alt
      text: 호환성 검증 결과
      link: /ko/reference/compatibility

features:
  - title: OpenAPI 3.0·3.1·3.2 지원
    details: 세 버전의 OpenAPI 문서에서 TypeScript SDK를 생성합니다.
  - title: 요청·응답과 스트리밍까지
    details: JSON과 XML, 파일 전송부터 SSE와 NDJSON 스트리밍까지 처리합니다.
  - title: 타입 검사와 실제 데이터 검증
    details: 호출 코드의 타입을 검사하고, 주고받는 데이터도 API 문서에 맞춰 검증합니다.
  - title: Link·웹훅·콜백 연동
    details: 응답에 이어지는 API 호출과 웹훅·콜백 수신에 필요한 코드를 생성합니다.
---

## TypeScript SDK 생성하기

현재 버전은 TypeScript 코드 생성을 지원합니다. 다음처럼 `--target typescript`를
지정하면 SDK 소스를 만들 수 있습니다.

```sh
openapi-sdkgen generate \
  --input ./openapi.yaml \
  --target typescript \
  --output ./src/generated/api
```

출력 디렉터리에는 클라이언트, 요청·응답 타입, 실행 코드가 생성됩니다.
생성된 클라이언트는 일반 TypeScript 모듈처럼 불러와 사용할 수 있습니다.

```ts
import { createClient } from "./generated/api";

const api = createClient({
  baseURL: "https://api.example.test/v1",
});

const todo = await api.todos.create({
  body: { title: "문서 작성" },
});
```

현재 TypeScript 사용 흐름은 [시작하기](./guide/getting-started.md)에서 확인할 수
있습니다. 생성과 CI 검증은 [SDK 생성과 검증](./guide/generate.md),
기능별 지원 범위는 [OpenAPI 지원 범위](./reference/capabilities.md)를
참고하세요.

현재 버전에서 제공하는 API와 연동 예제는
[레퍼런스](./reference/index.md)와 [예제](./examples/index.md)에 정리되어
있습니다.
