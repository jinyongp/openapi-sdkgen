# Troubleshooting



## Generation fails

Read the diagnostic code, location and hint. Use `--diagnostic-mode collect` to see more issues.
Check `--go-import-path`, schema/media support and [remote reference settings](./remote-inputs.md).
For output conflicts, see [regeneration](./generate.md).

## Imports or builds fail

Match `--go-import-path` to your `go.mod` module name and output directory.
`--check` verifies generation; run `go test ./...` to check the Go build separately.
[Download the Todo example](/downloads/golang-todo-app.zip) for a complete module setup.

## A request or stream fails

Inspect errors from the generated `types` package with `errors.As`:

```go
var failure *types.APIError
if errors.As(err, &failure) {
	fmt.Println(failure.Code)
}
```

For a call canceled through its context, this prints `REQUEST_ABORTED`. If `err` is `nil` or does not contain an SDK `APIError`, the branch prints nothing.

Check [authentication and transport](./transport.md), and close streams when stopping early.
See [OpenAPI support](../reference/capabilities.md) for XML mapping limits.
