# OpenAPI 지원 범위

openapi-sdkgen은 OpenAPI 3.0.x, 3.1.x, 3.2.x 문서를 읽고 문서에 선언된 버전에
맞춰 기능을 해석합니다. 생성은 fail-closed 방식이며, 선택한 TypeScript target이
사용된 기능을 안전하게 표현할 수 없으면 해당 OpenAPI 위치를 알려 주고
중단합니다.

이 페이지는 주요 공개 capability를 요약합니다. 실제 생성 흐름과 flag는
[CLI 레퍼런스](./cli.md)를 참고하세요.

<span id="supported-openapi-versions"></span>

## 지원 OpenAPI 버전

| OpenAPI | SDK 생성 | Sequential media |
| --- | --- | --- |
| 3.0.x | 지원 | 알려진 sequential content type에 일반 `schema`를 선언하면 complete buffered value로 처리할 수 있습니다. |
| 3.1.x | 지원 | 3.0.x와 같은 complete-value 처리를 지원하며 3.1 JSON Schema 모델을 사용합니다. |
| 3.2.x | 지원 | Media Type Object의 `itemSchema`, `prefixEncoding`, `itemEncoding`을 사용해 typed incremental stream과 positional/streaming multipart를 표현할 수 있습니다. |

따라서 생성기 전체가 OpenAPI 3.2에만 한정되는 것은 아닙니다. OpenAPI 3.2
`itemSchema`가 typed incremental streaming을 추가하고, 3.0/3.1은 complete
`schema` value에 built-in sequential framing을 사용할 수 있습니다. 정확한
streaming 계약은 [스트리밍 API](./streaming.md#openapi-version-support)를
참고하세요.

3.2 전용 필드는
[OpenAPI Media Type Object](https://spec.openapis.org/oas/v3.2.0.html#media-type-object)에
정의되어 있습니다.

## Client 요청과 응답

TypeScript target은 다음 내용을 타입과 실행 가능한 client 동작으로 생성합니다.

- path, HTTP method, path/query/header/cookie parameter, request body
- JSON, text, binary, form, multipart, 지원되는 streaming media
- status별 response, response header, raw response 접근
- 적용되는 OpenAPI/JSON Schema 계약에 따른 request 및 decoded response validation

Operation은 generated resource method, `"METHOD /path"` route, `operationId` 중
필요한 방식으로 호출할 수 있습니다.
[생성된 클라이언트 사용](../guide/client.md)에서 실제 호출 흐름을 설명합니다.

## Server와 security

OpenAPI Server Object와 server variable을 지원하며 operation/path/root 범위의
우선순위를 반영합니다. 호출자가 지정한
[`baseURL`](./client-api.md#clientoptions)은 server selection을 override합니다.

OpenAPI API key, HTTP Basic/Bearer, OAuth2, OpenID Connect, mutual TLS security
scheme을 지원합니다. 여러 Security Requirement Object가 대안으로 적용되면
생성된 요청 옵션이 사용할 수 있는
[`securityRequirement`](./client-api.md#security-requirement) 값을 TypeScript union으로
노출합니다.

Credential 획득은 애플리케이션이 담당합니다.
[인증, 전송, 스트림](../guide/transport.md)을 참고하세요.

## Link, pagination, stream

OpenAPI Link Object는 [`$links`](./client-api.md#link) 아래의 타입 안전 후속 호출
helper로 생성됩니다. `x-pagination`을 선언하면 pagination helper가 생성됩니다.
[OpenAPI x-* 확장](./extensions.md#x-pagination)을 참고하세요.

Sequential media는 generated operation-centric stream surface를 사용합니다.
`.stream()`, request source, built-in protocol, adapter, frame limit, lifecycle은
[스트리밍 API](./streaming.md)에 정리되어 있습니다.

## Webhook과 Callback

Webhook과 Callback Object는 inbound request를 설명합니다. 기본 target은 outbound
client artifact를 만들고, [`--with server`](./cli.md#typescript-server-add-on)가
inbound contract를 추가합니다. Add-on은 Fetch 기반 handler/router API를
만듭니다. HTTP listener, framework 연결, 공개 route, 인증 정책은
애플리케이션이 담당합니다.

Lookup 정보는 [생성된 서버 API](./server-api.md), 실제 연결 흐름은
[Webhook과 Callback 수신](../guide/server.md)을 참고하세요.

## JSON Schema vocabulary

생성기는 지원하는 OpenAPI 버전의 표준 JSON Schema vocabulary를 처리합니다.
Required custom vocabulary에는 추가 schema semantics가 필요하므로 신뢰된
compile-time schema extension을 등록합니다. Extension은 생성 중에
custom vocabulary를 표준 JSON Schema로 낮추고, generated runtime은 변환된 schema
의미를 사용합니다.

[사용자 정의 JSON Schema vocabulary](../guide/schema-vocabularies.md)를
참고하세요.

## SDK 전용 OpenAPI 확장

[`x-pagination`](./extensions.md#x-pagination),
[`x-envelope`](./extensions.md#x-envelope),
[`x-sdk-visibility`](./extensions.md#x-sdk-visibility),
[`x-sort`](./extensions.md#x-sort),
[`x-error-category`](./extensions.md#x-error-category) 같은 지원 `x-*` 필드는
선택적인 SDK 편의 기능을 추가합니다.
Custom JSON Schema vocabulary extension은 schema 의미를 처리합니다.

[OpenAPI x-* 확장](./extensions.md)을 참고하세요.

## Feature coverage

프로젝트는 지원 OpenAPI 버전에 대한 실행 가능한 feature evidence를 유지합니다.
문서 사이트는 지원 기능을 사용하는 방법에 집중하며, generation diagnostic이
특정 문서와 설치된 버전의 생성 가능 여부를 판정합니다.

Compatibility 진단은 scope-aware입니다. `effect: omit-operation` 또는
`effect: omit-capability` warning은 해당 surface 전체를 의도적으로 생성하지
않으면서 나머지 안전한 SDK는 계속 생성할 수 있다는 뜻입니다. `effect: block`
진단은 선택한 target의 emit을 막습니다.

예를 들어 path template과 path Parameter 이름이 일치하지 않으면
`COMP-PARAM-005`로 보고하고 잘못된 operation만 생략합니다. OAS 3.0/3.1에서
operation이 명시한 Security Requirement가 선언되지 않은 scheme을 참조하면
`COMP-SEC-001` operation omission이 되지만, 같은 문제가 root security에 있으면
document block입니다. sdkgen은 복구를 위해 path parameter 이름을 추측해 바꾸거나
security scheme을 만들어내지 않습니다. OAS 3.2의 Security Scheme URI 이름은 같은
이름의 component가 없다는 이유만으로 거부하지 않습니다.

## Compatibility 근거

Feature manifest는 기능별 정식 계약입니다. 프로젝트는 여기에 더해 실제 문서 기반
근거를 두 종류로 유지합니다.

- compatibility 작업 중 regression probe로 사용한 7개 provider corpus
- sdkgen 결과를 보기 전에 고정한 별도 20-provider APIs.guru holdout

현재 교정된 holdout에서 기본 TypeScript client profile은 20개 중 17개 문서를
생성하고 strict typecheck까지 통과합니다. Webhook 또는 Callback을 포함한 문서를
기존 `server` add-on으로 추가 검증하면 capability-adjusted support는 20개 중
19개입니다. 같은 holdout에서 2,830개 operation 중 2,829개가 유지되고, 명시적인
compatibility finding 38개 중 37개는 의미를 보존하는 처리로 끝납니다.

이 수치는 전체 OpenAPI 생태계에 대한 지원률 약속이 아니라 engineering evidence입니다.
Holdout은 인기도가 아니라 문서 크기로 층화되어 있고, OAS 3.0 문서 19개와 OAS 3.1
문서 1개로 구성됩니다. 현재 OAS 3.2 문서와 선택된 external-`$ref` 사례는 없습니다.
Capability-adjusted 기준으로 남은 실패는 generic nested Schema `$ref` 지원 공백입니다.

표본 선택 규칙, 고정 identity, 정확한 지표, 알려진 공백, 재현 명령은 저장소의
`docs/openapi-compatibility-architecture.md`와
`test/compatibility/README.md`를 참고하세요.

버전이 있는 JSON 계약은
[CLI diagnostics 레퍼런스](./cli.md#diagnostics)를 참고하세요.

SDK 생성에 사용한 원본 OpenAPI 문서는 generated metadata 진입점에서도 확인할 수
있습니다.
