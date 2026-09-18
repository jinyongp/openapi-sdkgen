# OpenAPI x-* extensions

Standard OpenAPI is enough to generate an SDK. The `x-*` fields on this page
are optional openapi-sdkgen conveniences layered on top of the ordinary OpenAPI
contract.

Required custom JSON Schema vocabularies use the
[Custom JSON Schema vocabularies](../guide/schema-vocabularies.md) workflow.

openapi-sdkgen validates every supported `x-*` declaration before writing code.
Invalid declarations stop generation with a diagnostic.

## Standard OpenAPI behavior

- `api.$routes["METHOD /path"]` identifies APIs by HTTP method and path,
  independently of `operationId`.
- Query, header, cookie, and path parameters keep their OpenAPI names.
- Schema constraints such as `required`, `minimum`, `pattern`, and `enum`
  apply to generated request and response validation.
- Unknown `x-*` fields remain available in metadata and have no effect on SDK
  behavior.

Declare filters as query parameters and `If-Match` or `Idempotency-Key` as header
parameters. The supported `x-*` fields on this page define the available SDK
extension behavior.

## `x-envelope`

Return the `data` property from successful responses.

```yaml
x-envelope: data
```

Every successful JSON response with a body must be an object with a `data`
property. A normal call returns `data`; `.raw()` returns the complete decoded
response.

Omit `x-envelope` when the complete response should be returned.

## `x-pagination`

Generate a `.paginate()` method for an API. The operation remains available as a
normal call when `x-pagination` is absent.

### Default form

The value is `cursor`, `offset`, or `both`.

```yaml
x-pagination: cursor
```

| Mode | Required query parameters |
| --- | --- |
| `cursor` | string `cursor`, positive integer `limit` |
| `offset` | non-negative integer `offset`, positive integer `limit` |
| `both` | `cursor`, `offset`, and `limit` |

Successful JSON responses must use one of these structures:

| Response shape | Items | Pagination data |
| --- | --- | --- |
| Root collection | `/items` | `/pagination/*` |
| Nested collection | `/data/items` | `/data/pagination/*` |
| `data` array | `/data` | `/meta/pagination/*` |

For cursor pagination, `nextCursor` must be a string or `null`. Offset
pagination may use `offset`, `limit`, and `total`, with their valid ranges
declared in the schema.

### Custom parameter names and response paths

Map custom query parameter names and JSON Pointers from the decoded response.

```yaml
x-pagination:
  mode: both
  request:
    cursor: cursorToken
    offset: pageOffset
    limit: pageSize
  response:
    items: /payload/todos
    nextCursor: /payload/page/next
    offset: /payload/page/offset
    limit: /payload/page/limit
    total: /payload/page/total
```

`mode` and `items` are required. Cursor mode also requires request `cursor` and
response `nextCursor`. Offset mode requires request `offset` and `limit`.

Pagination ends when the next cursor is absent or repeated, or when an offset
page is empty or reaches the final item.

## `x-sort`

Declare `x-sort` on the query parameter used for sorting.

```yaml
- name: sort
  in: query
  schema:
    type: array
    items:
      type: string
      enum: [title:asc, title:desc, createdAt:asc, createdAt:desc]
  x-sort:
    format: field-direction
```

The schema must be an array of unique `field:asc` or `field:desc` enum values.
The generated client accepts values such as:

```ts
{ field: "createdAt", direction: "desc" }
```

Use `x-sort` on client operations.

## `x-sdk-visibility`

Control how an API appears in the generated client.

```yaml
x-sdk-visibility: internal
```

- `internal`: expose the API through `$routes` and `$operations`; hide its resource
  method.
- `hidden`: remove the API and related client methods from generated output.

Omitting the extension generates a normal public API.

## `x-error-category`

Add a static error category when the outer `error` object has an exact `code`
and no `category`.

```yaml
x-error-category: validation
```

When the schema already declares a required `category`, that value takes
precedence. Conflicting declarations produce an error.

See [Generate an SDK](../guide/generate.md) for generation errors and CI use.
