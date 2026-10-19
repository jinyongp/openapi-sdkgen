# Stream AI responses

<span id="_1-server-publish-the-streaming-contract"></span>
<span id="_2-server-connect-your-ai-implementation"></span>
<span id="_2-server-use-the-ai-sdk-behind-the-api"></span>
<span id="_3-consumer-generate-the-client"></span>
<span id="_4-consumer-consume-typed-sse-items"></span>
<span id="ai-streaming-api-with-a-generated-client"></span>
<span id="default-sse-mapping"></span>

Send a question through the generated client and print the model's answer as it
arrives. The server uses `ai` and `@ai-sdk/openai` to call OpenAI and publishes
the response as SSE. [Download the complete example](/downloads/typescript-ai-streaming-app.zip)
for the server, client and setup instructions. Set an API key and model ID on the server.

## Read the stream {#consume}

After starting the downloaded example's server:

```ts
import { createClient } from "./generated/api/index.js";

const api = createClient({ baseURL: "http://127.0.0.1:18793" });
const stream = api.$operations.generate.stream({
  body: { prompt: "Explain what an OpenAPI SDK does in one sentence." },
});

for await (const event of stream) {
  const payload = JSON.parse(event.data);
  process.stdout.write(payload.text);
}
process.stdout.write("\n");
```

Text appears incrementally. An example answer is:

```text
An OpenAPI SDK turns your API contract into typed calls.
```

The wording and chunk boundaries depend on the model.

SSE `data` is a string. This contract puts JSON text inside it, so parse that value
before reading `text`. `itemSchema` validates the Event object; `contentSchema`
validates the JSON inside `data`.

## Connect an AI service {#service}

Set `OPENAI_API_KEY` and `OPENAI_MODEL`, then run `pnpm server`; run `pnpm start`
in another terminal. The README covers installation and compatible provider URLs.
The server's `src/server.ts` calls `streamText` with `provider.chat(modelID)` and
publishes each text delta as a JSON SSE event. The client receives that HTTP stream;
it never calls the model provider or receives the API key.

The example listens on loopback. Add application authentication before exposing
it to others. Model calls may incur provider charges.

## Cancellation and event conversion {#mapping}

Use request `signal` or `timeoutMS`, or call `stream.abort()`, to stop receiving.
The runnable client handles Ctrl+C and has a two-minute timeout. The server forwards
disconnection to the model request. Upstream failures return HTTP 502 before text
starts, or interrupt the response if it has already started.
For shared JSON conversion or event routing, use a
[stream adapter](../reference/streaming.md#streamadapter).
See [Streaming API](../reference/streaming.md) for framing and lifetime details.
