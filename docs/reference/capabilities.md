# OpenAPI support

openapi-sdkgen reads OpenAPI 3.0.x, 3.1.x, and 3.2.x documents and interprets
features according to the version declared by the document. Generation is fail-closed: when the selected TypeScript target cannot represent a
used feature safely, the command reports the OpenAPI location and stops.

This page summarizes the main public capability groups. For generation
workflows and flags, use the [CLI reference](./cli.md).

## Supported OpenAPI versions

| OpenAPI | SDK generation | Sequential media |
| --- | --- | --- |
| 3.0.x | Supported | Known sequential content types can use an ordinary `schema` as a complete buffered value. |
| 3.1.x | Supported | Same complete-value support as 3.0.x, with the 3.1 JSON Schema model. |
| 3.2.x | Supported | Adds Media Type Object `itemSchema`, `prefixEncoding`, and `itemEncoding`; these enable typed incremental streams and positional/streaming multipart. |

The generator therefore supports all three OpenAPI 3.x version lines.
OpenAPI 3.2 `itemSchema` adds typed incremental streaming; 3.0 and 3.1 can
still use built-in sequential framing for complete `schema` values. See
[Streaming API](./streaming.md#openapi-version-support) for the exact streaming
contract.

The 3.2-only fields come from the
[OpenAPI Media Type Object](https://spec.openapis.org/oas/v3.2.0.html#media-type-object).

## Client requests and responses

The TypeScript target generates types and executable client behavior for:

- paths, HTTP methods, path/query/header/cookie parameters, and request bodies;
- JSON, text, binary, form, multipart, and supported streaming media;
- status-specific responses, response headers, and raw response access;
- request and decoded-response validation from the applicable OpenAPI/JSON
  Schema contract.

Operations can be called through generated resource methods, exact
`"METHOD /path"` routes, or `operationId` values. See
[Use the generated client](../guide/client.md).

## Servers and security

OpenAPI Server Objects are available to generated operations, including
operation/path/root precedence and server variables. A caller-supplied
[`baseURL`](./client-api.md#clientoptions) overrides the selected OpenAPI server.

The generated client supports OpenAPI API keys, HTTP Basic/Bearer, OAuth2,
OpenID Connect, and mutual TLS security schemes. When several effective Security
Requirement Objects are alternatives, generated request options expose the
allowed [`securityRequirement`](./client-api.md#security-requirements) values as a TypeScript union.

Credential acquisition remains application-owned. See
[Authentication, transport, and streams](../guide/transport.md).

## Links, pagination, and streams

OpenAPI Link Objects become typed follow-up call helpers under
[`$links`](./client-api.md#links). Declaring `x-pagination` adds pagination
helpers; see [OpenAPI x-* extensions](./extensions.md#x-pagination).

Sequential media use the generated operation-centric stream surface. See
[Streaming API](./streaming.md) for `.stream()`, request sources, built-in
protocols, adapters, frame limits, and lifecycle behavior.

## Webhooks and Callbacks

Webhook and Callback Objects describe inbound requests. The base target contains
outbound client artifacts; [`--with server`](./cli.md#typescript-server-add-on)
adds the inbound contracts. The add-on generates Fetch-native handler/router
APIs while the application remains responsible for its HTTP listener, framework
integration, public routes, and authentication policy.

See [Generated server API](./server-api.md) for lookup details and
[Receive Webhooks and Callbacks](../guide/server.md) for the guided workflow.

## JSON Schema vocabularies

Standard JSON Schema vocabulary used by supported OpenAPI versions is handled by
the generator. A required unknown custom vocabulary needs additional schema semantics. Register
a trusted compile-time schema extension for that case. The extension lowers
the custom vocabulary to standard JSON Schema during generation; the generated
runtime uses the lowered schema semantics.

See [Custom JSON Schema vocabularies](../guide/schema-vocabularies.md).

## SDK-specific OpenAPI extensions

Supported `x-*` fields such as [`x-pagination`](./extensions.md#x-pagination),
[`x-envelope`](./extensions.md#x-envelope),
[`x-sdk-visibility`](./extensions.md#x-sdk-visibility),
[`x-sort`](./extensions.md#x-sort), and
[`x-error-category`](./extensions.md#x-error-category) configure generated SDK
conveniences. Custom JSON Schema vocabulary extensions handle schema semantics.

See [OpenAPI x-* extensions](./extensions.md).

## Feature coverage

The project maintains executable feature evidence for supported OpenAPI
versions. The documentation site focuses on how to use supported behavior;
generation diagnostics determine compatibility for a particular document and
installed version.

The source OpenAPI document used for generation is also available through the
generated metadata entry point.
