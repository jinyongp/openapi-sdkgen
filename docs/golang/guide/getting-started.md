# First Go SDK

Generate source with the native executable, then compile and run it in your own **Go 1.27+** module.
Install the CLI with Homebrew or npm, or download the native executable from
[Releases](https://github.com/jinyongp/openapi-sdkgen/releases):

```sh
brew install jinyongp/tap/openapi-sdkgen
# Alternative: npm install -g openapi-sdkgen
openapi-sdkgen --version
go mod init example.com/consumer
```

Save the [Todo document](/examples/golang-todo.json) as `openapi.json`.
The document includes a webhook, so this command enables the `server` addon.

```sh
openapi-sdkgen generate \
  --input ./openapi.json \
  --target golang \
  --go-import-path example.com/consumer/internal/api \
  --output ./internal/api \
  --with server \
  --incremental
```

`--go-import-path` must match your module and output directory. The generated root package
is `api` by default. The generator writes no `go.mod`; your application owns its dependencies.
The native executable needs no Go or Node.js to generate source. The npm launcher needs Node.js.

## Install the generated dependencies

```sh
go get github.com/andybalholm/brotli@v1.2.6 \
  github.com/klauspost/compress@v1.20.3 \
  github.com/nlnwa/whatwg-url@v0.6.2 \
  github.com/go-quickjs/go-quickjs@v0.27.0 \
  github.com/bits-and-blooms/bitset@v1.20.0 \
  golang.org/x/net@v0.61.0 golang.org/x/text@v0.43.0
go mod tidy
```

Only dependencies imported by generated features are required. No separate SDK runtime module is needed.

## Run a first request against a local mock

Save this as `main.go`. It owns a local server and closes it after one typed call.

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
        w.Header().Set("Content-Type", "application/json")
        w.WriteHeader(http.StatusCreated)
        fmt.Fprint(w, `{"id":"todo-1","title":"Ship the SDK"}`)
    }))
    defer server.Close()
    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()
    client, err := api.NewClient(api.ClientOptions{BaseURL: server.URL})
    if err != nil {
        return err
    }
    todo, err := client.Operations.CreateTodo(api.CreateTodoInput{
        Body: api.CreateTodoBody{Title: "Ship the SDK"},
    }).Do(ctx)
    if err != nil {
        return err
    }
    fmt.Println(todo.ID, todo.Title)
    return nil
}
```

```sh
go run .
```

```text
todo-1 Ship the SDK
```

Continue with [calls and errors](./client.md), [models](./models.md), and
[regeneration](./generate.md). The [complete Go example](../examples/index.md) includes streams and webhooks.
