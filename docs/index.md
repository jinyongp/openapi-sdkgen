---
layout: home

hero:
  name: openapi-sdkgen
  text: Generate TypeScript SDK source from OpenAPI
  tagline: Keep the API contract in OpenAPI, generate application-owned client source with its runtime, and verify it in CI.
  actions:
    - theme: brand
      text: Create your first SDK
      link: /guide/getting-started
    - theme: alt
      text: Generate and verify
      link: /guide/generate

features:
  - icon: 🧩
    title: Application-owned source
    details: Generate TypeScript into your project and compile it with the toolchain you already use.
  - icon: ✓
    title: Contract validation
    details: Validate request inputs and decoded responses, and use --check to verify generation while keeping output unchanged.
  - icon: ⚡
    title: Typed call surfaces
    details: Call Todo-style resources, exact HTTP routes, or operationId APIs from the same generated contract.
  - icon: ↗
    title: Optional inbound contracts
    details: Generate Fetch-native Webhook and Callback handlers for inbound requests described by your OpenAPI document.
---

## From OpenAPI to a Todo call

Generate the SDK into your application source:

```sh
openapi-sdkgen generate \
  --input ./openapi.yaml \
  --target typescript \
  --output ./src/generated/api
```

Then import it like ordinary TypeScript:

```ts
import { createClient } from "./generated/api";

const api = createClient({
  baseURL: "https://api.example.test/v1",
});

const todo = await api.todos.create({
  body: { title: "Write documentation" },
});
```

The generated directory contains the client, types, and source runtime needed by
your application.

Start with [Create your first SDK](./guide/getting-started.md) for a complete
minimal Todo contract. If generation is already part of your project, see
[Generate and verify an SDK](./guide/generate.md) for incremental updates,
`--check`, authenticated inputs, and remote references.
