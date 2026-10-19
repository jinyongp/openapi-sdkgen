# Go compatibility

The [support contract](./capabilities.md) describes implemented Go behavior. Start with the
[local examples](../examples/index.md) to verify your module, transport and schema features.

## Provider measurements

This page has no published GitHub Actions provider measurement for Go. It does not reuse another
SDK's generation/typechecking tables as Go evidence. A generation pass alone also does not establish
that a provider's server accepts every request or that every generated call was executed.

## Verify your own document

```sh
openapi-sdkgen generate --input ./openapi.yaml --target golang \
  --go-import-path example.com/consumer/internal/api --output ./internal/api --check
go test ./...
```

Generate first if the output directory does not exist. Add `--with server` for webhooks/callbacks.
Exercise actual calls and media against your mock or provider sandbox. Record the input version,
selected operations, addons and Go version. For cost comparisons see [selection and build cost](../guide/selection-cost.md).
