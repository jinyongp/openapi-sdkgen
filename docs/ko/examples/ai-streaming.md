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

| 코드베이스 | 소유 범위 | 주요 dependency |
| --- | --- | --- |
| `ai-service` | model/provider 설정, HTTP endpoint, OpenAPI contract | `ai`, model-provider package, server framework/runtime |
| `sdk-consumer` | generated SDK, application 동작 | openapi-sdkgen 출력물. AI SDK와 model-provider dependency 없음 |

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
                type: object
                required: [type, text]
                properties:
                  type:
                    const: text-delta
                  text:
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

AI SDK의 `streamText()`는 `textStream`을 text delta의 async iterable/Web
stream으로 제공합니다. 이 예제는 안정적인 public event shape 하나만
공개합니다. Public API에 tool call, source, reasoning 같은 event가 더 필요하면
server가 AI SDK의 richer stream을 추가 `itemSchema` variant로 매핑하면 됩니다.
Provider-specific wire event 자체를 public contract로 노출할 필요는 없습니다.

Server에서 사용하는 AI-side streaming API는 AI SDK
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
있다는 사실을 알고 있습니다. Built-in SSE에서는 흔히 사용하는
JSON-in-`data` mapping도 openapi-sdkgen이 기본으로 처리하므로 이 형태를 위해
adapter를 따로 작성할 필요가 없습니다.

## 4. Consumer: typed SSE item 사용

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
  process.stdout.write(event.text);
}
```

Consumer는 `ai`나 model provider package를 import하지 않습니다. Generated
SDK contract와 typed event를 사용하는 application code만 필요합니다.

## SSE 기본 mapping

Built-in SSE protocol은 표준 `data`, `event`, `id`, `retry` field를
그대로 parsing합니다. Custom stream adapter나 protocol을 지정하지 않으면
openapi-sdkgen이 각 SSE `data` 값을 JSON으로 parsing한 뒤 결과를
`itemSchema`로 validation/projection합니다.

Non-JSON data, named event routing, terminal marker, frame aggregation처럼 다른
application semantics가 필요할 때만 `StreamAdapter<ServerSentEvent, Item>`를
사용합니다. Custom adapter에는 raw `ServerSentEvent` frame이 전달되며,
지정한 adapter가 기본 JSON mapping을 대체합니다.

따라서 일반적인 JSON SSE는 별도 설정 없이 사용할 수 있고, 특수한 protocol은
명시적인 extension point로 처리할 수 있습니다. Server가 AI provider를
변경해도 generated client는 provider-specific dependency를 가질 필요가
없습니다.

Protocol/adapter 계약은 [스트리밍 API](../reference/streaming.md), 버전별 기능은
[OpenAPI 지원 범위](../reference/capabilities.md)를 참고하세요.
