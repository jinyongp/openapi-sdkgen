# Go compatibility

The [support contract](./capabilities.md) describes implemented Go behavior. Start with the
[local examples](../examples/index.md) to verify your module, transport and schema features.

## Provider measurements

<GoCompatibilityResults />

Generation and compilation do not establish that a provider's server accepts every request
or that every generated call was executed.

The measurement workflow separately generates Go clients and server handlers, then runs
`go test ./...` on the emitted module. Results record generation blocks and compiler failures
as failures. Microsoft Graph generation-only measurements are excluded from this Go profile.
These checks do not call a provider API.

DigitalOcean publishes authoring sources with references in Operation Objects. These require
bundling before generation. The measurement keeps both the original and prepared input hashes,
and checks operation identities, required parameters and response representations after compilation.
It does not repair invalid schemas or establish full schema equivalence.

## Verify your own document

```sh
openapi-sdkgen generate --input ./openapi.yaml --target golang \
  --go-import-path example.com/consumer/internal/api --output ./internal/api --check
go test ./...
```

Generate first if the output directory does not exist. Add `--with server` for webhooks/callbacks.
Exercise actual calls and media against your mock or provider sandbox. Record the input version,
selected operations, addons and Go version. For cost comparisons see [selection and build cost](../guide/selection-cost.md).
