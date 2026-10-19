# Find APIs before generating an SDK

Use `inspect` to browse an OpenAPI document, find operation IDs and routes, and
copy the APIs you need into your generation configuration.

## Browse and filter {#browse}

Start with a local JSON or YAML document:

```sh
openapi-sdkgen inspect --input ./openapi.yaml
```

The table shows each API's HTTP method, path, operation ID, tags, deprecated
status, and summary. The count below it shows matching APIs out of the document's
total. APIs without an operation ID appear as `—`; their routes still identify
them, such as `GET /tasks/{task-id}`.

Search by path, operation ID, or summary, then narrow the results by method:

```sh
openapi-sdkgen inspect --input ./openapi.yaml --search tasks --method GET
```

Search ignores case. Repeated values within a filter match any value; different
filters must all match. For example, `--tag Tasks --tag Users --method GET`
finds GET APIs tagged `Tasks` or `Users`. Tags and operation IDs match the
document's spelling exactly. Use `--deprecated false` to keep APIs whose
deprecated status is false or undeclared.

For a known API, use `--operation getTask` or
`--route 'GET /tasks/{task-id}'`. IDs and routes form one combined selection:
an API matching either is included. Path parameters keep the document's
`{task-id}` spelling; supply their values when calling the generated client.
A misspelled ID or route produces an error. A search with zero matches returns
an empty table or JSON result successfully.

The list covers the input document's `paths`, including operations mounted
through Path Item references. JSON/YAML files, URLs, stdin, and protected
references use the same [input options](../reference/cli.md#input-source-options) as
generation.

## Copy the results into your configuration {#selection}

Once the filters identify the APIs you want, request a TOML selection:

```sh
openapi-sdkgen inspect --input ./openapi.yaml --search tasks --method GET --format selection
```

For a document containing `GET /tasks/{task-id}`, the output is:

```toml
[selection]
routes = ['GET /tasks/{task-id}']
```

Copy this block into your existing configuration, replacing its current
`[selection]` block. Keep the document, target, and output settings alongside it:

```toml
source = "./openapi.yaml"
target = "golang"
output = "./internal/api"

[golang]
import_path = "example.com/consumer/internal/api"

[selection]
routes = ["GET /tasks/{task-id}"]
```

Then verify the selection and generate the SDK:

```sh
openapi-sdkgen generate --config ./openapi-sdkgen.toml --check
openapi-sdkgen generate --config ./openapi-sdkgen.toml
```

Selection output requires at least one matching API. Routes also work for APIs
without operation IDs. See [Generate only the APIs you need](./selective-client.md#generation)
for linked APIs, callbacks, and updating an existing SDK.

You can browse using the same configuration:

```sh
openapi-sdkgen inspect --config ./openapi-sdkgen.toml --search tasks
```

`inspect` reuses its input and reference settings and searches the whole
document. The configuration's generation selection applies when you run
`generate`. Enable Go analysis explicitly with `--target golang`.



## Find generated calls

```sh
openapi-sdkgen inspect --input ./openapi.json --target golang --search todos
```

Go analysis shows generated resource, route and operation names. Inspect analyzes the full-document client before filtering; it does not generate source.
