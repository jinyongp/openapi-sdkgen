# Go examples

| Task | Example | Contract |
| --- | --- | --- |
| First typed HTTP call | [First SDK](../guide/getting-started.md) | [Todo](/examples/golang-todo.json) |
| AI output over SSE | [AI streaming](./ai-streaming.md) | [Generation API](/examples/golang-ai-stream.json) |
| Typed inbound HTTP | [Webhook receiver](./webhook-server.md) | [Todo](/examples/golang-todo.json) |
| Named feature clients | [Named clients](../guide/named-clients.md) | [Orders/catalog](/examples/named-clients.json) |

The [complete Go Todo application](https://github.com/jinyongp/openapi-sdkgen/tree/main/examples/golang-todo-app)
runs raw responses, Links, resources, NDJSON, static selection and a webhook on local servers.
Each example owns cleanup and uses a bounded execution context. Code is identical in English and Korean.
