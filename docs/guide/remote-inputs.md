# Read remote documents and references

For local files, the commands in [Generate and verify](./generate.md) are enough.
Use this guide when reading a URL or stdin, authenticating document downloads, or
loading remote `$ref` values.

Document download credentials are separate from API call credentials in the
generated SDK. See [Authentication and transport](./transport.md) for the latter.
Replace the example URLs below with your actual document and reference addresses.

## Choose the input source

[`--input`](../reference/cli.md#input-source-options) accepts a local JSON/YAML file, a `file://` URL, an HTTP(S) URL, or
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

[`--input-base`](../reference/cli.md#input-source-options) supplies the location used to resolve relative references from
stdin. File and URL inputs already have their own base location.

## Read a protected OpenAPI URL

Pass protected input credentials through environment variables. This keeps secret
values out of command-line arguments. [`--http-header-env`](../reference/cli.md#authenticated-http-s-input) maps a request header to the name of an environment variable,
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
when the scheme requires it. Header mappings may be repeated and require an
`https://` root input; plaintext `http://` inputs are rejected before a request
is opened. The CLI also rejects transport-controlled headers such as `Host`,
`Cookie`, and `Proxy-Authorization`.

For mTLS or a private CA, use [`--tls-client-cert`, `--tls-client-key`, and `--tls-ca-file`](../reference/cli.md#authenticated-http-s-input). Provide the client certificate and key together. Mapped headers, client certificates, and private CA settings are scoped to the root
OpenAPI origin. Only same-origin requests receive them.

## Use remote `$ref` values reproducibly

Root OpenAPI URL loading and cross-origin `$ref` fetching use separate controls.
Cross-origin remote references require an explicit allowlist.

On the first run, use [`--allow-remote-ref`](../reference/cli.md#remote-ref-options) for each exact HTTPS origin and [`--update-ref-lock`](../reference/cli.md#remote-ref-options) to update the integrity lock:

```sh
openapi-sdkgen generate \
  --input ./openapi.yaml \
  --target typescript \
  --allow-remote-ref https://schemas.example.test \
  --update-ref-lock \
  --output ./src/generated/api
```

For a local root file, the default lock path is
`<input>.openapi-sdkgen.lock`. Later runs omit [`--update-ref-lock`](../reference/cli.md#remote-ref-options) and verify
remote content against the lock before generation continues.

[`--offline`](../reference/cli.md#remote-ref-options) resolves references only from the locked local cache and performs no
network fetches. Use [`--ref-lock <path>`](../reference/cli.md#remote-ref-options) when you need an explicit lock
location, including URL/stdin workflows that cannot derive one from a local
input filename.

The root OpenAPI URL is trusted only at its exact original origin (scheme, host,
and port). Redirects must stay on that origin, so use the final canonical root
URL rather than relying on a cross-origin redirect. Authentication configured
for the root URL is scoped to the same boundary. Cross-origin remote references
remain separately authorized through `--allow-remote-ref`.
