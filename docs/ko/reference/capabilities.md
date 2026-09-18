# OpenAPI 지원 범위

openapi-sdkgen은 OpenAPI 3.0.x, 3.1.x, 3.2.x 문서를 읽고 문서에 선언된 버전에
맞춰 기능을 해석합니다. 생성은 fail-closed 방식이며, 선택한 TypeScript target이
사용된 기능을 안전하게 표현할 수 없으면 해당 OpenAPI 위치를 알려 주고
중단합니다.

이 페이지는 주요 공개 capability를 요약합니다. 실제 생성 흐름과 flag는
[CLI 레퍼런스](./cli.md)를 참고하세요.

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
우선순위를 반영합니다. 호출자가 지정한 `baseURL`은 server selection을
override합니다.

OpenAPI API key, HTTP Basic/Bearer, OAuth2, OpenID Connect, mutual TLS security
scheme을 지원합니다. 여러 Security Requirement Object가 대안으로 적용되면
생성된 요청 옵션이 사용할 수 있는 `securityRequirement` 값을 TypeScript union으로
노출합니다.

Credential 획득은 애플리케이션이 담당합니다.
[인증, 전송, 스트림](../guide/transport.md)을 참고하세요.

## Link, pagination, stream

OpenAPI Link Object는 `$links` 아래의 타입 안전 후속 호출 helper로 생성됩니다.
지원되는 streaming response는 `$streams` 아래의 `AsyncIterable`로
노출됩니다.

`x-pagination`을 선언하면 pagination helper가 생성됩니다. 표준 OpenAPI
operation은 일반 호출로 계속 사용할 수 있습니다.
[OpenAPI x-* 확장](./extensions.md)을 참고하세요.

## Webhook과 Callback

Webhook과 Callback Object는 inbound request를 설명합니다. 기본 target은 outbound
client artifact를 만들고, `--with server`가 inbound contract를 추가합니다.
Add-on은 Fetch 기반 handler/router API를 만듭니다. HTTP listener, framework
연결, 공개 route, 인증 정책은 애플리케이션이 담당합니다.

[Webhook과 Callback 수신](../guide/server.md)에서 자세히 설명합니다.

## JSON Schema vocabulary

생성기는 지원하는 OpenAPI 버전의 표준 JSON Schema vocabulary를 처리합니다.
Required custom vocabulary에는 추가 schema semantics가 필요하므로 신뢰된
compile-time schema extension을 등록합니다. Extension은 생성 중에
custom vocabulary를 표준 JSON Schema로 낮추고, generated runtime은 변환된 schema
의미를 사용합니다.

[사용자 정의 JSON Schema vocabulary](../guide/schema-vocabularies.md)를
참고하세요.

## SDK 전용 OpenAPI 확장

`x-pagination`, `x-envelope`, `x-sdk-visibility`, `x-sort`,
`x-error-category` 같은 지원 `x-*` 필드는 선택적인 SDK 편의 기능을 추가합니다.
Custom JSON Schema vocabulary extension은 schema 의미를 처리합니다.

[OpenAPI x-* 확장](./extensions.md)을 참고하세요.

## Feature coverage

프로젝트는 지원 OpenAPI 버전에 대한 실행 가능한 feature evidence를 유지합니다.
문서 사이트는 지원 기능을 사용하는 방법에 집중하며, generation diagnostic이
특정 문서와 설치된 버전의 생성 가능 여부를 판정합니다.

SDK 생성에 사용한 원본 OpenAPI 문서는 generated metadata 진입점에서도 확인할 수
있습니다.
