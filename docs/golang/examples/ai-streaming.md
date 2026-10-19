# Stream AI responses


Send a question through the generated client and print the model's answer as it
arrives. The server calls OpenAI through `github.com/openai/openai-go/v3` and
publishes text SSE. [Download the complete example](/downloads/golang-ai-streaming-app.zip)
for both Go modules and run instructions. Set an API key and model ID on the server.

## Read the stream {#consume}

Generate an SDK from the [Generation API contract](/examples/golang-ai-stream.json) and configure a client.
This snippet runs inside a function; supply a `ctx context.Context`:

```go
events := client.Operations.Generate(api.GenerateInput{
	Body: api.GenerateBody{Prompt: "Explain what an OpenAPI SDK does in one sentence."},
}).Stream(ctx)
defer events.Close()

for events.Next() {
	fmt.Print(events.Item().Data)
}
if err := events.Err(); err != nil {
	return err
}
fmt.Println()
```

Text appears incrementally. An example answer is:

```text
An OpenAPI SDK turns your API contract into typed calls.
```

The wording and chunk boundaries depend on the model.

This contract returns a `Data` string for each SSE event. Keep `defer events.Close()`
for early exits and check `Err()` after the loop.

## Run the AI server {#service}

Set `OPENAI_API_KEY` and `OPENAI_MODEL`, then run `go run .` from the download's
`server/` directory. In another terminal, run `go run .` from the example root.
The README covers SDK generation, dependency installation and provider URLs.
The server is a separate Go module with the official OpenAI library. The client
module needs no external libraries and receives no provider credentials.

The runnable client handles Ctrl+C and has a two-minute timeout. Disconnecting it
cancels the model request. Upstream failures return HTTP 502 before text starts,
or interrupt the response if it has already started.

The example server listens on loopback. Add application authentication before
exposing it to others. Model calls may incur provider charges.
See [Streaming API](../reference/streaming.md) for frame bounds and event adaptation.
