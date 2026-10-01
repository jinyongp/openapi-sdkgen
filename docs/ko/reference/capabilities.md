# OpenAPI 지원 범위

openapi-sdkgen은 OpenAPI 3.0.x, 3.1.x, 3.2.x 문서를 읽고, 문서에 선언된 버전에
맞춰 SDK를 생성합니다. 선택한 생성 대상이 사용된 기능을 안전하게 표현할 수
없으면, 문제가 있는 OpenAPI 위치를 알려 주고 생성을 중단합니다.

이 페이지는 지원하는 기능과 사용 조건을 설명합니다. 생성 명령과 옵션은
[CLI 레퍼런스](./cli.md), 실제 문서로 측정한 결과는
[호환성 검증 결과](./compatibility.md)에서 확인할 수 있습니다.

<span id="supported-openapi-versions"></span>

## 지원 OpenAPI 버전

| OpenAPI | SDK 생성 | 연속 데이터 처리 |
| --- | --- | --- |
| 3.0.x | 지원 | 전체 데이터를 모아 하나의 값으로 처리 |
| 3.1.x | 지원 | 전체 값 처리에 3.1 스키마 규칙 적용 |
| 3.2.x | 지원 | 항목별 타입과 순차 처리 지원 |

OpenAPI 3.0과 3.1에서도 지원되는 연속 데이터 형식을 사용할 수 있습니다.
일반 `schema`를 선언하면 전체 데이터를 모아 하나의 값으로 처리합니다.
3.2의 `itemSchema`는 각 항목에 타입을 지정하고, 도착한 항목부터 처리할 수
있도록 확장합니다. `prefixEncoding`과 `itemEncoding`으로 순서가 있는
multipart 데이터와 스트리밍 multipart 데이터도 표현할 수 있습니다. 자세한 동작은
[스트리밍 API](./streaming.md#openapi-version-support)를 참고하세요.

3.2 전용 필드의 정의는
[OpenAPI의 미디어 타입 객체 명세](https://spec.openapis.org/oas/v3.2.0.html#media-type-object)에
있습니다.

## 클라이언트 요청과 응답

TypeScript를 생성 대상으로 선택하면 요청과 응답에 필요한 타입 및 실행 코드를
생성합니다. 경로와 HTTP 메서드, 경로·쿼리·헤더·쿠키 매개변수, 요청 본문을
지원합니다. JSON, 텍스트, 바이너리, 폼, multipart 데이터와 지원되는 스트리밍
형식도 처리합니다.

응답은 상태 코드별로 구분하며, 응답 헤더와 원본 응답에 접근할 수 있습니다.
요청 값과 해석된 응답 값은 해당 문서의 OpenAPI 및 JSON Schema 규칙에 따라
검증합니다.

API 작업은 생성된 메서드, `"METHOD /path"` 형태의 경로, `operationId` 중
필요한 방식으로 호출할 수 있습니다.
[생성된 클라이언트 사용](../guide/client.md)에서 실제 호출 흐름을 설명합니다.

## 접속 주소와 인증

문서에 선언된 서버 주소와 서버 변수를 지원합니다. 서버 주소는 API 작업,
경로, 문서 전체 순으로 우선 적용됩니다. 호출할 때
[`baseURL`](./client-api.md#clientoptions)을 지정하면 그 주소를 사용합니다.

API 키, HTTP Basic·Bearer, OAuth2, OpenID Connect, 상호 TLS 인증을 지원합니다.
여러 인증 요구 사항 중 하나를 선택할 수 있는 경우,
[`securityRequirement`](./client-api.md#security-requirement)에 지정할 수 있는
값을 TypeScript 타입으로 제공합니다.

인증 정보나 토큰을 발급받는 과정은 애플리케이션에서 구현합니다.
[인증, 전송, 스트림](../guide/transport.md)을 참고하세요.

## 후속 호출, 페이지 조회, 스트리밍

OpenAPI의 Link 객체는 [`$links`](./client-api.md#link) 아래에 타입을 검사할 수
있는 후속 호출 함수로 생성됩니다. `x-pagination`을 선언하면 여러 페이지에
걸친 데이터를 조회하는 보조 기능이 생성됩니다.
[OpenAPI x-* 확장](./extensions.md#x-pagination)을 참고하세요.

연속 데이터는 해당 API 작업의 스트리밍 기능으로 처리합니다. `.stream()` 호출,
요청 데이터 공급, 기본 프로토콜, 어댑터, 프레임 크기 제한, 시작과 종료 동작은
[스트리밍 API](./streaming.md)에 정리되어 있습니다.

## 웹훅과 콜백 수신

OpenAPI의 웹훅과 콜백은 애플리케이션이 받는 요청을 설명합니다. 기본 SDK는
외부 API를 호출하는 클라이언트를 생성합니다.
[`--with server`](./cli.md#typescript-server-add-on)를 지정하면 수신 요청을
처리하는 타입과 Fetch 기반 처리 함수·라우터도 생성합니다.

HTTP 서버 실행, 프레임워크 연결, 공개 경로 설정, 인증 정책은 애플리케이션에서
구현합니다. API의 자세한 사용법은 [생성된 서버 API](./server-api.md), 연동
과정은 [웹훅과 콜백 수신](../guide/server.md)을 참고하세요.

## JSON Schema 어휘

지원하는 OpenAPI 버전에 정의된 표준 JSON Schema 어휘를 처리합니다. 문서가
알 수 없는 사용자 정의 어휘를 필수로 요구하면, 그 의미를 해석할 수 있는
신뢰된 스키마 확장을 등록해야 합니다.

확장은 SDK 생성 시 사용자 정의 어휘를 표준 JSON Schema로 변환합니다.
생성된 실행 코드는 변환된 스키마를 사용합니다.
[사용자 정의 JSON Schema 어휘](../guide/schema-vocabularies.md)를 참고하세요.

## SDK 전용 OpenAPI 확장

[`x-pagination`](./extensions.md#x-pagination),
[`x-envelope`](./extensions.md#x-envelope),
[`x-sdk-visibility`](./extensions.md#x-sdk-visibility),
[`x-sort`](./extensions.md#x-sort),
[`x-error-category`](./extensions.md#x-error-category)를 선언하면 SDK에 편의
기능을 추가할 수 있습니다. 사용자 정의 JSON Schema 어휘 확장은 스키마의
의미를 해석하는 용도로 사용합니다.

각 필드의 사용법은 [OpenAPI x-* 확장](./extensions.md)에 있습니다.

## 생성이 중단되거나 일부 기능이 생략되는 경우

특정 문서를 설치된 버전으로 생성할 수 있는지는 생성 명령의 진단 결과로
확인합니다. 경고에 `effect: omit-operation`이 있으면 해당 API 작업을,
`effect: omit-capability`가 있으면 해당 보조 기능을 생략합니다. 나머지 SDK는
계속 생성할 수 있습니다. `effect: block`은 생성을 중단한다는 뜻입니다.

예를 들어 경로에 쓰인 매개변수 이름과 선언된 이름이 다르면 `COMP-PARAM-005`로
알리고 해당 API 작업만 생략합니다. OpenAPI 3.0·3.1에서 API 작업의 인증 요구
사항이 선언되지 않은 인증 방식을 참조하면 `COMP-SEC-001`로 알리고 그 작업을
생략합니다. 같은 문제가 문서 전체의 인증 설정에 있으면 생성을 중단합니다.

생성기는 잘못된 입력을 복구하려고 매개변수 이름을 추측하거나 인증 방식을
만들어 넣지 않습니다. OpenAPI 3.2에서 URI로 지정한 인증 방식은 같은 이름의
구성 요소가 없다는 이유만으로 거부하지 않습니다. 진단 형식은
[CLI 진단 결과](./cli.md#diagnostics)를 참고하세요.

## 실문서 검증 결과

다양한 제공자의 공개 문서로 SDK 생성과 엄격한 TypeScript 타입 검사를
수행합니다. 개발에 사용한 문서와 별도로 선정한 독립 표본, 제공자가 공개한
문서, 특정 기능을 확인하도록 작성한 테스트 문서를 구분해 검증합니다.

각 검증 모음의 측정 결과와 의미, 문서별 결과, 원본 JSON은
[호환성 검증 결과](./compatibility.md)에서 확인할 수 있습니다.
