# OpenAPI 지원

Go는 OpenAPI 3.0.x·3.1.x·3.2.x에서 타입이 있는 모델·송신 클라이언트·선택적 수신 처리기를
생성합니다. 지원하지 않는 선언은 명세 위치와 함께 진단합니다. 명세 전체 오류는 생성을
중단하며 작업·호출 경로의 문제는 해당 기능을 생략할 수 있습니다. CI에서 리소스 생략을
실패로 처리하려면 `--fail-on-resource-omission`을 사용하세요.

| 영역 | Go 동작 |
| --- | --- |
| 호출 | Operations·메서드/경로 Routes·리소스 선택자·타입이 있는 입력과 출력 |
| 스키마 | 버전별 의미·부재/null·방향별 투영·열거값·합성·재귀·제약 |
| 미디어 | JSON·텍스트·바이너리·XML·form·multipart·선언된 증분 미디어 |
| 인증 | API key·Basic/Bearer·OAuth2/OIDC·상호 TLS. 자격 증명 획득은 호스트가 담당 |
| 서버 | 작업/경로/루트 우선순위·변수·명시적 BaseURL |
| 응답 | 상태/미디어별 변형·선언된 헤더·Raw·타입이 있는 HTTP 오류 |
| Link/확장 | 타입이 있는 Link·페이지 순회·정렬·노출 범위·envelope·오류 분류 |
| 수신 | `--with server`: 타입이 있는 `net/http` 웹훅과 콜백 |
| 메타데이터 | `--with metadata`: 원래 명세를 담은 독립 데이터 패키지 |

## URL과 정규식

생성된 Go 소스에는 외부 런타임 모듈 의존성이 없습니다. URL 해석과 출처 정규화는 WHATWG URL
의미를 따릅니다. 스키마 패턴은 고정 Unicode 17 자료와 ECMAScript Unicode `u` 의미를 사용하며,
범위별 `i`·`m`·`s` 수정자, 전후방 탐색과 역참조를 지원합니다. 매칭은 실행 한도와 `context.Context`의 취소·기한을 따르고,
한도를 넘으면 스키마 불일치와 구분되는 오류를 반환합니다.
응답 압축은 사용자 HTTP 클라이언트가 담당합니다. [인증과 전송](../guide/transport.md)을 참고하세요.

## 스트림과 OpenAPI 버전 {#openapi-version-support}

OpenAPI 3.2의 `itemSchema`는 타입이 있는 증분 항목을, `prefixEncoding`·`itemEncoding`은
위치별·스트리밍 multipart를 표현합니다. 3.0·3.1도 알려진 순차 미디어에서 일반 `schema`를
전체 버퍼 값으로 사용할 수 있습니다. 프레이밍·스키마 검증·HTTP 전송은 각각 다른 계층입니다.

## 플랫폼과 매핑 경계

애플리케이션이 Go 1.27 이상·모듈·전송·리스너를 제공합니다. Go는 유효한 HTTP 토큰 메서드를
지원하며 브라우저 Fetch 제한을 적용하지 않습니다. 별도 CONNECT 터널 API는 없습니다.
XML은 namespace·attribute·CDATA·wrapped와 타입이 있는 합성 매핑을 지원하지만 임의의
추가 속성·위치별 prefix item 매핑에는 제약이 있습니다. 표현할 수 없으면 JSON이나 명시적
코덱을 사용하세요. 프레임 제한은 총 응답 크기나 디코더 이력을 제한하지 않습니다.
[스트림](./streaming.md)과 [호환성](./compatibility.md)을 확인하세요.
