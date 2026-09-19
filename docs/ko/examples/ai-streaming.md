# Generated client로 AI streaming API 사용

이 예제는 AI 구현과 generated SDK consumer를 서로 다른 코드베이스로 나눕니다.

```text
ai-service/
  openapi.yaml
  src/model.ts
  src/server.ts

sdk-consumer/
  src/generated/api/
  src/client.ts
```

**server**가 AI SDK와 model provider를 소유합니다. OpenAPI 3.2로 설명된 일반
HTTP/SSE API를 공개합니다. **consumer**는 그 contract에서 client를 생성하며
AI SDK package에 의존하지 않습니다.

## 1. Server: streaming contract 공개

Server repository가 OpenAPI 문서를 소유합니다.

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
                oneOf:
                  - type: object
                    required: [type, text]
                    properties:
                      type:
                        const: text-delta
                      text:
                        type: string
                  - type: object
                    required: [type, message]
                    properties:
                      type:
                        const: error
                      message:
                        type: string
```

`itemSchema`는 SSE frame을 adapter가 변환한 뒤의 application event를
설명합니다. Wire protocol은 표준 `text/event-stream`을 그대로 사용합니다.

## 2. Server: API 내부에서 AI SDK 사용

Server는 AI SDK를 내부 구현에 사용할 수 있으며 SDK consumer에 해당 dependency를
노출하지 않습니다. `src/model.ts`는 애플리케이션이 소유하는 provider
configuration입니다.

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
      try {
        for await (const text of result.textStream) {
          const event = { type: "text-delta", text };
          controller.enqueue(
            encoder.encode(`data: ${JSON.stringify(event)}\n\n`),
          );
        }
      } catch {
        controller.enqueue(
          encoder.encode(
            `data: ${JSON.stringify({
              type: "error",
              message: "Generation failed",
            })}\n\n`,
          ),
        );
      } finally {
        controller.close();
      }
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

AI SDK의 `streamText()`는 `textStream`을 text delta의 async iterable/Web
stream으로 제공합니다. Public API에 tool call, source, reasoning 같은 event가
필요하면 AI SDK의 더 풍부한 stream을 추가 `itemSchema` variant로 매핑하면
됩니다. Provider-specific wire event 자체를 public contract로 노출할 필요는
없습니다.

현재 server-side API는 AI SDK
[`streamText` 레퍼런스](https://ai-sdk.dev/docs/reference/ai-sdk-core/stream-text)를
참고하세요.

## 3. Consumer: client 생성

Consumer repository는 OpenAPI contract와 openapi-sdkgen만 필요합니다.

```sh
pnpm exec openapi-sdkgen generate \
  --input https://api.example.test/openapi.yaml \
  --target typescript \
  --output ./src/generated/api
```

Generated client는 `generate` operation에 incremental response item 타입이
있다는 사실을 알고 있습니다. 남은 application-specific 규칙은 SSE의 각
`data`에 JSON event 하나가 들어 있다는 점입니다.

## 4. Consumer: SSE frame을 generated event로 변환

이 mapping은 client repository가 소유합니다.

```ts
// sdk-consumer/src/client.ts
import {
  createClient,
  type OperationStreamItem,
  type ServerSentEvent,
  type StreamAdapter,
} from "./generated/api";

type AIEvent = OperationStreamItem<"generate">;

const aiEventAdapter: StreamAdapter<ServerSentEvent, AIEvent> = {
  async *decode(events) {
    for await (const event of events) {
      yield JSON.parse(event.data) as AIEvent;
    }
  },

  async *encode(items) {
    for await (const item of items) {
      yield { data: JSON.stringify(item) };
    }
  },
};

const api = createClient({
  baseURL: "https://api.example.test",
  streamCodecs: {
    "text/event-stream": {
      adapter: aiEventAdapter,
    },
  },
});

const stream = api.$operations.generate.stream({
  body: {
    prompt: "릴리스 노트를 요약해줘.",
  },
});

for await (const event of stream) {
  if (event.type === "text-delta") {
    process.stdout.write(event.text);
  } else if (event.type === "error") {
    console.error(event.message);
  }
}
```

Consumer는 `ai`나 model provider package를 import하지 않습니다. Generated
SDK contract와 typed event를 사용하는 application code만 필요합니다.

## Adapter가 client에 있는 이유

SSE는 `data`, `event`, `id`, `retry` 같은 framing field를 정의합니다.
이 예제의 public API는 그 위에 두 번째 semantic layer인 JSON application
event를 정의합니다.

openapi-sdkgen은 이 두 경계를 분리합니다.

1. built-in SSE protocol이 wire frame을 `ServerSentEvent`로 parsing합니다.
2. `StreamAdapter`가 frame을 public application event로 변환합니다.
3. generated runtime이 결과를 `itemSchema`로 validation/projection합니다.
4. application code는 generated `AIEvent` union을 받습니다.

이 구조에서는 server가 AI provider를 변경해도 client contract가 provider
dependency를 가질 필요가 없습니다.

Protocol/adapter 계약은 [스트리밍 API](../reference/streaming.md), 버전별 기능은
[OpenAPI 지원 범위](../reference/capabilities.md)를 참고하세요.
