---
layout: home

hero:
  name: openapi-sdkgen
  text: Generate SDK source from OpenAPI
  tagline: Read OpenAPI 3.0, 3.1, and 3.2 documents and generate application SDK source for an explicitly selected target. The current release includes the TypeScript target.
  actions:
    - theme: brand
      text: Get started
      link: /guide/getting-started
    - theme: alt
      text: Open Playground
      link: /playground
    - theme: alt
      text: Compatibility results
      link: /reference/compatibility

features:
  - title: OpenAPI 3.0, 3.1, and 3.2
    details: Generate TypeScript SDKs from all three OpenAPI version lines.
  - title: Requests, responses, and streams
    details: Handle JSON, XML, and file transfers alongside SSE and NDJSON streams.
  - title: Type checking and data validation
    details: Type-check your API calls and validate exchanged data against the API document.
  - title: Links, Webhooks, and Callbacks
    details: Generate code for follow-up API calls and for receiving Webhooks and Callbacks.
---

## Current target: TypeScript

The current release includes the TypeScript target. Select it explicitly when
generating an SDK.

```sh
openapi-sdkgen generate \
  --input ./openapi.yaml \
  --target typescript \
  --output ./src/generated/api
```

The TypeScript target currently emits the client, generated types, and source
runtime into the output directory. Import the generated client like ordinary
TypeScript.

```ts
import { createClient } from "./generated/api";

const api = createClient({
  baseURL: "https://api.example.test/v1",
});

const todo = await api.todos.create({
  body: { title: "Write documentation" },
});
```

Start with [Get started](./guide/getting-started.md) for the current TypeScript
workflow. See [Generate and verify](./guide/generate.md) for generation and CI
checks, and [OpenAPI support](./reference/capabilities.md) for the capability
boundary of the selected target.

Use the [Reference](./reference/index.md) and [Examples](./examples/index.md) for
the APIs and integration patterns available in the current release.
