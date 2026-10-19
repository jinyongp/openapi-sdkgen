import { createClient } from "./generated/api/index.js";

interface TextDelta {
  readonly type: "text-delta";
  readonly text: string;
}

const api: ReturnType<typeof createClient> = createClient({
  baseURL: process.env.AI_API_BASE_URL ?? "http://127.0.0.1:18793",
});
const abort: AbortController = new AbortController();
process.once("SIGINT", (): void => abort.abort());

const stream: ReturnType<typeof api.$operations.generate.stream> =
  api.$operations.generate.stream(
    {
      body: {
        prompt:
          process.argv[2] ??
          "Explain what an OpenAPI SDK does in one sentence.",
      },
    },
    {
      signal: abort.signal,
      timeoutMS: 120_000,
    },
  );

for await (const event of stream) {
  const payload: TextDelta = JSON.parse(event.data) as TextDelta;
  process.stdout.write(payload.text);
}
process.stdout.write("\n");
