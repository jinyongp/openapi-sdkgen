# OpenAPI support

openapi-sdkgen reads OpenAPI 3.0.x, 3.1.x, and 3.2.x documents and interprets
features according to the version declared by the document. Generation is fail-closed: when the selected TypeScript target cannot represent a
used feature safely, the command reports the OpenAPI location and stops.

This page summarizes the main public capability groups. For generation
workflows and flags, use the [CLI reference](./cli.md).

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
operation/path/root precedence and server variables. A caller-supplied `baseURL` overrides the selected OpenAPI server.

The generated client supports OpenAPI API keys, HTTP Basic/Bearer, OAuth2,
OpenID Connect, and mutual TLS security schemes. When several effective Security
Requirement Objects are alternatives, generated request options expose the
allowed `securityRequirement` values as a TypeScript union.

Credential acquisition remains application-owned. See
[Authentication, transport, and streams](../guide/transport.md).

## Links, pagination, and streams

OpenAPI Link Objects become typed follow-up call helpers under `$links`.
Supported streaming responses are exposed as `AsyncIterable` values under
`$streams`.

Declaring `x-pagination` adds pagination helpers. Standard OpenAPI operations
remain available as normal calls. See [OpenAPI x-* extensions](./extensions.md).

## Webhooks and Callbacks

Webhook and Callback Objects describe inbound requests. The base target contains
outbound client artifacts; `--with server` adds the inbound contracts when the
application receives them. The
add-on generates Fetch-native handler/router APIs while the application remains
responsible for its HTTP listener, framework integration, public routes, and
authentication policy.

See [Receive Webhooks and Callbacks](../guide/server.md).

## JSON Schema vocabularies

Standard JSON Schema vocabulary used by supported OpenAPI versions is handled by
the generator. A required unknown custom vocabulary needs additional schema semantics. Register
a trusted compile-time schema extension for that case. The extension lowers
the custom vocabulary to standard JSON Schema during generation; the generated
runtime uses the lowered schema semantics.

See [Custom JSON Schema vocabularies](../guide/schema-vocabularies.md).

## SDK-specific OpenAPI extensions

Supported `x-*` fields such as `x-pagination`, `x-envelope`,
`x-sdk-visibility`, `x-sort`, and `x-error-category` configure generated SDK
conveniences. Custom JSON Schema vocabulary extensions handle schema semantics.

See [OpenAPI x-* extensions](./extensions.md).

## Feature coverage

The project maintains executable feature evidence for supported OpenAPI
versions. The documentation site focuses on how to use supported behavior;
generation diagnostics determine compatibility for a particular document and
installed version.

The source OpenAPI document used for generation is also available through the
generated metadata entry point.
