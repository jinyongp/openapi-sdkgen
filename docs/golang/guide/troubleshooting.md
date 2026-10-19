# Troubleshooting

A missing import path is a generation error. A missing dependency is resolved in your own
module; generation does not install it. For incremental output, keep the managed manifest
with the directory. Inspect `types.APIError` using `errors.As` and stable error codes; error
text omits private request/response data:

```go
var failure *types.APIError
if errors.As(err, &failure) {
    fmt.Println(failure.Code)
}
```

XML supports the documented namespace/attribute/CDATA/wrapped and typed composition paths.
Arbitrary additional-property and positional prefix-item XML mappings retain the existing
mapping limits; select JSON or an explicit codec when XML cannot express the schema.
Use the [Go Todo example](https://github.com/jinyongp/openapi-sdkgen/tree/main/examples/golang-todo-app)
for the complete import set, local server and executable error/cleanup handling.

## Locate a generation failure

Use `--diagnostic-mode collect` and read the diagnostic code, location and hint. Check
`--go-import-path`, unsupported schema/media mappings and [remote reference permissions](./remote-inputs.md).
`--check` verifies generation and managed output; run `go test ./...` separately.
