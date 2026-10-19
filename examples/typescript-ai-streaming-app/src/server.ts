import { once } from "node:events";
import {
  createServer,
  type IncomingMessage,
  type ServerResponse,
} from "node:http";
import { createOpenAI } from "@ai-sdk/openai";
import { streamText } from "ai";

const apiKey: string | undefined = process.env.OPENAI_API_KEY;
const modelID: string | undefined = process.env.OPENAI_MODEL;
if (!apiKey || !modelID) {
  throw new Error(
    "Set OPENAI_API_KEY and OPENAI_MODEL before starting the server.",
  );
}
const provider: ReturnType<typeof createOpenAI> = createOpenAI({
  apiKey,
  baseURL: process.env.OPENAI_BASE_URL,
});
const model: ReturnType<typeof provider.chat> = provider.chat(modelID);

async function generate(
  request: IncomingMessage,
  response: ServerResponse,
): Promise<void> {
  if (request.method !== "POST" || request.url !== "/generate") {
    response.writeHead(404).end("Not found");
    return;
  }
  const abort: AbortController = new AbortController();
  response.once("close", (): void => abort.abort());
  const timer: ReturnType<typeof setTimeout> = setTimeout(
    (): void => abort.abort(),
    120_000,
  );
  try {
    const chunks: Buffer[] = [];
    let size: number = 0;
    for await (const chunk of request) {
      const buffer: Buffer = Buffer.from(chunk);
      size += buffer.length;
      if (size > 64 * 1024) {
        response.writeHead(413).end("Prompt is too large");
        return;
      }
      chunks.push(buffer);
    }
    let input: unknown;
    try {
      input = JSON.parse(Buffer.concat(chunks).toString("utf8"));
    } catch {
      response.writeHead(400).end("Expected a JSON prompt");
      return;
    }
    if (
      typeof input !== "object" ||
      input === null ||
      !("prompt" in input) ||
      typeof input.prompt !== "string" ||
      !input.prompt.trim()
    ) {
      response.writeHead(400).end("Expected a non-empty prompt string");
      return;
    }
    const result: ReturnType<typeof streamText> = streamText({
      model,
      prompt: input.prompt,
      abortSignal: abort.signal,
      maxRetries: 0,
      onError: (): void => {
        if (!abort.signal.aborted) console.error("AI provider stream failed");
      },
    });
    for await (const part of result.fullStream) {
      if (part.type === "error") throw part.error;
      if (part.type !== "text-delta") continue;
      if (!response.headersSent) {
        response.writeHead(200, {
          "content-type": "text/event-stream",
          "cache-control": "no-cache",
        });
      }
      const event: string = JSON.stringify({
        type: "text-delta",
        text: part.text,
      });
      if (!response.write(`data: ${event}\n\n`)) {
        await once(response, "drain", { signal: abort.signal });
      }
    }
    if (!response.headersSent) throw new Error("The model returned no text");
    response.end();
  } catch {
    if (!response.headersSent)
      response.writeHead(502).end("AI generation failed");
    else response.destroy();
  } finally {
    clearTimeout(timer);
    abort.abort();
  }
}

const server: ReturnType<typeof createServer> = createServer(
  (request: IncomingMessage, response: ServerResponse): void => {
    void generate(request, response);
  },
);
server.listen(
  Number(process.env.AI_API_PORT ?? 18793),
  "127.0.0.1",
  (): void => {
    console.log("AI API listening");
  },
);
