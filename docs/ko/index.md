---
layout: home

hero:
  name: openapi-sdkgen
  text: OpenAPI에서 SDK 소스 생성
  tagline: OpenAPI 3.0, 3.1, 3.2 문서로 애플리케이션에서 사용할 SDK 소스를 생성합니다. 생성한 TypeScript 코드를 애플리케이션 소스로 관리합니다.
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
  - title: OpenAPI 3.0·3.1·3.2 지원
    details: 세 버전의 OpenAPI 문서에서 TypeScript SDK를 생성합니다.
  - title: 요청·응답과 스트리밍까지
    details: JSON과 XML, 파일 전송부터 SSE와 NDJSON 스트리밍까지 처리합니다.
  - title: 타입 검사와 실제 데이터 검증
    details: 호출 코드의 타입을 검사하고, 주고받는 데이터도 API 문서에 맞춰 검증합니다.
  - title: Link·웹훅·콜백 연동
    details: 응답에 이어지는 API 호출과 웹훅·콜백 수신에 필요한 코드를 생성합니다.
---

## 명세에서 호출까지

명세를 저장하고 SDK를 생성한 뒤 애플리케이션에서 불러옵니다. API 서버는 별도로
준비합니다. [시작하기](./guide/getting-started.md)는 서버 없이 모의 응답으로 첫 호출을
확인할 수 있는 완결된 예제입니다.

| 하려는 작업 | 읽을 문서 |
| --- | --- |
| 처음 생성하고 결과 실행하기 | [시작하기](./guide/getting-started.md) |
| 명세 변경을 반영하고 CI에서 검사하기 | [SDK 생성과 검증](./guide/generate.md) |
| 요청을 보내고 응답·오류 처리하기 | [클라이언트 사용](./guide/client.md) |
| 파일·스트림·웹훅 연동하기 | [사용 예제](./examples/index.md) |
| 특정 옵션과 지원 조건 찾기 | [레퍼런스](./reference/index.md) |
