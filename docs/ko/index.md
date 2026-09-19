---
layout: home

hero:
  name: openapi-sdkgen
  text: OpenAPI에서 TypeScript SDK 소스 생성
  tagline: API 계약은 OpenAPI에 두고, 애플리케이션이 소유하는 client source를 생성해 기존 toolchain에서 바로 사용하고 CI에서 검증하세요.
  actions:
    - theme: brand
      text: 첫 SDK 만들기
      link: /ko/guide/getting-started
    - theme: alt
      text: SDK 생성과 검증
      link: /ko/guide/generate

features:
  - icon: 🧩
    title: 애플리케이션이 소유하는 소스
    details: TypeScript를 프로젝트 안에 생성하고 이미 사용하는 compiler와 bundler로 함께 빌드합니다.
  - icon: ✓
    title: 계약 검증
    details: 요청 입력과 decoded response를 검증하고, --check로 생성 가능 여부와 drift를 파일 변경 없이 확인합니다.
  - icon: ⚡
    title: 타입이 지정된 호출 방식
    details: 같은 OpenAPI 계약에서 Todo 같은 resource method, HTTP method/path route, operationId API를 생성합니다.
  - icon: ↗
    title: 선택적인 inbound 계약
    details: OpenAPI가 inbound Webhook이나 Callback을 정의하면 Fetch 기반 handler를 추가로 생성합니다.
---

## OpenAPI에서 Todo 호출까지

SDK를 애플리케이션 소스 안에 생성합니다.

```sh
openapi-sdkgen generate \
  --input ./openapi.yaml \
  --target typescript \
  --output ./src/generated/api
```

생성된 코드는 일반 TypeScript처럼 import해서 사용합니다.

```ts
import { createClient } from "./generated/api";

const api = createClient({
  baseURL: "https://api.example.test/v1",
});

const todo = await api.todos.create({
  body: { title: "문서 작성" },
});
```

생성 디렉터리에는 애플리케이션이 필요한 client, type, source runtime이 함께
들어갑니다.

완전한 최소 Todo 계약부터 따라가려면 [첫 SDK 만들기](./guide/getting-started.md)를
보세요. 이미 프로젝트에 생성을 연결했다면 [SDK 생성과 검증](./guide/generate.md)에서
증분 갱신, `--check`, 인증이 필요한 입력, 원격 참조를 확인할 수 있습니다.

전체 integration 경계를 다루는 내용은 [예제](./examples/index.md)에서 확인할 수
있습니다. 정확한 generated API와 호환성 정보는 [레퍼런스](./reference/index.md),
[스트리밍 API](./reference/streaming.md), [생성된 서버 API](./reference/server-api.md)를
사용하세요.
