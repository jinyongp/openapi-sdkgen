<span id="generated-client로-ai-streaming-api-사용"></span>

# 생성된 클라이언트로 AI 스트리밍 API 사용

이 예제는 AI 서비스를 구현하는 서버와 생성된 SDK를 사용하는 애플리케이션을
서로 다른 코드베이스로 나눕니다.

```text
ai-service/
  openapi.yaml
  src/model.ts
  src/server.ts

sdk-consumer/
  src/generated/api/
  src/client.ts
```

**서버**가 AI SDK와 모델 제공자를 소유합니다. OpenAPI 3.2로 설명된 일반
HTTP/SSE API를 공개합니다. **사용 애플리케이션**은 그 명세에서 클라이언트를 생성하며
AI SDK 패키지에 의존하지 않습니다.

| 코드베이스 | 소유 범위 | 주요 의존성 |
| --- | --- | --- |
| `ai-service` | 모델·제공자 설정, HTTP 엔드포인트, OpenAPI 명세 | `ai`, 모델 제공자 패키지, 서버 프레임워크·실행 환경 |
| `sdk-consumer` | 생성된 SDK, 애플리케이션 동작 | openapi-sdkgen 출력물. AI SDK와 모델 제공자 의존성 없음 |

<span id="_1-server-streaming-contract-공개"></span>

## 1. 서버: 스트리밍 명세 공개

서버 저장소가 OpenAPI 문서를 소유합니다.

```yaml
openapi: 3.2.0
info:
  title: Generation API
  version: 1.0.0

paths:
  /generate:
    post:
      operationId: generate
      requestBody:
        required: true
        content:
          application/json:
            schema:
              type: object
              required: [prompt]
              properties:
                prompt:
                  type: string
      responses:
        "200":
          description: Generation events
          content:
            text/event-stream:
              itemSchema:
                type: object
                required: [data]
                properties:
                  data:
                    type: string
                    contentMediaType: application/json
                    contentSchema:
                      type: object
                      required: [type, text]
                      properties:
                        type:
                          const: text-delta
                        text:
                          type: string
```

`itemSchema`는 표준 SSE 이벤트 객체를 설명합니다. 문자열 `data`에 들어 있는
새 텍스트 조각을 나타내는 `text-delta` 데이터는 클라이언트가 이벤트를 반환하기 전에
`contentSchema`로 검증합니다.

<span id="_2-server-api-내부에서-ai-sdk-사용"></span>

## 2. 서버: API 내부에서 AI SDK 사용

서버는 AI SDK를 내부 구현에 사용할 수 있으며 SDK 사용 애플리케이션에 해당 의존성을
노출하지 않습니다. `src/model.ts`는 애플리케이션이 관리하는 모델 제공자
설정입니다.

```ts
// ai-service/src/server.ts
import { streamText } from "ai";
import { model } from "./model";

export async function handleGenerate(request: Request): Promise<Response> {
  const { prompt } = await request.json() as { prompt: string };
  const result = streamText({ model, prompt });
  const encoder = new TextEncoder();

  const body = new ReadableStream<Uint8Array>({
    async start(controller) {
      for await (const text of result.textStream) {
        const event = { type: "text-delta", text };
        controller.enqueue(
          encoder.encode(`data: ${JSON.stringify(event)}\n\n`),
        );
      }
      controller.close();
    },
  });

  return new Response(body, {
    headers: {
      "content-type": "text/event-stream",
      "cache-control": "no-cache",
    },
  });
}
```

AI SDK의 `streamText()`는 새로 생성된 텍스트 조각을 `textStream`으로 제공합니다.
비동기 반복 가능한 객체와 웹 스트림으로 사용할 수 있습니다. 이 예제는 텍스트
이벤트만 공개합니다. 도구 호출, 출처, 추론 같은 이벤트도 필요하면 서버에서
추가 이벤트 형식을 정의하고 `itemSchema`에 선언하세요. 모델 제공자의 전송 형식과
별개로 애플리케이션에서 사용할 공개 이벤트 형식을 정할 수 있습니다.

서버에서 사용하는 스트리밍 API는 AI SDK
[`streamText` 레퍼런스](https://ai-sdk.dev/docs/reference/ai-sdk-core/stream-text)를
참고하세요.

<span id="_3-consumer-client-생성"></span>

## 3. 사용 애플리케이션: 클라이언트 생성

사용 애플리케이션 저장소는 OpenAPI 명세와 openapi-sdkgen만 필요합니다.

```sh
pnpm exec openapi-sdkgen generate \
  --input https://api.example.test/openapi.yaml \
  --target typescript \
  --output ./src/generated/api
```

생성된 클라이언트는 `generate` API의 각 이벤트에 응답 타입을 적용합니다.
`data`를 문자열로 유지하면서 선언한 내부 JSON 구조를 검증합니다.

<span id="_4-consumer-typed-sse-item-사용"></span>

## 4. 사용 애플리케이션: 타입이 지정된 SSE 항목 사용

```ts
// sdk-consumer/src/client.ts
import { createClient } from "./generated/api";

const api = createClient({
  baseURL: "https://api.example.test",
});

const stream = api.$operations.generate.stream({
  body: {
    prompt: "릴리스 노트를 요약해줘.",
  },
});

for await (const event of stream) {
  const payload = JSON.parse(event.data) as { type: "text-delta"; text: string };
  process.stdout.write(payload.text);
}
```

사용 애플리케이션은 `ai`나 모델 제공자 패키지를 가져오지 않습니다. 생성된
SDK 명세와 타입이 지정된 이벤트를 사용하는 애플리케이션 코드만 필요합니다.

<span id="sse-기본-mapping"></span>

## SSE 기본 변환

기본 SSE 프로토콜은 표준 `data`, `event`, `id`, `retry` 필드를 반환합니다.
`itemSchema`는 이벤트 객체를 검증합니다. 이 예제의 `contentSchema`는 `data`의
내부 JSON도 검증하며 공개 `data` 값은 문자열로 유지됩니다.

JSON 데이터 직접 반환, 이름에 따른 이벤트 분기, 종료 표시, 프레임 묶기처럼
애플리케이션별 변환이 필요하면 `StreamAdapter<ServerSentEvent, Item>`를 사용합니다.
SDK는 어댑터 결과에 애플리케이션 스키마를 적용합니다. 스트리밍 레퍼런스에서
명시적 JSON 호환 어댑터 예제를 확인할 수 있습니다.

서버가 AI 모델 제공자를 변경해도 생성된 클라이언트는 제공자별 의존성을
가질 필요가 없습니다.

프로토콜/어댑터 계약은 [스트리밍 API](../reference/streaming.md), 버전별 기능은
[OpenAPI 지원 범위](../reference/capabilities.md)를 참고하세요.
