// A test-only provider boundary. Downloaded examples always use real provider SDKs.
import assert from "node:assert/strict";
import { createServer } from "node:http";

let cancellations = 0;
let requests = 0;
const server = createServer(async (request, response) => {
  if (request.url === "/stats") {
    response.setHeader("content-type", "application/json");
    response.end(JSON.stringify({ cancellations, requests }));
    return;
  }
  try {
    assert.equal(request.method, "POST");
    assert.equal(request.url, "/v1/chat/completions");
    assert.equal(request.headers.authorization, "Bearer example-test-key");
    let body = "";
    for await (const chunk of request) body += chunk;
    const input = JSON.parse(body);
    assert.equal(input.model, "example-test-model");
    assert.equal(input.stream, true);
    const prompt = input.messages.find(
      (message) => message.role === "user",
    )?.content;
    assert.equal(typeof prompt, "string");
    requests++;
    if (prompt === "upstream-error") {
      response.writeHead(401, { "content-type": "application/json" });
      response.end(
        JSON.stringify({
          error: { message: "Invalid test key", type: "authentication_error" },
        }),
      );
      return;
    }
    assert.ok(
      [
        "Explain what an OpenAPI SDK does in one sentence.",
        "upstream-wait",
        "upstream-disconnect",
      ].includes(prompt),
    );
    response.writeHead(200, { "content-type": "text/event-stream" });
    const write = (text) =>
      response.write(
        `data: ${JSON.stringify({
          id: "chatcmpl-example",
          object: "chat.completion.chunk",
          created: 1,
          model: input.model,
          choices: [
            { index: 0, delta: { content: text }, finish_reason: null },
          ],
        })}\n\n`,
      );
    write("An OpenAPI SDK ");
    if (prompt === "upstream-wait") {
      response.once("close", () => {
        cancellations++;
      });
      return;
    }
    setTimeout(() => {
      if (prompt === "upstream-disconnect") {
        response.destroy();
        return;
      }
      write("turns your API contract into typed calls.");
      response.end("data: [DONE]\n\n");
    }, 50);
  } catch (error) {
    console.error(error);
    response
      .writeHead(500)
      .end("Provider request did not match the test contract");
  }
});
server.listen(Number(process.env.AI_PROVIDER_PORT), "127.0.0.1", () =>
  console.log("AI provider fixture listening"),
);
