# Stream AI responses with a generated TypeScript client

Install the `openapi-sdkgen` CLI, Node.js 24, pnpm 12 and Bash. Run from this directory:

```sh
bash ./setup.sh
```

`openapi.yaml` defines the contract. `setup.sh` generates SDK source and compiles
`src/` with the included TypeScript configuration. Set `SDKGEN_BIN` to use a
specific generator executable.

Start the server in one terminal. Set an OpenAI API key and a model ID available
to your account. Model calls may incur provider charges.

```sh
export OPENAI_API_KEY=your-api-key
export OPENAI_MODEL=your-model-id
pnpm server
```

In another terminal:

```sh
pnpm start "Explain what an OpenAPI SDK does in one sentence."
```

The answer appears as the model produces text, for example:

```text
An OpenAPI SDK turns your API contract into typed calls.
```

The wording and chunk boundaries depend on the model. Press Ctrl+C to cancel.
The client times out after two minutes. Closing it cancels the model request.
Provider failures before the first text return HTTP 502; failures during streaming
interrupt the response instead of completing it.

`src/server.ts` calls `streamText` from `ai` with `@ai-sdk/openai`, then publishes
JSON text-delta SSE events at `POST /generate`. `src/client.ts` uses the generated
SDK to call that endpoint and print text. The key stays on the server. Both files
are compiled by `pnpm build`.

To use a compatible provider, set `OPENAI_BASE_URL` on the server (include its
`/v1` prefix). The server uses Chat Completions. `AI_API_PORT` defaults to `18793`;
set the client's `AI_API_BASE_URL` to match if you change it. This server listens
on loopback for local use; add application authentication before exposing it to others.
