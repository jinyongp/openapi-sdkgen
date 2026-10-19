# Examples

Try an AI streaming client or receive a Webhook. Use the
[Playground](/typescript/playground) when you want to experiment with a small OpenAPI
document and inspect generated source immediately.

## Choose an example by task

| What to try | Example and setup |
| --- | --- |
| Generation through a first response | [Getting started](/typescript/guide/getting-started): Node.js and pnpm; mock responses included |
| File transfers and follow-up calls | [Files, Links, and streams](/typescript/guide/files-links-streams): feature-specific document and API server |
| Receive AI output incrementally | [AI streaming](/typescript/examples/ai-streaming): local mock SSE; model and server setup for a real service |
| Receive external requests | [Webhook receiver](/typescript/examples/webhook-server): local Fetch requests; HTTP host for deployment |

The AI streaming and webhook examples include a local check with expected output. Follow Getting started
for the shared installation and ESM setup, then adapt the integration to your application.

## AI streaming API

[AI streaming](/typescript/examples/ai-streaming) generates a client from a streaming contract
and reads SSE events. Try the mock response locally, then connect an AI service.

## Webhook receiver

[Receive an OpenAPI Webhook](/typescript/examples/webhook-server) shows the opposite direction:
OpenAPI 3.1 describes an inbound Webhook, [`--with server`](/typescript/reference/cli#with-server) generates its typed
Fetch router, and the host application owns route mounting and authentication.
