# 예제

외부 서비스와 애플리케이션을 연결하는 예제를 모았습니다.
작은 OpenAPI 문서를 바로 수정하면서 생성 결과를
확인하려면 [플레이그라운드](../playground.md)를 사용하세요.

<span id="ai-streaming-api"></span>

## AI 스트리밍 API

[생성된 클라이언트로 AI 스트리밍 API 사용](./ai-streaming.md)은 AI 서비스와 SDK
사용 애플리케이션을 별도 코드베이스로 나눕니다.

- 서버 애플리케이션이 AI SDK, 모델 제공자, HTTP 엔드포인트, OpenAPI 문서를
  소유합니다.
- openapi-sdkgen은 공개된 명세에서 클라이언트를 생성합니다.
- 클라이언트 애플리케이션은 생성된 SDK와 애플리케이션에 맞춘 스트림 어댑터를
  소유합니다.

API 구현과 SDK 사용 애플리케이션이 서로 다른 저장소 또는 배포 단위에 있을 때
이 구조를 그대로 적용할 수 있습니다.

<span id="webhook-receiver"></span>

## 웹훅 수신

[OpenAPI 웹훅 수신](./webhook-server.md)은 반대 방향의 연동을
보여줍니다. OpenAPI 3.1이 수신 웹훅을 설명하고, [`--with server`](../reference/cli.md#with-server)가 타입이 지정된
Fetch 라우터를 생성하며, 호스트 애플리케이션이 경로 연결과 인증을 담당합니다.
