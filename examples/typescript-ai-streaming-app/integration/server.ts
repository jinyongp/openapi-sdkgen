import { streamText } from "ai";
import { model } from "./model.js";

interface TextDelta {
  readonly type: "text-delta";
  readonly text: string;
}

export async function handleGenerate(request: Request): Promise<Response> {
  let input: unknown;
  try {
    input = await request.json();
  } catch {
    return new Response("Invalid JSON", { status: 400 });
  }
  if (
    typeof input !== "object" ||
    input === null ||
    !("prompt" in input) ||
    typeof input.prompt !== "string"
  ) {
    return new Response("Expected a prompt string", { status: 400 });
  }
  const result: ReturnType<typeof streamText> = streamText({
    model,
    prompt: input.prompt,
    abortSignal: request.signal,
    onError: ({ error }): void => {
      console.error(error);
    },
  });
  const encoder: TextEncoder = new TextEncoder();

  const body: ReadableStream<Uint8Array> = new ReadableStream<Uint8Array>({
    async start(controller): Promise<void> {
      try {
        for await (const text of result.textStream) {
          const event: TextDelta = { type: "text-delta", text };
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
