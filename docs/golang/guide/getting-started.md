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

macOS / Linux:

```sh
curl -fsSL https://jinyongp.dev/openapi-sdkgen/install.sh -o install.sh && sh install.sh
```

Windows PowerShell:

```powershell
Invoke-WebRequest https://jinyongp.dev/openapi-sdkgen/install.ps1 -OutFile install.ps1 -ErrorAction Stop
powershell -NoProfile -ExecutionPolicy Bypass -File .\install.ps1
```

Add the printed installation directory to `PATH` if needed, then run
`openapi-sdkgen --version`. The default is `~/.local/bin` on macOS/Linux or
`%LOCALAPPDATA%\Programs\openapi-sdkgen\bin` on Windows.

To choose a released version or directory, use `sh install.sh --version X.Y.Z --install-dir /your/bin`
or `powershell -NoProfile -ExecutionPolicy Bypass -File .\install.ps1 -Version X.Y.Z -InstallDir C:\your\bin`.
Download the installer again to upgrade. To uninstall, remove the installed executable.

Other options: [download an executable](https://github.com/jinyongp/openapi-sdkgen/releases),
`npm install -g openapi-sdkgen` (Node.js 22+), or `brew install jinyongp/tap/openapi-sdkgen`.

## Generate the SDK {#generate}

In your Go module, save the [Todo contract](/examples/golang-todo.json) as `openapi.json`.
The contract includes a webhook, so enable `--with server`.

```sh
openapi-sdkgen generate \
  --input ./openapi.json \
  --target golang \
  --output ./internal/api \
  --with server \
  --incremental
```

The CLI finds the nearest `go.mod` above the output and derives the import path.
For module `example.com/consumer`, it is `example.com/consumer/internal/api`.
Run the same command to update the SDK when the contract changes.

## Call the API {#call}

For module `example.com/consumer`, import the generated package as
`api "example.com/consumer/internal/api"`. Use your own module path as the prefix.
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

With the Todo example server, the output is:

```text
Created todo-1: Ship the SDK
```

Read the created item's `ID` and `Title` from `todo`. To run without preparing an
API server, use the example ZIP above. It includes the local server, Go module,
generation command and run instructions.

Continue with [calls and errors](./client.md), [models](./models.md), and [regeneration](./generate.md).
