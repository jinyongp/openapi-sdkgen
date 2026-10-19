import { createClient } from "./generated/api/index.js";

interface TextDelta {
  readonly type: "text-delta";
  readonly text: string;
}

const api: ReturnType<typeof createClient> = createClient({
  baseURL: "https://api.example.test",
  fetch: async (): Promise<Response> =>
    new Response(
      'data: {"type":"text-delta","text":"Hello"}\n\n' +
        'data: {"type":"text-delta","text":"world"}\n\n',
      { headers: { "content-type": "text/event-stream" } },
    ),
});

const stream: ReturnType<typeof api.$operations.generate.stream> =
  api.$operations.generate.stream({
    body: {
      prompt: "Summarize the release notes.",
    },
  });

for await (const event of stream) {
  const payload: TextDelta = JSON.parse(event.data) as TextDelta;
  console.log(payload.text);
}
