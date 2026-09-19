# AI streaming API with a generated client

This example keeps the AI implementation and the generated SDK consumer in
separate codebases.

```text
ai-service/
  openapi.yaml
  src/model.ts
  src/server.ts

sdk-consumer/
  src/generated/api/
  src/client.ts
```

The **server** owns the AI SDK and model provider. It publishes a normal HTTP/SSE
API described by OpenAPI 3.2. The **consumer** generates its client from that
contract and does not depend on the AI SDK package.

## 1. Server: publish the streaming contract

The server repository owns the OpenAPI document:

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

`itemSchema` describes the application event after the SSE frame has been
adapted. The wire protocol is still standard `text/event-stream`.

## 2. Server: use the AI SDK behind the API

The server can use the AI SDK internally without exposing that dependency to SDK
consumers. `src/model.ts` is application-owned provider configuration.

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

The AI SDK's `streamText()` API exposes `textStream` as an async iterable/Web
stream of text deltas. If the public API needs tool calls, sources, reasoning, or
other event kinds, the server can map the AI SDK's richer stream into additional
OpenAPI `itemSchema` variants instead of leaking provider-specific wire events
through the public contract.

See the AI SDK
[`streamText` reference](https://ai-sdk.dev/docs/reference/ai-sdk-core/stream-text)
for its current server-side API.

## 3. Consumer: generate the client

The consumer repository only needs the OpenAPI contract and openapi-sdkgen:

```sh
pnpm exec openapi-sdkgen generate \
  --input https://api.example.test/openapi.yaml \
  --target typescript \
  --output ./src/generated/api
```

The generated client now knows that `generate` has an incremental response item
type. The remaining application-specific detail is that each SSE `data` field
contains one JSON event.

## 4. Consumer: adapt SSE frames to generated events

The client repository owns that mapping:

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
    prompt: "Summarize the release notes.",
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

The consumer does not import `ai` or a model provider package. Its dependencies
are the generated SDK contract and whatever application code consumes the typed
events.

## Why the adapter lives on the client side

SSE defines framing fields such as `data`, `event`, `id`, and `retry`.
The public API in this example defines a second semantic layer: JSON application
events inside `data`.

openapi-sdkgen keeps those concerns separate:

1. the built-in SSE protocol parses wire frames into `ServerSentEvent`;
2. `StreamAdapter` converts the frame into the public application event;
3. the generated runtime validates/projects the result through `itemSchema`;
4. application code receives the generated `AIEvent` union.

This keeps the server free to change AI providers and keeps the generated client
free of provider-specific dependencies.

See [Streaming API](../reference/streaming.md) for the protocol/adapter contract
and [OpenAPI support](../reference/capabilities.md) for version-specific
capabilities.
