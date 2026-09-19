---
layout: home

hero:
  name: openapi-sdkgen
  text: OpenAPI에서 SDK 소스 생성
  tagline: OpenAPI 3.0, 3.1, 3.2 문서를 읽고 선택한 target에 맞는 애플리케이션 SDK 소스를 생성합니다. 현재 버전은 TypeScript target을 제공합니다.
  actions:
    - theme: brand
      text: 시작하기
      link: /ko/guide/getting-started
    - theme: alt
      text: 플레이그라운드
      link: /ko/playground

features:
  - icon: ◇
    title: OpenAPI 3.x 입력
    details: 문서에 선언된 버전에 맞춰 OpenAPI 3.0, 3.1, 3.2 기능을 해석합니다.
  - icon: ↗
    title: 명시적인 target 선택
    details: 출력 target을 명시적으로 선택합니다. 선택한 target이 생성 결과의 언어를 결정합니다.
  - icon: 🧩
    title: 애플리케이션이 소유하는 결과물
    details: 생성된 SDK 소스를 애플리케이션이 관리하는 output 디렉터리에 저장합니다.
  - icon: ✓
    title: 생성과 검증
    details: 선택한 target이 사용된 기능을 안전하게 표현할 수 없으면 생성을 중단하고, --check로 파일을 바꾸지 않고 생성 가능 여부를 확인합니다.
---

## 현재 target: TypeScript

현재 버전은 TypeScript target을 제공합니다. SDK를 생성할 때 target을
명시적으로 선택합니다.

```sh
openapi-sdkgen generate \
  --input ./openapi.yaml \
  --target typescript \
  --output ./src/generated/api
```

현재 TypeScript target은 client, 생성 타입, source runtime을 출력
디렉터리에 생성합니다. 생성된 client는 일반 TypeScript처럼 import해
사용할 수 있습니다.

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
있습니다. 생성과 CI 검증은 [SDK 생성과 검증](./guide/generate.md), 선택한
target의 지원 범위는 [OpenAPI 지원 범위](./reference/capabilities.md)를
참고하세요.

현재 버전에서 제공하는 API와 연동 예제는
[레퍼런스](./reference/index.md)와 [예제](./examples/index.md)에 정리되어
있습니다.
