# Go Todo SDK example

Run typed inputs, `Raw`, a response Link, resource paths, an incremental stream,
static selection, and a webhook against temporary local HTTP servers. No external
API server or Node.js installation is needed.

You need Go 1.27+ and a Go-enabled `openapi-sdkgen` executable on your `PATH`.
From this directory:

```sh
bash ./setup.sh
go run .
```

`setup.sh` generates `generated/api` with the server addon. The generated SDK
uses only the Go standard library. To use another executable, set `SDKGEN_BIN`
to its path.

Expected output:

```text
created: 201
linked: todo-1 Ship the SDK
resource: todo-1
event: Ship the SDK
selected: todo-1
webhook: todo-1
```

The example closes its servers and stream, drains the webhook response, and uses
a five-second context deadline. Start with `main.go` and change `openapi.json`
to try another contract. Regenerate after changing the document.
