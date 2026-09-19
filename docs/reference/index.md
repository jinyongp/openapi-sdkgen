# Reference

Use this section when you know the API, option, type, or OpenAPI capability you
need to look up. Task-oriented walkthroughs remain in [Guides](../guide/getting-started.md);
complete multi-codebase integrations live in [Examples](../examples/index.md).

| Need | Reference |
| --- | --- |
| CLI commands, generation modes, input/auth/ref flags | [CLI](./cli.md) |
| `createClient`, client/request options, call surfaces, raw results, errors | [Generated client API](./client-api.md) |
| Webhook/Callback routers, handlers, inbound options | [Generated server API](./server-api.md) |
| `.stream()`, `OperationStream`, request streams, SSE, protocols/adapters/codecs | [Streaming API](./streaming.md) |
| generated request/response/component/helper types | [Generated TypeScript types](./typescript-types.md) |
| OpenAPI 3.0/3.1/3.2 support and version-specific behavior | [OpenAPI support](./capabilities.md) |
| `x-pagination`, `x-envelope`, `x-sort`, visibility, error categories | [OpenAPI x-* extensions](./extensions.md) |

## Public API boundaries

The generated **client API** covers outbound calls made by an SDK consumer.
The generated **server API** covers inbound Webhooks and Callbacks when generation
uses `--with server`. The **Streaming API** is shared by outbound streams and
inbound sequential bodies, so its protocol/adapter contracts have one canonical
reference.

OpenAPI feature support is documented separately from generated TypeScript API
shape. This keeps version questions such as “is this available in OpenAPI 3.1?”
in [OpenAPI support](./capabilities.md), while TypeScript usage remains in the API
references above.
