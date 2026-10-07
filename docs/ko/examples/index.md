# 예제

외부 서비스와 애플리케이션을 연결하는 예제를 모았습니다.
작은 OpenAPI 예제를 불러와 생성 결과를
확인하려면 [플레이그라운드](../playground.md)를 사용하세요.

<span id="ai-streaming-api"></span>

## 작업에 맞는 예제 선택

| 확인할 내용 | 예제와 준비 조건 |
| --- | --- |
| SDK 생성부터 첫 응답까지 | [시작하기](../guide/getting-started.md): Node.js·pnpm만으로 모의 응답까지 확인 |
| 파일 전송과 응답 후속 호출 | [파일, Link, 스트림](../guide/files-links-streams.md): 기능별 명세와 API 서버 필요 |
| AI 응답 순차 수신 | [AI 스트리밍](./ai-streaming.md): 모의 SSE로 확인, 실제 연동에는 모델·서버 구성 필요 |
| 외부 요청 수신 | [웹훅 수신](./webhook-server.md): 로컬 Fetch 요청으로 확인, 배포에는 HTTP 서버 필요 |

두 예제에는 별도 서비스 없이 실행하는 로컬 확인 코드와 기대 결과가 있습니다.
설치와 ESM 설정은 시작하기를 따르고, 실제 연동은 애플리케이션에 맞게 구성하세요.

## AI 스트리밍 API

[AI 스트리밍](./ai-streaming.md)은 스트리밍 명세에서 클라이언트를 생성하고
SSE 이벤트를 읽는 예제입니다. 모의 응답으로 확인한 뒤 실제 AI 서비스에 연결하세요.

<span id="webhook-receiver"></span>

## 웹훅 수신

[OpenAPI 웹훅 수신](./webhook-server.md)은 반대 방향의 연동을
보여줍니다. OpenAPI 3.1이 수신 웹훅을 설명하고, [`--with server`](../reference/cli.md#with-server)가 타입이 지정된
Fetch 라우터를 생성하며, 호스트 애플리케이션이 경로 연결과 인증을 담당합니다.
