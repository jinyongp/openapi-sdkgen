# 예제

<span id="ai-streaming-api"></span>
<span id="webhook-receiver"></span>
<span id="작업에-맞는-예제-선택"></span>
<span id="ai-스트리밍-api"></span>
<span id="webhook-receiver"></span>

전체 실행 코드는 ZIP으로 내려받을 수 있습니다. 각 ZIP의 README에 설치와 실행 방법이 있습니다.
문서에서는 해당 기능의 핵심 코드와 사용 조건을 확인하세요.

## Todo API

항목 생성과 목록 조회를 실제 로컬 HTTP 서버에 요청합니다.
[예제 ZIP](/downloads/typescript-todo-app.zip) · [사용 흐름](../guide/getting-started.md)

## 파일을 서버로 가져오기

클라이언트의 NDJSON 파일을 별도 HTTP 서버에 전송하고 처리 결과를 받습니다.
[예제 ZIP](/downloads/typescript-request-streaming-app.zip) · [주요 코드](../guide/files-links-streams.md#send-streaming-request-bodies)

## 파일 변환

JSON 연락처 파일을 업로드하고 서버가 변환한 NDJSON 파일을 저장합니다.
[예제 ZIP](/downloads/typescript-file-conversion-app.zip) · [주요 코드](./file-conversion.md)

## 웹훅 서명과 중복 처리

별도 발신자의 HMAC 서명을 검증하고 같은 전송의 중복 처리를 막습니다.
[예제 ZIP](/downloads/typescript-signed-webhook-app.zip) · [주요 코드](./signed-webhooks.md)

## AI 응답 스트리밍

AI SDK 서버를 통해 모델에 질문하고 생성된 클라이언트로 답변을 읽습니다.
[예제 ZIP](/downloads/typescript-ai-streaming-app.zip) · [핵심 코드](./ai-streaming.md)

## 백그라운드 매출 보고서

작업을 제출하고 진행 상황을 조회한 뒤 집계된 CSV를 저장합니다. 별도 작업은 취소합니다.
[예제 ZIP](/downloads/typescript-report-jobs-app.zip) · [주요 코드](./report-jobs.md)

## 웹훅 수신

생성된 수신 처리기를 연결하고 인증과 입력 검증을 확인합니다.
[예제 ZIP](/downloads/typescript-webhook-app.zip) · [핵심 코드](./webhook-server.md)

## 인증·페이지 조회·파일 전송

토큰, 경로·쿼리·헤더, 페이지 조회, raw 응답과 바이너리 전송을 함께 사용합니다.
[예제 ZIP](/downloads/typescript-advanced-app.zip) · [소스 보기](https://github.com/jinyongp/openapi-sdkgen/tree/main/examples/typescript-advanced-app)

## OpenAPI 기능 종합 예제

여러 HTTP 메서드, 미디어, 웹훅과 콜백을 하나의 애플리케이션에서 사용합니다.
[예제 ZIP](/downloads/typescript-openapi-capabilities-app.zip) · [소스 보기](https://github.com/jinyongp/openapi-sdkgen/tree/main/examples/typescript-openapi-capabilities-app)

설치 없이 작은 명세의 생성 결과만 보려면 [플레이그라운드](../playground.md)를 사용하세요.
