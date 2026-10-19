# OpenAPI support

Go generates typed models, outbound clients and optional inbound handlers from OpenAPI 3.0.x,
3.1.x and 3.2.x. Unsupported declarations report their OpenAPI location. Document-wide errors stop
generation; operation/surface diagnostics can omit the affected capability. Use
`--fail-on-resource-omission` when CI must reject resource omissions.

| Area | Go behavior |
| --- | --- |
| Calls | Operations, method/path Routes, resource selectors, typed inputs and outputs |
| Schema | Version-specific schema semantics, presence/null, projections, enums, unions, recursion and constraints |
| Media | JSON, text, binary, XML, forms, multipart and declared incremental media |
| Security | API keys, Basic/Bearer, OAuth2/OIDC, mutual TLS; credential acquisition is host-owned |
| Servers | Operation/path/root precedence, variables and explicit BaseURL override |
| Responses | Status/media variants, declared headers, Raw, typed HTTP errors |
| Links/extensions | Typed Link targets, pagination, sort, visibility, envelope and error category |
| Inbound | `--with server`: typed `net/http` webhooks and callbacks |
| Metadata | `--with metadata`: independent original-document data package |

## Streaming versions {#openapi-version-support}

OpenAPI 3.2 `itemSchema` enables typed incremental items, with `prefixEncoding` and `itemEncoding`
for positional/streaming multipart. In 3.0/3.1, known sequential media can still use ordinary `schema`
as a complete buffered value. Framing, schema validation and HTTP transport remain distinct layers.

## Platform and mapping boundaries

The application supplies Go 1.27+, its own module, transport and listener. Go supports valid HTTP token
methods; browser Fetch restrictions do not apply. There is no separate CONNECT tunnel API.
XML supports documented namespace/attribute/CDATA/wrapped and typed composition mappings; arbitrary
additional-property and positional prefix-item XML mappings retain their mapping limits.
Use JSON or an explicit codec where XML cannot express the schema. A frame bound does not cap total
response size or decoder history. See [streams](./streaming.md) and [compatibility](./compatibility.md).
