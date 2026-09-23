# CLI reference

Use this page to look up command syntax and flag behavior. For a task-oriented
walkthrough, start with [Getting started](../guide/getting-started.md) or
[Generate and verify an SDK](../guide/generate.md).

## Help and version

```sh
openapi-sdkgen --help
openapi-sdkgen generate --help
openapi-sdkgen --version
```

`generate --help` lists the targets, add-ons, and flags supported by the installed
CLI.

## `generate`

```text
openapi-sdkgen generate [options]
```

`--input` and `--target` are required unless they are supplied by `--config`.
Normal generation also requires `--output`; check mode makes `--output` optional.

### Core options

| Option | Meaning |
| --- | --- |
| `--config <path>` | Load repeated generation settings from one explicit TOML file; CLI flags override matching config values |
| `--input <source>` | OpenAPI 3.0.x, 3.1.x, or 3.2.x JSON/YAML source: local path, `file://` URL, HTTP(S) URL, or `-` for stdin |
| `--target typescript` | Generate the TypeScript target |
| `--output <directory>` | Generated directory; with `--check`, verify an existing managed output |
| `--check` | Compile and prepare while leaving generated output unchanged; with `--output`, also verify managed-output drift |
| `--incremental` | Update an existing manifest-owned output directory |
| `--with <addon>` | Add target-specific artifacts; currently `server`; repeatable |
| `--diagnostics-format human|json` | Select human-readable or versioned JSON diagnostics |

Choose either `--check` or `--incremental` for a run. `--output` expects a
directory path; standard output is not a supported generation destination.

## Project configuration

Use `--config <path>` when a project repeatedly uses the same generation
settings:

```toml
source = "./openapi.yaml"
target = "typescript"
output = "./src/generated/api"
addons = ["server"]
incremental = true
diagnostics_format = "human"

[input]
tls_ca_file = "./certs/internal-ca.pem"

[input.headers_from_env]
Authorization = "OPENAPI_TOKEN"

[references]
allow = ["https://schemas.example.com"]
lock = "./openapi.refs.lock"

[schema]
extensions = ["./schema-extensions/example.json"]
```

```sh
openapi-sdkgen generate --config ./openapi-sdkgen.toml
```

Supported config keys are intentionally narrower than the complete CLI surface:

| Config key | CLI equivalent |
| --- | --- |
| `source` | `--input` |
| `target` | `--target` |
| `output` | `--output` |
| `addons` | repeatable `--with` |
| `incremental` | `--incremental` |
| `diagnostics_format` | `--diagnostics-format` |
| `input.base` | `--input-base` |
| `input.headers_from_env` | repeatable `--http-header-env` |
| `input.tls_client_cert` | `--tls-client-cert` |
| `input.tls_client_key` | `--tls-client-key` |
| `input.tls_ca_file` | `--tls-ca-file` |
| `references.allow` | repeatable `--allow-remote-ref` |
| `references.lock` | `--ref-lock` |
| `references.offline` | `--offline` |
| `schema.extensions` | repeatable `--schema-extension` |

Configuration loading is explicit. The CLI does not search the current directory,
parent directories, the home directory, or a global config location. Without
`--config`, existing CLI-only behavior is unchanged.

Relative local paths in the config are resolved from the config file directory,
not from the process working directory. This applies to the local OpenAPI source,
output directory, input base, TLS certificate/key/CA files, reference lock, and
schema-extension manifests. HTTP(S) URLs, `file://` URLs, and stdin `-` keep
their normal source semantics.

CLI flags override config values. For repeatable options, one or more explicit
CLI occurrences replace the complete config list instead of appending to it.
This rule applies to add-ons, remote-reference origins, schema extensions, and
HTTP header environment mappings. Explicit boolean values such as
`--offline=false` also override config booleans.

The config file can map HTTP header names only to **environment-variable names**.
It does not accept a credential value field. Keep the secret in the environment:

```toml
[input.headers_from_env]
Authorization = "OPENAPI_TOKEN"
```

`--check`, `--update-ref-lock`, and `--help` remain CLI-only execution
controls. Unknown TOML keys are rejected so misspelled settings cannot be
silently ignored.

## Fresh, incremental, and check modes

Fresh generation creates a new output directory:

```sh
openapi-sdkgen generate \
  --input ./openapi.yaml \
  --target typescript \
  --output ./src/generated/api
```

Use `--incremental` after the first successful generation to update the same
managed directory. The output manifest controls which files may be replaced or
removed; unmanaged files are preserved.

Omit `--output` for a compiler/target preflight:

```sh
openapi-sdkgen generate \
  --input ./openapi.yaml \
  --target typescript \
  --check
```

Add an existing managed output to verify checked-in generated source while
leaving the directory unchanged:

```sh
openapi-sdkgen generate \
  --input ./openapi.yaml \
  --target typescript \
  --check \
  --output ./src/generated/api
```

The managed check fails for generated content/path drift, a generation
fingerprint change, edited or missing owned files, an invalid manifest, or an
unmanaged path conflict.

See [Generate and verify an SDK](../guide/generate.md) for the intended CI and
regeneration workflows.

## Diagnostics

The default format is `human`. Use JSON when another tool needs structured
output:

```sh
openapi-sdkgen generate \
  --input ./openapi.yaml \
  --target typescript \
  --check \
  --diagnostics-format json 2> diagnostics.json
```

