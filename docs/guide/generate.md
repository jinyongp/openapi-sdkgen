# Generate and verify an SDK

Normal generation creates SDK source from an OpenAPI document. Check mode verifies
that SDK generation can succeed and that existing generated files are up to date.
Use it in CI, an editor, or a pre-commit task.

Examples below use [`openapi-sdkgen`](../reference/cli.md) directly. If the CLI is installed as a
project dependency, prefix the command with `pnpm exec`.

If you already completed Getting started, go straight to
[regeneration](#regenerate-an-existing-sdk). Use fresh generation only for a new output directory.

## Create a fresh generated directory

The normal command creates a new managed output directory:

```sh
openapi-sdkgen generate \
  --input ./openapi.yaml \
  --target typescript \
  --output ./src/generated/api
```

The TypeScript target writes the client, generated types, source runtime, and
OpenAPI version information. Fresh generation creates a new output directory.

Generated source belongs to the application repository. Regenerate generator-owned
files through the CLI; managed-output validation detects manual edits. Generation
publishes the output atomically after the full operation succeeds.

## Regenerate an existing SDK

After the first successful generation, use [`--incremental`](../reference/cli.md#fresh-incremental-and-check-modes) with the same
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

## Check generation

Use [`--check`](../reference/cli.md#fresh-incremental-and-check-modes) when CI, an editor, or a pre-commit task needs to validate whether
the OpenAPI document can be generated.

Omit [`--output`](../reference/cli.md#core-options) to check whether the document can generate an SDK:

```sh
openapi-sdkgen generate \
  --input ./openapi.yaml \
  --target typescript \
  --check
```

If generated source is checked into the repository, add the existing managed
output directory. The command compares the expected SDK with the existing files
and leaves the directory unchanged:

```sh
openapi-sdkgen generate \
  --input ./openapi.yaml \
  --target typescript \
  --check \
  --output ./src/generated/api
```

The check fails on generated content/path drift, a changed generation
fingerprint, edited or missing owned files, an invalid manifest, or a conflict
with an unmanaged path. Choose either [`--check`](../reference/cli.md#fresh-incremental-and-check-modes) or [`--incremental`](../reference/cli.md#fresh-incremental-and-check-modes) for a run.

## Choose where generated source lives

You can commit the managed SDK directory alongside the OpenAPI document. Pin the
generator version, regenerate locally with `--incremental`, and run the
output-aware `--check` command above in CI before compiling the application.
Keep the generated manifest with its files.

Alternatively, generate the SDK before the application build and leave that
output directory out of version control. Commit the OpenAPI input and generator
configuration, pin the generator version, and run generation before TypeScript
compilation in a clean checkout. `--incremental` works for both the first build
and later builds. A check with no output directory verifies that the input can
generate; it does not compare files that have not been generated yet.

## Check the CI result

A successful check exits with `0`; generation failures or file drift exit non-zero.
`--check` does not compile TypeScript or call your API. Run the application's type
checks and tests after generation validation.

Use `--diagnostic-mode collect` to see several issues in one run. Add
`--diagnostics-format json` only when a CI tool needs to parse diagnostics.
See the [CLI diagnostics reference](../reference/cli.md#diagnostics) for the format
and failure-reporting contracts.

## Other inputs and optional features

| Task | Next page |
| --- | --- |
| URLs, stdin, document credentials, remote reference locks | [Remote documents and references](./remote-inputs.md) |
| Add Webhook or Callback receivers | [Receive Webhooks and Callbacks](./server.md) |
| Export the original OpenAPI document | [Metadata option](../reference/cli.md#metadata-addon) |
| Handle custom JSON Schema vocabularies | [Schema vocabularies](./schema-vocabularies.md) |

Once the SDK is ready, continue with [client usage](./client.md) for calls and error handling.
