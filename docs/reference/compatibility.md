# Compatibility results

These results show SDK generation and TypeScript typechecking for each OpenAPI
document.

Results are extracted from GitHub Actions. The tables link to the measured runs;
download the [run provenance](/compatibility-results/provenance.json) alongside the results.

## SDK generation results

An **API client SDK** sends requests to an API. An **SDK with Webhooks and
Callbacks** also includes code for receiving inbound requests.

**18 of 20 succeeded** means SDK generation and typechecking succeeded for
18 of the 20 documents. **Generated API calls** counts the callable operations in
successfully generated SDKs. Documents with Webhooks or Callbacks include
the `--with server` results.
**SDK generation time** measures reading the document and writing the SDK files.
The summary shows the sum of the individual document times.

<CompatibilityResults />

For documents with Webhooks or Callbacks, use `--with server` to generate the
receiving code alongside the client. This code parses and validates incoming
requests before passing typed values to your handlers.
See [Receive Webhooks and Callbacks](../guide/server.md).

## Generate only the APIs you need {#graph-selection}

Choose which APIs to generate to include their calls and required dependencies
in the SDK. As an example, we selected nine user, group, and drive APIs from
Microsoft Graph beta. The table shows the selected SDK's generation results
from a recorded Actions run.

<GraphSelection />

Use [API selection](../guide/selective-client.md#generation) to generate the
subset your application needs. The document results below show full API generation.

## Documents covered

### Major API providers

Public API documents from GitHub, Stripe, Cloudflare, GitLab, DigitalOcean,
and Twilio cover large APIs and multi-file schemas. The first
document table shows each provider's generation results and timing.
All six documents passed SDK generation and typechecking. Microsoft Graph beta
is measured separately for generation time and size in [API selection](#graph-selection).

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
Select a document name to open the OpenAPI source (JSON or YAML) used for its results.
A document containing only Webhooks has zero API calls and generated Webhook
handlers. You can also download the original report and input manifest.

<CompatibilityResults evidence />
