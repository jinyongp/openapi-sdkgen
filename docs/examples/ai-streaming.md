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

`itemSchema` describes the standard SSE Event object. Its `data` string contains
a text-delta payload, validated by `contentSchema` before the client yields it.

<span id="_2-server-use-the-ai-sdk-behind-the-api"></span>

## 2. Server: connect your AI implementation

The server can use the AI SDK internally without exposing that dependency to SDK
consumers. `src/model.ts` is application-owned provider configuration.

This section is integration code. Install `ai` and your provider package,
export a configured `model` from `src/model.ts`, and route `POST /generate`
to `handleGenerate` in your HTTP host. To test only the consumer, use the
mock response in step 4.

Use an ESM project (`"type": "module"`). Node servers also need `@types/node`;
AI SDK type dependencies may require `@types/json-schema`. Follow your chosen
provider's installation instructions for credentials and model configuration.

```ts
// ai-service/src/server.ts
import { streamText } from "ai";
import { model } from "./model.js";

export async function handleGenerate(request: Request) {
  let input: unknown;
  try {
    input = await request.json();
  } catch {
    return new Response("Invalid JSON", { status: 400 });
  }
  if (typeof input !== "object" || input === null ||
      !("prompt" in input) || typeof input.prompt !== "string") {
    return new Response("Expected a prompt string", { status: 400 });
  }
  const result = streamText({
    model, prompt: input.prompt, abortSignal: request.signal,
    onError: ({ error }) => { console.error(error); },
  });
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
        controller.close();
      } catch (error: unknown) {
        controller.error(error);
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

In a separate consumer project, use the installation and ESM setup from
[Getting started](../guide/getting-started.md) and save step 1 as `openapi.yaml`.
Add `--incremental` to the command when updating an existing generated directory.

```sh
pnpm exec openapi-sdkgen generate \
  --input ./openapi.yaml \
  --target typescript \
  --output ./src/generated/api
```

The generated client knows that `generate` has an incremental Event response.
It preserves `data` as a string and validates the declared embedded JSON shape.

## 4. Consumer: consume typed SSE items

```ts
// sdk-consumer/src/client.ts
import { createClient } from "./generated/api/index.js";

interface TextDelta {
  readonly type: "text-delta";
  readonly text: string;
}

const api = createClient({
  baseURL: "https://api.example.test",
  fetch: async () => new Response(
    'data: {"type":"text-delta","text":"Hello"}\n\n' +
    'data: {"type":"text-delta","text":"world"}\n\n',
    { headers: { "content-type": "text/event-stream" } },
  ),
});

const stream = api.$operations.generate.stream({
  body: {
    prompt: "Summarize the release notes.",
  },
});

for await (const event of stream) {
  const payload = JSON.parse(event.data) as TextDelta;
  console.log(payload.text);
}
```

Run with your application's existing build tool, or use an installed TypeScript
compiler for this standalone check. Compiler installation is optional for SDK generation.

```sh
pnpm exec tsc --strict --target ES2022 --module NodeNext --moduleResolution NodeNext \
  --lib ES2022,DOM,DOM.Iterable --outDir dist src/client.ts
node dist/client.js
```

Expected output is `Hello` and `world` on separate lines. The mock SSE response
checks generation, compilation, and stream consumption without a model call.
For the real service, remove `fetch` and replace `baseURL` with its address.
The server framework and model configuration must be supplied separately.



## Default SSE mapping

The built-in SSE protocol returns standard `data`, `event`, `id`, and `retry`
fields. `itemSchema` validates that Event object. In this example, `contentSchema`
also validates the JSON carried by `data`, which remains a string.

Use `StreamAdapter<ServerSentEvent, Item>` for application-specific transformations
such as direct JSON payload output, named-event routing, terminal markers, or
frame aggregation. Adapter output is validated against the SDK application
schema. The streaming reference includes an explicit JSON compatibility adapter.


See [Streaming API](../reference/streaming.md) for the protocol/adapter contract
and [OpenAPI support](../reference/capabilities.md) for version-specific
capabilities.
