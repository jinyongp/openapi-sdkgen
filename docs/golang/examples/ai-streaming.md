# Stream AI responses


Read and process SSE events one at a time. [Download the complete example](/downloads/golang-ai-streaming-app.zip)
for the contract, Go module, local server and runnable consumer. No model or API key is needed.

## Read the stream {#consume}

Generate an SDK from the [Generation API contract](/examples/golang-ai-stream.json) and configure a client.
This snippet runs inside a function; supply a `ctx context.Context`:

```go
events := client.Operations.Generate(api.GenerateInput{
	Body: api.GenerateBody{Prompt: "Summarize the release notes."},
}).Stream(ctx)
defer events.Close()

for events.Next() {
	fmt.Println(events.Item().Data)
}
if err := events.Err(); err != nil {
	return err
}
```

This contract returns a `Data` string for each SSE event. Keep `defer events.Close()`
for early exits and check `Err()` after the loop.

To connect an AI service, change the client's server address. Model and provider
configuration stays on the server. See [Streaming API](../reference/streaming.md)
for cancellation, frame bounds and JSON event adaptation.
