# Compatibility results

These results show which OpenAPI documents generated an SDK and passed
TypeScript typechecking.

Last measured: **October 1, 2026** · Generator: **development version after v9.0.0**

## SDK generation results

An **API client SDK** sends requests to an API. An **SDK with Webhooks and
Callbacks** also includes code for receiving inbound requests.

**18 of 20 succeeded** means SDK generation and typechecking succeeded for
18 of the 20 documents. **Generated API calls** counts the callable operations in
successfully generated SDKs, including the `--with server` results for documents
with Webhooks or Callbacks.

<CompatibilityResults />

For documents with Webhooks or Callbacks, use `--with server` to generate the
receiving code alongside the client. This code parses and validates incoming
requests before passing typed values to your handlers.
See [Receive Webhooks and Callbacks](../guide/server.md).

## Documents covered

### Independent holdout

This set contains public OpenAPI 3.0 and 3.1 documents from 20 providers selected
from APIs.guru. API client SDK generation succeeded for 18 documents. Including
server support for Listen Notes' Webhooks and UniCourt's Callbacks brings the
result to all 20 documents.

### Provider-published documents

Zenith Payments published these two OpenAPI 3.2.0 documents. SDK generation
succeeded for both, covering authentication, Schema references, and Links.

### Feature examples

This set contains one real Resend API document and nine authored example
documents covering OpenAPI 3.2, streaming, and reference handling. Resend and the
Webhook example use generation with server support.

For individual feature support, see [OpenAPI support](./capabilities.md).

## Document results

Expand a set to see API call counts and generated Webhook or Callback handlers.
A document containing only Webhooks has zero API calls and generated Webhook
handlers. You can also download the original report and input manifest.

<CompatibilityResults evidence />
