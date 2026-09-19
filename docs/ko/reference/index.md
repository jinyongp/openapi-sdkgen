# 레퍼런스

찾고 싶은 API, 옵션, 타입, OpenAPI 기능이 이미 정해져 있을 때 사용하는
lookup 문서입니다. 작업 흐름은 [사용 가이드](../guide/getting-started.md), 여러
코드베이스가 연결되는 완전한 integration은 [예제](../examples/index.md)에
정리되어 있습니다.

| 찾는 내용 | 레퍼런스 |
| --- | --- |
| CLI command, generation mode, input/auth/ref 옵션 | [CLI](./cli.md) |
| `createClient`, client/request 옵션, 호출 surface, raw 결과, 오류 | [생성된 클라이언트 API](./client-api.md) |
| Webhook/Callback router, handler, inbound 옵션 | [생성된 서버 API](./server-api.md) |
| `.stream()`, `OperationStream`, request stream, SSE, protocol/adapter/codec | [스트리밍 API](./streaming.md) |
| generated request/response/component/helper 타입 | [생성된 TypeScript 타입](./typescript-types.md) |
| OpenAPI 3.0/3.1/3.2 지원 범위와 버전별 동작 | [OpenAPI 지원 범위](./capabilities.md) |
| `x-pagination`, `x-envelope`, `x-sort`, visibility, error category | [OpenAPI x-* 확장](./extensions.md) |

## 공개 API 경계

Generated **client API**는 SDK consumer가 보내는 outbound 호출을 다룹니다.
Generated **server API**는 `--with server`에서 생성되는 inbound Webhook과
Callback을 다룹니다. **Streaming API**는 outbound stream과 inbound sequential
body가 같은 protocol/adapter contract를 사용하므로 하나의 canonical
레퍼런스에서 설명합니다.

OpenAPI 기능 지원 여부는 generated TypeScript API와 분리해 설명합니다.
“OpenAPI 3.1에서도 가능한가?” 같은 질문은 [OpenAPI 지원 범위](./capabilities.md),
TypeScript에서 어떻게 사용하는지는 위 API 레퍼런스에서 확인할 수 있습니다.
