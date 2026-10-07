---
layout: home

hero:
  name: openapi-sdkgen
  text: Generate SDK source from OpenAPI
  tagline: Read OpenAPI 3.0, 3.1, and 3.2 documents and generate application SDK source as TypeScript files you own and build with your application.
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

## From document to API call

Save your document, generate an SDK, and import it into your application. Prepare
the API server separately. [Getting started](./guide/getting-started.md) is a complete
example that verifies a first call with mock responses and needs no server.

| Task | Page |
| --- | --- |
| Generate and run your first SDK | [Getting started](./guide/getting-started.md) |
| Apply document changes and check them in CI | [Generate and verify](./guide/generate.md) |
| Send requests and handle responses and failures | [Client usage](./guide/client.md) |
| Integrate files, streams, or Webhooks | [Examples](./examples/index.md) |
| Look up options and support conditions | [Reference](./reference/index.md) |
