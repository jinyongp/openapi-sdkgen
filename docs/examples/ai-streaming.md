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

| Codebase | Owns | Key dependencies |
| --- | --- | --- |
| `ai-service` | model/provider configuration, HTTP endpoint, OpenAPI contract | `ai`, model-provider package, server framework/runtime |
| `sdk-consumer` | generated SDK, application behavior | openapi-sdkgen output; no AI SDK or model-provider dependency |

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
                type: object
                required: [type, text]
                properties:
                  type:
                    const: text-delta
                  text:
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

The AI SDK's `streamText()` API exposes `textStream` as an async iterable/Web
stream of text deltas. This example intentionally publishes one stable public
event shape. If the API also needs tool calls, sources, reasoning, or other event
kinds, the server can map the AI SDK's richer stream into additional OpenAPI
`itemSchema` variants instead of leaking provider-specific wire events through
the public contract.

See the AI SDK
[`streamText` reference](https://ai-sdk.dev/docs/reference/ai-sdk-core/stream-text)
for the AI-side streaming API used by the server.

## 3. Consumer: generate the client

The consumer repository only needs the OpenAPI contract and openapi-sdkgen:

```sh
pnpm exec openapi-sdkgen generate \
  --input https://api.example.test/openapi.yaml \
  --target typescript \
  --output ./src/generated/api
```

The generated client knows that `generate` has an incremental response item
type. For built-in SSE, openapi-sdkgen also handles the common JSON-in-`data`
mapping automatically, so the consumer does not need an adapter for this shape.

## 4. Consumer: consume typed SSE items

```ts
// sdk-consumer/src/client.ts
import { createClient } from "./generated/api";

const api = createClient({
  baseURL: "https://api.example.test",
});

const stream = api.$operations.generate.stream({
  body: {
    prompt: "Summarize the release notes.",
  },
});

for await (const event of stream) {
  process.stdout.write(event.text);
}
```

The consumer does not import `ai` or a model provider package. Its dependencies
are the generated SDK contract and whatever application code consumes the typed
events.

## Default SSE mapping

The built-in SSE protocol still parses standard `data`, `event`, `id`, and
`retry` fields. When no custom stream adapter or protocol is configured,
openapi-sdkgen parses each SSE `data` value as JSON and validates/projects the
result through `itemSchema`.

Use `StreamAdapter<ServerSentEvent, Item>` only when the application needs
different semantics, such as non-JSON data, named-event routing, a terminal
marker, frame aggregation, or another application-specific mapping. A custom
adapter receives the raw `ServerSentEvent` frames and replaces the default JSON
mapping.

This keeps the common JSON SSE path configuration-free while preserving an
explicit extension point for specialized protocols. The server remains free to
change AI providers, and the generated client stays free of provider-specific
dependencies.

See [Streaming API](../reference/streaming.md) for the protocol/adapter contract
and [OpenAPI support](../reference/capabilities.md) for version-specific
capabilities.
