# Stream AI responses with a generated Go client

Install Go 1.27+, Bash and the `openapi-sdkgen` CLI. Run from this directory:

```sh
bash ./setup.sh
```

Start the server in one terminal. Set an OpenAI API key and a model ID available
to your account. Model calls may incur provider charges.

```sh
export OPENAI_API_KEY=your-api-key
export OPENAI_MODEL=your-model-id
cd server
go run .
```

In another terminal, from the example root:

```sh
go run . "Explain what an OpenAPI SDK does in one sentence."
```

The answer appears as the model produces text, for example:

```text
An OpenAPI SDK turns your API contract into typed calls.
```

The wording and chunk boundaries depend on the model. Press Ctrl+C to cancel.
The client times out after two minutes; closing it cancels the model request.
Provider failures before the first text return HTTP 502. Failures during streaming
interrupt the response so the client reports an error instead of a complete answer.

`server/main.go` calls Chat Completions through the official
`github.com/openai/openai-go/v3` library and publishes text SSE at `POST /generate`.
The server has its own `go.mod` and `go.sum`. The root module uses only the generated
SDK and the standard library. `setup.sh` generates the client and downloads the
server's dependencies; it does not make a model call.

Set `OPENAI_BASE_URL` on the server for a compatible provider (include its `/v1`
prefix). `AI_API_PORT` defaults to `18794`; set the client's `AI_API_BASE_URL` to
match if you change it. The API key stays on the server. This server listens on
loopback for local use; add application authentication before exposing it to others.
