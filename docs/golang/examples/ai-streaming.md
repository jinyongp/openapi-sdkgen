# AI streaming over SSE

The server owns model/provider dependencies and publishes an HTTP contract. A Go consumer needs
only the generated SDK and the Go standard library; it does not import the model provider.
Save the [Generation API document](/examples/golang-ai-stream.json) as `openapi.json`.
Use [module/dependency setup](../guide/getting-started.md), then generate:

```sh
openapi-sdkgen generate --input ./openapi.json --target golang \
  --go-import-path example.com/consumer/internal/api --output ./internal/api
```

This complete `main.go` uses a local SSE mock. Replace its handler with your server-side AI
implementation when connecting a real provider. The declared SSE item has a required `data` string.

```go
package main

import (
    "context"
    "fmt"
    "net/http"
    "net/http/httptest"
    "time"

    api "example.com/consumer/internal/api"
)

func main() {
    if err := run(); err != nil {
        panic(err)
    }
}

func run() error {
    server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        w.Header().Set("Content-Type", "text/event-stream")
        fmt.Fprint(w, "data: hello\n\n")
        w.(http.Flusher).Flush()
    }))
    defer server.Close()
    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()
    client, err := api.NewClient(api.ClientOptions{BaseURL: server.URL})
    if err != nil {
        return err
    }
    events := client.Operations.Generate(api.GenerateInput{
        Body: api.GenerateBody{Prompt: "Say hello"},
    }).Stream(ctx)
    defer events.Close()
    for events.Next() {
        fmt.Println(events.Item().Data)
    }
    if err := events.Err(); err != nil {
        return err
    }
    return nil
}
```

```sh
go run .
```

```text
hello
```

Keep `defer events.Close()` when stopping early and inspect `Err()` after iteration.
Context cancellation interrupts the exchange. `MaxStreamFrameBytes` limits one frame, not the
whole model output. Add `contentMediaType`/`contentSchema` to validate structured SSE data.
See [streaming API](../reference/streaming.md) for custom adapters and input streams.
