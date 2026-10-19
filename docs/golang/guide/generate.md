# Generate and verify

The generator emits Go source into your application. Start with [First Go SDK](./getting-started.md)
for module setup and a working request. The commands below use its Todo contract,
which includes a webhook and needs `--with server`. Omit the addon for contracts without inbound operations.

## Fresh output

```sh
openapi-sdkgen generate --input ./openapi.json --target golang \
  --output ./internal/api --with server
```

The output includes operation models, clients, shared HTTP/schema source and the requested addons.
Your module owns `go.mod` and `go.sum`. Generated source uses only the standard library.

The import path combines the nearest output ancestor's `go.mod` module path with
the output directory relative to that module. Nested modules take precedence.
To generate source for a separate module before it exists, or otherwise override
that path, set `--go-import-path` or `[golang].import_path`. The CLI flag takes
precedence over configuration. If no module is found, an explicit path is required.
The CLI does not create or modify `go.mod`.

## Regenerate an existing SDK

```sh
openapi-sdkgen generate --input ./openapi.json --target golang \
  --output ./internal/api --with server --incremental
```

<!--@include: ../../_shared/en/managed-output.md -->

## Check before compiling

```sh
openapi-sdkgen generate --input ./openapi.json --target golang \
  --output ./internal/api --with server --check
go test ./...
```

`--check` writes no source. With an output directory it detects drift in generated paths, content,
fingerprints and the manifest. Without output it only verifies that generation succeeds.
`--check` and `--incremental` are mutually exclusive. A successful check exits zero; failures exit nonzero.
It does not compile Go or contact your API.

A `--check` run without `--output` has no destination for inference; supply
`--go-import-path` or `[golang].import_path` in that case.

## Configuration

```toml
source = "./openapi.json"
target = "golang"
output = "./internal/api"
addons = ["server"]

[golang]
package = "api"
```

```sh
openapi-sdkgen generate --config ./openapi-sdkgen.toml --incremental
```

`--go-package` overrides the root package name. Use `--with server` for webhook/callback source and
`--with metadata` for runtime metadata. CLI flags override their configuration fields.
See [CLI](../reference/cli.md), [remote inputs](./remote-inputs.md) and [selection](./selective-client.md).
