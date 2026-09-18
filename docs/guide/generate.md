# Generate and verify an SDK

Generation has two modes. Normal generation updates application source. Check
mode runs the same compiler and target preparation while leaving generated output
unchanged, which fits CI, editor, and pre-commit validation.

Examples below use `openapi-sdkgen` directly. If the CLI is installed as a
project dependency, prefix the command with `pnpm exec`.

## Create a fresh generated directory

The normal command creates a new managed output directory:

```sh
openapi-sdkgen generate \
  --input ./openapi.yaml \
  --target typescript \
  --output ./src/generated/api
```

The TypeScript target writes the client, generated types, source runtime, and
OpenAPI metadata. Fresh generation creates a new output directory.

Generated source belongs to the application repository. Regenerate generator-owned
files through the CLI; managed-output validation detects manual edits. Generation
publishes the output atomically after the full operation succeeds.

## Regenerate an existing SDK

After the first successful generation, use `--incremental` with the same
managed directory:

```sh
openapi-sdkgen generate \
  --input ./openapi.yaml \
  --target typescript \
  --output ./src/generated/api \
  --incremental
```

The manifest in the output directory records the files owned by the generator.
An incremental run:

- keeps unchanged generated files in place;
- atomically replaces changed files;
- removes only stale files previously owned by the manifest;
- preserves files you added outside the manifest;
- stops if an owned generated file was edited, the manifest is invalid, a new
  generated path conflicts with an unmanaged file, or another writer holds the
  output lock.

For a self-contained local input whose generation fingerprint still matches,
an unchanged incremental run can also skip compilation and emission.

## Check generation

Use `--check` when CI, an editor, or a pre-commit task needs to validate whether
the OpenAPI document can be generated.

Omitting `--output` runs input loading, compilation, and target preparation, then
exits after the preflight:

```sh
openapi-sdkgen generate \
  --input ./openapi.yaml \
  --target typescript \
  --check
```

If generated source is checked into the repository, add the existing managed
output directory. The command regenerates the expected artifact set in memory
and verifies that the managed directory is exact while leaving it unchanged:

```sh
openapi-sdkgen generate \
  --input ./openapi.yaml \
  --target typescript \
  --check \
  --output ./src/generated/api
```

The check fails on generated content/path drift, a changed generation
fingerprint, edited or missing owned files, an invalid manifest, or a conflict
with an unmanaged path. Choose either `--check` or `--incremental` for a run.

## Use diagnostics in CI and tools

Human-readable diagnostics are the default. Tooling can request the stable,
versioned JSON report:

```sh
openapi-sdkgen generate \
  --input ./openapi.yaml \
  --target typescript \
  --check \
  --diagnostics-format json 2> diagnostics.json
```

The report contains `schemaVersion`, severity counts, diagnostics, and skipped
pipeline phases. Diagnostic source names are sanitized before rendering; URL credentials, queries,
and fragments are removed from the report.

Exit status reports the result: zero means the requested check succeeded; a
non-zero status reports generation diagnostics, drift, or an operational error.

## Choose the input source

`--input` accepts a local JSON/YAML file, a `file://` URL, an HTTP(S) URL, or
`-` for stdin.

```sh
# Local file
openapi-sdkgen generate --input ./openapi.yaml --target typescript --check

# Development server
openapi-sdkgen generate \
  --input http://localhost:4010/openapi.json \
  --target typescript \
  --check

# Standard input
curl https://api.example.test/openapi.yaml | \
  openapi-sdkgen generate \
    --input - \
    --input-base https://api.example.test/openapi.yaml \
    --target typescript \
    --check
```

`--input-base` supplies the location used to resolve relative references from
stdin. File and URL inputs already have their own base location.

## Read a protected OpenAPI URL

Pass protected input credentials through environment variables. This keeps secret
values out of command-line arguments. `--http-header-env` maps a request header to the name of an environment variable,
and the generator reads its value internally:

```sh
export OPENAPI_TOKEN='Bearer example-token'

openapi-sdkgen generate \
  --input https://api.internal.example/openapi.yaml \
  --http-header-env Authorization=OPENAPI_TOKEN \
  --target typescript \
  --check
```

`Authorization=OPENAPI_TOKEN` means “read the `OPENAPI_TOKEN` environment
variable.” `$OPENAPI_TOKEN` and `${OPENAPI_TOKEN}` are shell-expansion forms;
this option expects the environment-variable name itself.

For a one-command environment assignment:

```sh
OPENAPI_TOKEN='Bearer example-token' \
  openapi-sdkgen generate \
    --input https://api.internal.example/openapi.yaml \
    --http-header-env Authorization=OPENAPI_TOKEN \
    --target typescript \
    --check
```

The environment variable contains the complete header value, including `Bearer`
when the scheme requires it. Header mappings may be repeated. The CLI rejects
transport-controlled headers such as `Host`, `Cookie`, and `Proxy-Authorization`.

For mTLS or a private CA, use `--tls-client-cert`, `--tls-client-key`, and
`--tls-ca-file`. Provide the client certificate and key together. Mapped headers, client certificates, and private CA settings are scoped to the root
OpenAPI origin. Only same-origin requests receive them.

## Use remote `$ref` values reproducibly

Root OpenAPI URL loading and cross-origin `$ref` fetching use separate controls.
Cross-origin remote references require an explicit allowlist.

On the first run, allow each exact HTTPS origin and update the integrity lock:

```sh
openapi-sdkgen generate \
  --input ./openapi.yaml \
  --target typescript \
  --allow-remote-ref https://schemas.example.test \
  --update-ref-lock \
  --output ./src/generated/api
```

For a local root file, the default lock path is
`<input>.openapi-sdkgen.lock`. Later runs omit `--update-ref-lock` and verify
remote content against the lock before generation continues.

`--offline` resolves references only from the locked local cache and performs no
network fetches. Use `--ref-lock <path>` when you need an explicit lock
location, including URL/stdin workflows that cannot derive one from a local
input filename.

Authentication configured for the root OpenAPI URL is scoped to that origin.

## Generate inbound Webhook and Callback code

The base TypeScript target generates an outbound client. For Webhooks or Callbacks
that your application receives, add the optional server artifact set:

```sh
openapi-sdkgen generate \
  --input ./openapi.yaml \
  --target typescript \
  --with server \
  --output ./src/generated/api
```

`--with server` adds Fetch-native handler/router entry points. Your application
connects them to its HTTP listener, framework, routes, and deployment environment. See [Receive Webhooks and Callbacks](./server.md).

For documents limited to outbound operations, use the base client artifact set.

## Required custom JSON Schema vocabularies

A document that declares a required custom JSON Schema vocabulary needs a trusted
local schema extension. OpenAPI `x-*` fields configure SDK convenience features.

See [Custom JSON Schema vocabularies](./schema-vocabularies.md) for the manifest,
SHA-256, JSON-RPC lowering, integrity-lock workflow, and security boundary.

For a compact lookup of every flag, see the [CLI reference](../reference/cli.md).
