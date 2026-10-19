---
layout: home

hero:
  name: openapi-sdkgen
  text: TypeScript SDKs from OpenAPI
  tagline: Read OpenAPI 3.0, 3.1, and 3.2 documents and generate application SDK source as TypeScript files you own and build with your application.
  actions:
    - theme: brand
      text: Get started
      link: /typescript/guide/getting-started
    - theme: alt
      text: Open Playground
      link: /typescript/playground
    - theme: alt
      text: Compatibility results
      link: /typescript/reference/compatibility

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

## Read the usage flow

[Getting started](./guide/getting-started.md) shows generation and application calls.
[Download a complete example](./examples/index.md) when you want to run the code.

- [Client calls and errors](./guide/client.md)
- [Regenerate after contract changes](./guide/generate.md)
- [Authentication and transport](./guide/transport.md)
- [CLI, types and options](./reference/index.md)

<span id="from-document-to-api-call"></span>
