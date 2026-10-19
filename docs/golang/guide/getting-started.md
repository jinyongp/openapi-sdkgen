# First Go SDK

<span id="generate-and-use-a-go-sdk"></span>
<span id="generate-into-your-module"></span>


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
  github.com/go-quickjs/go-quickjs@v0.27.0
go mod tidy
```

This example directly uses four external modules. There is no separate SDK runtime
module to install, but the generated Go source does have external dependencies.

| Feature | Module used by Go source | TypeScript equivalent |
| --- | --- | --- |
| Brotli response decoding | `andybalholm/brotli` | Fetch response decoding in the host runtime |
| Zstandard response decoding | `klauspost/compress/zstd` | Fetch response decoding where supported by the host runtime |
| URL parsing and normalization | `nlnwa/whatwg-url` | Built-in `URL` |
| JavaScript-compatible regular expression validation | `go-quickjs/go-quickjs/jsregexp` | Built-in `RegExp` |

The current transport imports the compression and URL modules even for a basic generated
client. The regular expression module is included when selected schemas require regular
expression validation or when the `server` addon's validation code needs it.
Go's standard `net/url` and `regexp` packages do not provide the same URL and regular
expression behavior guaranteed here.

`bitset`, `golang.org/x/net`, and `golang.org/x/text` are indirect dependencies of
`whatwg-url`. Go installs them automatically; they do not need separate entries in the
installation command. `go mod tidy` removes modules unused by the generated code.
Generated TypeScript has no separate npm runtime dependencies: the JavaScript host
provides the corresponding capabilities.

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
