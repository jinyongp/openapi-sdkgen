# 예제

여기에는 하나의 OpenAPI 문서나 한 번의 generated call보다 큰 integration
경계를 다루는 예제를 모읍니다. 작은 OpenAPI 문서를 바로 수정하면서 생성 결과를
확인하려면 [플레이그라운드](../playground.md)를 사용하세요.

## AI streaming API

[Generated client로 AI streaming API 사용](./ai-streaming.md)은 AI service와 SDK
consumer를 별도 코드베이스로 나눕니다.

- server application이 AI SDK, model provider, HTTP endpoint, OpenAPI 문서를
  소유합니다.
- openapi-sdkgen은 공개된 contract에서 client를 생성합니다.
- client application은 generated SDK와 application-specific stream adapter를
  소유합니다.

API 구현과 SDK consumer가 서로 다른 repository 또는 deployment unit에 있을 때
이 구조를 그대로 적용할 수 있습니다.

## Webhook receiver

[OpenAPI Webhook 수신](./webhook-server.md)은 반대 방향의 integration을
보여줍니다. OpenAPI 3.1이 inbound Webhook을 설명하고, [`--with server`](../reference/cli.md#with-server)가 typed
Fetch router를 생성하며, host application이 route mount와 인증을 소유합니다.
