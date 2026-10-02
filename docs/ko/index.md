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
  - icon: ◇
    title: OpenAPI 3.x 입력
    details: 문서에 선언된 버전에 맞춰 OpenAPI 3.0, 3.1, 3.2 기능을 해석합니다.
  - icon: ↗
    title: 생성 대상 선택
    details: 생성할 코드의 언어와 기능을 결정하는 대상을 명시적으로 선택합니다.
  - icon: 🧩
    title: 애플리케이션이 소유하는 결과물
    details: 생성된 SDK 소스를 애플리케이션이 관리하는 출력 디렉터리에 저장합니다.
  - icon: ✓
    title: 생성과 검증
    details: 사용된 기능을 안전하게 표현할 수 없으면 생성을 중단합니다. <code>--check</code>로 파일을 바꾸지 않고 생성 가능 여부를 확인할 수 있습니다.
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
