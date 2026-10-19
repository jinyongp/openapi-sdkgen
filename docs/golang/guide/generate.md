# Generate and verify

The generator emits Go source into your application. Start with [First Go SDK](./getting-started.md)
for module setup and a working request.

## Fresh output

```sh
openapi-sdkgen generate --input ./openapi.json --target golang \
  --go-import-path example.com/consumer/internal/api --output ./internal/api
```

The output includes operation models, clients, shared HTTP/schema source and the requested addons.
Your module owns `go.mod` and `go.sum`. Generated source uses only the standard library.

## Regenerate an existing SDK

```sh
openapi-sdkgen generate --input ./openapi.json --target golang \
  --go-import-path example.com/consumer/internal/api --output ./internal/api --incremental
```

<!--@include: ../../_shared/en/managed-output.md -->

## Check before compiling

```sh
openapi-sdkgen generate --input ./openapi.json --target golang \
  --go-import-path example.com/consumer/internal/api --output ./internal/api --check
go test ./...
```

`--check` writes no source. With an output directory it detects drift in generated paths, content,
fingerprints and the manifest. Without output it only verifies that generation succeeds.
`--check` and `--incremental` are mutually exclusive. A successful check exits zero; failures exit nonzero.
It does not compile Go or contact your API.

## Configuration

```toml
source = "./openapi.json"
target = "golang"
output = "./internal/api"

[golang]
import_path = "example.com/consumer/internal/api"
package = "api"
```

```sh
openapi-sdkgen generate --config ./openapi-sdkgen.toml --incremental
```

`--go-package` overrides the root package name. Use `--with server` for webhook/callback source and
`--with metadata` for runtime metadata. CLI flags override their configuration fields.
See [CLI](../reference/cli.md), [remote inputs](./remote-inputs.md) and [selection](./selective-client.md).
