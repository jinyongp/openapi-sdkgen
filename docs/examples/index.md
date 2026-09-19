# Examples

These examples show complete integration boundaries that are larger than one
OpenAPI document or one generated call. Use the
[Playground](../playground.md) when you want to experiment with a small OpenAPI
document and inspect generated source immediately.

## AI streaming API

[AI streaming API with a generated client](./ai-streaming.md) separates the AI
service from its SDK consumer:

- the server application owns the AI SDK, model provider, HTTP endpoint, and
  OpenAPI document;
- openapi-sdkgen generates a client from that published contract;
- the client application owns the generated SDK and any application-specific
  stream adapter.

This structure is useful when the API implementation and SDK consumer live in
separate repositories or deployment units.

## Webhook receiver

[Receive an OpenAPI Webhook](./webhook-server.md) shows the opposite direction:
OpenAPI 3.1 describes an inbound Webhook, [`--with server`](../reference/cli.md#with-server) generates its typed
Fetch router, and the host application owns route mounting and authentication.
