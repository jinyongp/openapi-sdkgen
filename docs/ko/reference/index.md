# 레퍼런스

API, 옵션, 타입, OpenAPI 기능을 찾아볼 수 있는 문서입니다. 처음부터 따라
하는 작업 흐름은 [사용 가이드](../guide/getting-started.md), 여러 코드베이스를
연결하는 연동 예시는 [예제](../examples/index.md)에 정리되어 있습니다.

| 찾는 내용 | 레퍼런스 |
| --- | --- |
| 생성 명령과 모드, 입력·인증·참조 옵션 | [CLI](./cli.md) |
| `createClient`, 클라이언트·요청 옵션, 호출 방식, 원본 응답, 오류 | [생성된 클라이언트 API](./client-api.md) |
| 웹훅·콜백 수신 라우터, 처리 함수, 요청 수신 옵션 | [생성된 서버 API](./server-api.md) |
| `.stream()`, `OperationStream`, 요청 스트림, SSE, 프로토콜·어댑터·코덱 | [스트리밍 API](./streaming.md) |
| 생성된 요청·응답·구성 요소·보조 기능의 타입 | [생성된 TypeScript 타입](./typescript-types.md) |
| OpenAPI 3.0/3.1/3.2 지원 범위와 버전별 동작 | [OpenAPI 지원 범위](./capabilities.md) |
| 실문서 호환성 측정 결과, 검증 입력 선정 방법, 재현 근거 | [호환성 검증 결과](./compatibility.md) |
| `x-pagination`, `x-envelope`, `x-sort`, 공개 범위, 오류 분류 | [OpenAPI x-* 확장](./extensions.md) |

## 공개 API 경계

**클라이언트 API**는 SDK를 사용하는 애플리케이션이 외부로 보내는 요청을
다룹니다. **서버 API**는 `--with server`로 생성하며, 웹훅과 콜백을 수신하는
데 사용합니다. 요청을 보내거나 받을 때 연속 데이터를 처리하는 방법은
**스트리밍 API**에서 함께 설명합니다.

OpenAPI 기능 지원 여부와 생성된 TypeScript API의 사용법은 각각 설명합니다.
“OpenAPI 3.1에서도 가능한가?” 같은 질문은 [OpenAPI 지원 범위](./capabilities.md),
TypeScript에서 어떻게 사용하는지는 위 API 레퍼런스에서 확인할 수 있습니다.
