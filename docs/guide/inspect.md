# Find APIs before generating an SDK

Use `inspect` to browse an OpenAPI document, find operation IDs and routes, and
copy the APIs you need into your generation configuration. This command is
available in the next release.

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
target = "typescript"
output = "./src/generated/api"

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
`generate`. Enable TypeScript analysis explicitly with `--target typescript`.

## Find generated client calls {#typescript}

To see how APIs are exposed by the TypeScript client, add a target:

```sh
openapi-sdkgen inspect --input ./openapi.yaml --target typescript --search tasks
```

The additional columns show the resource call expression, such as
`api.users.list()`, and the available `resource`, `routes`, and `operations`
surfaces. `resource` is the nested client API; `routes` identifies a call through
`api.$routes["GET /users"]`; `operations` uses its original ID through
`api.$operations.listUsers`.

Call expressions describe a client containing the document's full API set.
Filters narrow the displayed rows after that analysis. Generating a subset can
change names when APIs compete for the same resource member. Hidden APIs and
omitted APIs are labeled; a resource omission includes its reason, while
available exact routes and operation IDs remain visible.

For large documents, use the default command to find APIs quickly. Target
analysis examines the full client, so its time and memory cost is higher even
when a filter matches only a few rows.

## Use JSON in scripts {#json}

```sh
openapi-sdkgen inspect --input ./openapi.yaml --method GET --format json
```

The result contains `schemaVersion`, document information, `total`, `matched`,
and an `operations` array sorted by path and method. Each operation includes
its exact route, ID, tags, summary, deprecated status, and source location.
An undeclared operation ID is `null`. With TypeScript analysis, each row also
has a `typescript` object describing its call surfaces, and the result records
`analysisScope: "full-document-client"`.

Standard output contains the chosen result format; diagnostics go to standard
error. Use `--diagnostics-format json` when a script also needs structured
diagnostics. All flags and JSON fields are covered in the
[CLI reference](../reference/cli.md#inspect).

## Measured lookup times {#measurements}

The table compares API lookup with full TypeScript call analysis using locally
stored documents. Times are medians of three runs per provider and five runs
for the small example. Memory shows the highest process RSS across those runs.

<InspectMeasurements />
