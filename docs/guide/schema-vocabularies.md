# Custom JSON Schema vocabularies

Use [`--schema-extension`](../reference/cli.md#schema-extensions) when an OpenAPI 3.1 or 3.2 schema declares a required
custom JSON Schema vocabulary that openapi-sdkgen needs an external compiler to
interpret.

Schema extensions compile custom JSON Schema vocabularies into ordinary JSON Schema
before SDK generation continues. OpenAPI `x-*` fields configure SDK conveniences
such as pagination or visibility.

## When you need an extension

A schema can declare a vocabulary as required:

```yaml
components:
  schemas:
    TodoTitle:
      $vocabulary:
        https://schemas.example.test/todo-v1: true
      x-todo-title-policy: concise
      type: string
```

Register an extension for every required custom vocabulary. Generation stops when
its semantics are unavailable.

The extension is trusted local code that runs during SDK generation and returns a
replacement JSON Schema fragment. Generated application source contains the lowered
schema semantics.

## Create a manifest

Register one or more vocabulary compilers in a version 1 JSON manifest:

```json
{
  "version": 1,
  "extensions": [
    {
      "vocabularies": ["https://schemas.example.test/todo-v1"],
      "command": "./bin/todo-schema-extension",
      "args": [],
      "sha256": "<sha256-of-the-executable>"
    }
  ]
}
```

`command` may be relative to the manifest and resolves to an executable regular
file. `sha256` matches the executable content. When the executable changes, update
the trusted digest.

The executable speaks the versioned JSON-RPC schema-extension protocol. It
declares the vocabularies it handles and lowers matching schemas to a standard
JSON Schema object or boolean schema for the TypeScript target.

## Record the extension digest

For a local OpenAPI file, the integrity lock defaults to
`<input>.openapi-sdkgen.lock`. The first successful run records the trusted
extension digest:

```sh
openapi-sdkgen generate \
  --input ./openapi.yaml \
  --target typescript \
  --schema-extension ./todo-schema-extension.json \
  --update-ref-lock \
  --output ./src/generated/api
```

Later runs omit [`--update-ref-lock`](../reference/cli.md#remote-ref-options):

```sh
openapi-sdkgen generate \
  --input ./openapi.yaml \
  --target typescript \
  --schema-extension ./todo-schema-extension.json \
  --output ./src/generated/api \
  --incremental
```

If the executable changes, update its manifest digest and run
[`--update-ref-lock`](../reference/cli.md#remote-ref-options) to record the new trusted digest.

When the root OpenAPI document comes from an HTTP(S) URL or stdin, there is no
local input filename from which to derive a lock path. Provide [`--ref-lock`](../reference/cli.md#remote-ref-options):

```sh
openapi-sdkgen generate \
  --input https://api.example.test/openapi.yaml \
  --ref-lock ./openapi-sdkgen.lock \
  --schema-extension ./todo-schema-extension.json \
  --update-ref-lock \
  --target typescript \
  --output ./src/generated/api
```

## Security boundary

Register executables you trust to run on the machine performing generation.
The manifest and integrity lock pin the executable identity. Register trusted code
because the extension runs with the permissions of the generation process.

Generation diagnostics use the supported schema-extension protocol result. Arbitrary
extension process output stays outside application diagnostics.

For ordinary SDK-specific `x-*` fields such as
[`x-pagination`](../reference/extensions.md#x-pagination) and
[`x-sdk-visibility`](../reference/extensions.md#x-sdk-visibility), see
[OpenAPI x-* extensions](../reference/extensions.md).
For CLI flags, see the [CLI reference](../reference/cli.md).
