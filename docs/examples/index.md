# Examples

Try an AI streaming client or receive a Webhook. Use the
[Playground](../playground.md) when you want to experiment with a small OpenAPI
document and inspect generated source immediately.

## Choose an example by task

| What to try | Example and setup |
| --- | --- |
| Generation through a first response | [Getting started](../guide/getting-started.md): Node.js and pnpm; mock responses included |
| File transfers and follow-up calls | [Files, Links, and streams](../guide/files-links-streams.md): feature-specific document and API server |
| Receive AI output incrementally | [AI streaming](./ai-streaming.md): local mock SSE; model and server setup for a real service |
| Receive external requests | [Webhook receiver](./webhook-server.md): local Fetch requests; HTTP host for deployment |

Both examples include a local check with expected output. Follow Getting started
for the shared installation and ESM setup, then adapt the integration to your application.

## AI streaming API

[AI streaming](./ai-streaming.md) generates a client from a streaming contract
and reads SSE events. Try the mock response locally, then connect an AI service.

## Webhook receiver

[Receive an OpenAPI Webhook](./webhook-server.md) shows the opposite direction:
OpenAPI 3.1 describes an inbound Webhook, [`--with server`](../reference/cli.md#with-server) generates its typed
Fetch router, and the host application owns route mounting and authentication.
