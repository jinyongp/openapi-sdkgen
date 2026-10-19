# First Go SDK

<span id="generate-and-use-a-go-sdk"></span>
<span id="generate-into-your-module"></span>
<span id="install-the-generated-dependencies"></span>
<span id="raw-responses-and-links"></span>
<span id="security-and-transport"></span>
<span id="selection-and-compilation-cost"></span>
<span id="streams-files-and-custom-codecs"></span>
<span id="troubleshooting"></span>
<span id="typed-calls-and-presence"></span>
<span id="webhooks-and-callbacks"></span>

Generate Go source from OpenAPI and import it into your module. Building the
SDK requires Go 1.27+. [Download the Go Todo example](/downloads/golang-todo-app.zip)
for a complete runnable application.

## Install the CLI {#install}

```sh
brew install jinyongp/tap/openapi-sdkgen
```

For npm, use `npm install -g openapi-sdkgen`. You can also download a
[native executable](https://github.com/jinyongp/openapi-sdkgen/releases).

## Generate the SDK {#generate}

In your Go module, save the [Todo contract](/examples/golang-todo.json) as `openapi.json`.
The contract includes a webhook, so enable `--with server`.
This command assumes the module is named `example.com/consumer`:

```sh
openapi-sdkgen generate \
  --input ./openapi.json \
  --target golang \
  --go-import-path example.com/consumer/internal/api \
  --output ./internal/api \
  --with server \
  --incremental
```

Match `--go-import-path` to your module and output directory. Run the same command
to update the SDK when the contract changes.

## Call the API {#call}

Import the generated package as `api "example.com/consumer/internal/api"`.
Inside a function, send a request as follows. Supply a `ctx context.Context` and
set `baseURL` to your API server's address.

```go
client, err := api.NewClient(api.ClientOptions{BaseURL: "http://localhost:8080"})
if err != nil {
	return err
}

todo, err := client.Operations.CreateTodo(api.CreateTodoInput{
	Body: api.CreateTodoBody{Title: "Ship the SDK"},
}).Do(ctx)
if err != nil {
	return err
}
fmt.Printf("Created %s: %s\n", todo.ID, todo.Title)
```

Read the created item's `ID` and `Title` from `todo`. To run without preparing an
API server, use the example ZIP above. It includes the local server, Go module,
generation command and run instructions.

Continue with [calls and errors](./client.md), [models](./models.md), and [regeneration](./generate.md).
