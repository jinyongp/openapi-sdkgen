---
layout: home

hero:
  name: openapi-sdkgen
  text: OpenAPI에서 Go SDK 생성
  tagline: OpenAPI 3.0, 3.1, 3.2 문서로 애플리케이션에서 사용할 SDK 소스를 생성합니다. 생성한 Go 코드를 애플리케이션 소스로 관리합니다.
  actions:
    - theme: brand
      text: 시작하기
      link: /ko/golang/guide/getting-started
    - theme: alt
      text: 플레이그라운드
      link: /ko/golang/playground
    - theme: alt
      text: 호환성 검증 결과
      link: /ko/golang/reference/compatibility

features:
  - title: OpenAPI 3.0·3.1·3.2 지원
    details: 세 버전의 OpenAPI 문서에서 Go SDK를 생성합니다.
  - title: 요청·응답과 스트리밍까지
    details: JSON과 XML, 파일 전송부터 SSE와 NDJSON 스트리밍까지 처리합니다.
  - title: 타입 검사와 실제 데이터 검증
    details: 호출 코드의 타입을 검사하고, 주고받는 데이터도 API 문서에 맞춰 검증합니다.
  - title: Link·웹훅·콜백 연동
    details: 응답에 이어지는 API 호출과 웹훅·콜백 수신에 필요한 코드를 생성합니다.
---

## 문서에서 사용 흐름 확인하기

명세로 SDK를 생성하고 애플리케이션에서 불러오는 방법은 [시작하기](./guide/getting-started.md)에 있습니다.
실행할 코드는 [예제 모음](./examples/index.md)에서 내려받으세요.

- [클라이언트 호출과 오류 처리](./guide/client.md)
- [명세 변경 후 재생성](./guide/generate.md)
- [인증과 전송](./guide/transport.md)
- [CLI·타입·옵션 레퍼런스](./reference/index.md)

<span id="openapi에서-go-sdk-생성하기"></span>