The JSON envelope is versioned and contains counts, diagnostics, and skipped
phases. Diagnostic reports are written to stderr; generated artifacts are written to the
output directory.

## Input source options

### `--input <source>`

Accepted sources:

```sh
# Local file
openapi-sdkgen generate --input ./openapi.yaml --target typescript --check

# file URL
openapi-sdkgen generate --input file:///workspace/openapi.yaml --target typescript --check

# HTTP(S) URL
openapi-sdkgen generate --input https://api.example.test/openapi.yaml --target typescript --check

# stdin
cat ./openapi.yaml | openapi-sdkgen generate --input - --target typescript --check
```

### `--input-base <source>`

Use this when stdin needs a base location for relative references:

```sh
curl https://api.example.test/openapi.yaml | \
  openapi-sdkgen generate \
    --input - \
    --input-base https://api.example.test/openapi.yaml \
    --target typescript \
    --check
```

File and URL inputs already provide their own base.

## Authenticated HTTP(S) input

### `--http-header-env <header=env>`

Maps an HTTP header name to an environment-variable name. The environment
variable's value becomes the complete header value.

```sh
export OPENAPI_TOKEN='Bearer example-token'

openapi-sdkgen generate \
  --input https://api.internal.example/openapi.yaml \
  --http-header-env Authorization=OPENAPI_TOKEN \
  --target typescript \
  --check
```

Pass the environment-variable name, as in `Authorization=OPENAPI_TOKEN`.
`Authorization=$OPENAPI_TOKEN` and `Authorization=${OPENAPI_TOKEN}` invoke shell
expansion, placing the secret value in argv and violating the expected
`Header-Name=ENV_VAR` syntax.

The option is repeatable. Environment variables must exist and contain a
non-empty valid header value. Duplicate header names are rejected. `Host`,
`Cookie`, connection-management headers, proxy authorization, and other
unsafe transport-controlled headers cannot be mapped.

Mapped request headers require an `https://` root input. The CLI rejects
`--http-header-env` before opening a request when the root input uses `http://`.

### `--tls-client-cert <path>` and `--tls-client-key <path>`

Supply a PEM client certificate and private key together for HTTPS input.

### `--tls-ca-file <path>`

Add PEM certificate authorities for the HTTPS input. This extends the certificate trust set while preserving normal TLS verification.

Mapped headers, client certificates, and private CA settings are protected input
credentials scoped to the root OpenAPI origin. Root redirects must remain on
that exact origin (scheme, host, and port), and only same-origin requests receive
the protected transport settings. Cross-origin remote references use the separate
`--allow-remote-ref` policy.

## TypeScript server add-on

### `--with server`

Adds Fetch-native inbound handler/router artifacts for OpenAPI Webhooks and
Callbacks.

```sh
openapi-sdkgen generate \
  --input ./openapi.yaml \
  --target typescript \
  --with server \
  --output ./src/generated/api
```

The add-on generates Fetch-native inbound contracts. Your application supplies the
HTTP listener, framework integration, and public routes.

See [Receive Webhooks and Callbacks](../guide/server.md).

## Remote `$ref` options

Remote references use an explicit allowlist and integrity lock for reproducible,
fail-closed resolution.

| Option | Meaning |
| --- | --- |
| `--allow-remote-ref <origin>` | Allow one exact HTTPS origin for remote `$ref`; repeatable |
| `--ref-lock <path>` | Use an explicit remote-reference/schema-extension integrity lock |
| `--update-ref-lock` | Create or update accepted reference/extension digests after successful compilation |
| `--offline` | Resolve remote references from the locked local cache with no network fetch |

For a local input file, the default lock path is
`<input>.openapi-sdkgen.lock`.

First use of a cross-origin reference:

```sh
openapi-sdkgen generate \
  --input ./openapi.yaml \
  --target typescript \
  --allow-remote-ref https://schemas.example.test \
  --update-ref-lock \
  --output ./src/generated/api
```

Later runs omit `--update-ref-lock` and verify remote content against the lock.

Same-origin relative references from an HTTP(S) root input are allowed as part
of that input. URL/stdin roots require an explicit `--ref-lock` for remote
`$ref` fetching because no local input filename exists for the default lock
path. A different origin additionally needs `--allow-remote-ref`.
Credentials configured for the root origin remain scoped to that origin.

## Schema extensions

### `--schema-extension <manifest>`

Registers a trusted local compiler for a required custom JSON Schema vocabulary. Repeat the option when more than one manifest is needed.

Schema extensions handle required custom JSON Schema vocabularies. SDK-specific
OpenAPI `x-*` fields are documented under
[OpenAPI x-* extensions](./extensions.md).

A schema-extension manifest is versioned, names the vocabulary URI(s), points to
an executable and arguments, and pins the executable by SHA-256. Extension
digests share the reference integrity lock. The first accepted version therefore
requires `--update-ref-lock`.

For local OpenAPI files the lock path can be derived automatically. Schema
extensions with URL or stdin root input require an explicit `--ref-lock`.

The executable runs during generation and lowers the custom vocabulary to ordinary
JSON Schema. Treat it as trusted local code running with the generation process's
permissions; generated source contains the lowered schema semantics.

See [Custom JSON Schema vocabularies](../guide/schema-vocabularies.md) for the
manifest shape, trust model, and first-run/steady-state commands.
