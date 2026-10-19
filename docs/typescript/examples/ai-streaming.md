# Stream AI responses

<span id="_1-server-publish-the-streaming-contract"></span>
<span id="_2-server-connect-your-ai-implementation"></span>
<span id="_2-server-use-the-ai-sdk-behind-the-api"></span>
<span id="_3-consumer-generate-the-client"></span>
<span id="_4-consumer-consume-typed-sse-items"></span>
<span id="ai-streaming-api-with-a-generated-client"></span>
<span id="default-sse-mapping"></span>

Read an OpenAPI-described SSE response through the generated client.
[Download the complete example](/downloads/typescript-ai-streaming-app.zip) for the
contract, client, build configuration and local run instructions. No model or API key is needed.

## Read the stream {#consume}

Generate an SDK from the example's `openapi.yaml`, then set `baseURL` to your service:

```ts
import { createClient } from "./generated/api/index.js";

const api = createClient({ baseURL: "https://api.example.test" });
const stream = api.$operations.generate.stream({
  body: { prompt: "Summarize the release notes." },
});

for await (const event of stream) {
  const payload = JSON.parse(event.data);
  console.log(payload.text);
}
```

The downloaded example supplies a fixed local SSE response and prints:

```text
Hello
world
```

Your service determines the event values and how many items arrive.

SSE `data` is a string. This contract puts JSON text inside it, so parse that value
before reading `text`. `itemSchema` validates the Event object; `contentSchema`
validates the JSON inside `data`.

## Connect an AI service {#service}

Model and provider configuration belongs on the server. The client uses only its
HTTP contract. The download's `integration/server.ts` shows how to publish AI SDK
output as SSE. Supply model configuration and an HTTP host to use that server code.

## Cancellation and event conversion {#mapping}

Use request `signal` or `timeoutMS`, or call `stream.abort()`, to stop receiving.
For shared JSON conversion or event routing, use a
[stream adapter](../reference/streaming.md#streamadapter).
See [Streaming API](../reference/streaming.md) for framing and lifetime details.
